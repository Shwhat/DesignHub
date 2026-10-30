package app

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Notification events.
var notifyEvents = []string{"Assigned", "Mentioned", "Status changed", "Approval requested", "Approval decided", "Due soon", "Overdue", "Watched record changed"}

// wantsEvent reports whether a user receives an event (all on by default).
func (a *App) wantsEvent(ctx context.Context, q querier, userID, event string) bool {
	var b []byte
	if err := q.QueryRow(ctx, `SELECT notify_events FROM users WHERE id = $1 AND active`, userID).Scan(&b); err != nil {
		return false
	}
	m := map[string]bool{}
	json.Unmarshal(b, &m)
	if v, ok := m[event]; ok {
		return v
	}
	return true
}

// recordVisibleTo checks access for a user who is not the current request
// user. Notifications are never sent about records the recipient cannot see.
func (a *App) recordVisibleTo(ctx context.Context, userID, recordID string) bool {
	u, err := a.loadUser(ctx, userID)
	if err != nil {
		return false
	}
	return a.canSee(ctx, u, recordID)
}

// notifyQ creates one notification inside a transaction or on the pool.
func (a *App) notifyQ(ctx context.Context, q querier, by *User, recipient, event, recordID, summary string) {
	if by != nil && recipient == by.ID {
		return
	}
	if !a.wantsEvent(ctx, q, recipient, event) || !a.recordVisibleTo(ctx, recipient, recordID) {
		return
	}
	var trig any
	if by != nil {
		trig = by.ID
	}
	if _, err := q.Exec(ctx, `INSERT INTO notifications (recipient_id, event, record_id, triggered_by, summary) VALUES ($1, $2, $3, $4, $5)`,
		recipient, event, recordID, trig, truncate(summary, 300)); err != nil {
		log.Printf("notify: %v", err)
		return
	}
	a.Events.Publish(Event{Type: "notification", Users: []string{recipient}})
}

func (a *App) notify(ctx context.Context, by *User, recipient, event, recordID, summary string) {
	a.notifyQ(ctx, a.DB, by, recipient, event, recordID, summary)
}

func (a *App) watchersOf(ctx context.Context, q querier, recordID string) []string {
	rows, err := q.Query(ctx, `SELECT user_id::text FROM watchers WHERE record_id = $1`, recordID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if rows.Scan(&s) == nil {
			out = append(out, s)
		}
	}
	return out
}

func (a *App) notifyWatchersQ(ctx context.Context, q querier, by *User, recordID, event, summary string) {
	for _, w := range a.watchersOf(ctx, q, recordID) {
		a.notifyQ(ctx, q, by, w, event, recordID, summary)
	}
}

func (a *App) notifyWatchers(ctx context.Context, by *User, recordID, event, summary string) {
	a.notifyWatchersQ(ctx, a.DB, by, recordID, event, summary)
}

var mentionRe = regexp.MustCompile(`(?:^|[^\w@])@([a-zA-Z0-9][a-zA-Z0-9._-]{1,39})`)

// notifyMentions notifies users mentioned as @username in text.
func (a *App) notifyMentions(ctx context.Context, by *User, recordID, title, text string) {
	seen := map[string]bool{}
	for _, m := range mentionRe.FindAllStringSubmatch(text, -1) {
		name := strings.ToLower(strings.TrimRight(m[1], "."))
		if seen[name] {
			continue
		}
		seen[name] = true
		var id string
		if err := a.DB.QueryRow(ctx, `SELECT id::text FROM users WHERE lower(username) = $1 AND active`, name).Scan(&id); err != nil {
			continue
		}
		a.notify(ctx, by, id, "Mentioned", recordID, by.DisplayName+" mentioned you on "+recordID+" "+title)
	}
}

// ---------------------------------------------------------------------------
// Inbox

type notificationRow struct {
	ID          int64     `json:"id"`
	Event       string    `json:"event"`
	RecordID    string    `json:"recordId"`
	RecordTitle string    `json:"recordTitle"`
	RecordKind  string    `json:"recordKind"`
	By          string    `json:"by"`
	Summary     string    `json:"summary"`
	CreatedAt   time.Time `json:"createdAt"`
	Read        bool      `json:"read"`
}

func (a *App) apiNotifications(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	q := r.URL.Query()
	filter := q.Get("filter")
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	cond := ""
	switch filter {
	case "unread":
		cond = " AND NOT n.read"
	case "mentions":
		cond = " AND n.event = 'Mentioned'"
	case "approvals":
		cond = " AND n.event IN ('Approval requested', 'Approval decided')"
	}
	rows, err := a.DB.Query(ctx, `SELECT n.id, n.event, n.record_id, COALESCE(rc.title, ''), COALESCE(rc.kind, ''), COALESCE(us.display_name, 'DesignHub'),
		n.summary, n.created_at, n.read FROM notifications n LEFT JOIN users us ON us.id = n.triggered_by LEFT JOIN records rc ON rc.id = n.record_id
		WHERE n.recipient_id = $1 AND NOT n.dismissed`+cond+` ORDER BY n.created_at DESC LIMIT $2`, u.ID, limit)
	if err != nil {
		return err
	}
	defer rows.Close()
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	out := []notificationRow{}
	for rows.Next() {
		var n notificationRow
		if err := rows.Scan(&n.ID, &n.Event, &n.RecordID, &n.RecordTitle, &n.RecordKind, &n.By, &n.Summary, &n.CreatedAt, &n.Read); err != nil {
			return err
		}
		if vis(n.RecordID) {
			out = append(out, n)
		}
	}
	var unread int
	a.DB.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE recipient_id = $1 AND NOT read AND NOT dismissed`, u.ID).Scan(&unread)
	return ok(w, map[string]any{"items": out, "unread": unread})
}

func (a *App) apiNotificationCount(w http.ResponseWriter, r *http.Request) error {
	var n int
	a.DB.QueryRow(r.Context(), `SELECT count(*) FROM notifications WHERE recipient_id = $1 AND NOT read AND NOT dismissed`, userFrom(r).ID).Scan(&n)
	return ok(w, map[string]int{"unread": n})
}

func (a *App) apiMarkNotifications(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	var in struct {
		IDs     []int64 `json:"ids"`
		All     bool    `json:"all"`
		Read    *bool   `json:"read"`
		Dismiss bool    `json:"dismiss"`
		Record  string  `json:"record"` // mark notifications for a record read
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	var err error
	switch {
	case in.Dismiss:
		_, err = a.DB.Exec(ctx, `UPDATE notifications SET dismissed = true, read = true WHERE recipient_id = $1 AND id = ANY($2)`, u.ID, in.IDs)
	case in.All:
		_, err = a.DB.Exec(ctx, `UPDATE notifications SET read = true WHERE recipient_id = $1`, u.ID)
	case in.Record != "":
		_, err = a.DB.Exec(ctx, `UPDATE notifications SET read = true WHERE recipient_id = $1 AND record_id = $2`, u.ID, in.Record)
	case in.Read != nil:
		_, err = a.DB.Exec(ctx, `UPDATE notifications SET read = $3 WHERE recipient_id = $1 AND id = ANY($2)`, u.ID, in.IDs, *in.Read)
	}
	if err != nil {
		return err
	}
	a.Events.Publish(Event{Type: "notification", Users: []string{u.ID}})
	return ok(w, nil)
}

// dueDateNotifications sends Due soon and Overdue notifications once per
// work item and day. It runs from the scheduler.
func (a *App) dueDateNotifications(ctx context.Context) {
	rows, err := a.DB.Query(ctx, `SELECT w.id, w.title, w.assignee_id::text, w.due_date < current_date FROM work_items w
		WHERE w.assignee_id IS NOT NULL AND w.due_date IS NOT NULL AND w.due_date <= current_date + 2
		AND w.status NOT IN ('Done', 'Cancelled', 'Closed')`)
	if err != nil {
		log.Printf("due dates: %v", err)
		return
	}
	type due struct {
		id, title, user string
		overdue         bool
	}
	var list []due
	for rows.Next() {
		var d due
		if rows.Scan(&d.id, &d.title, &d.user, &d.overdue) == nil {
			list = append(list, d)
		}
	}
	rows.Close()
	for _, d := range list {
		ev := "Due soon"
		if d.overdue {
			ev = "Overdue"
		}
		var exists bool
		a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM notifications WHERE recipient_id = $1 AND record_id = $2 AND event = $3 AND created_at > now() - interval '20 hours')`,
			d.user, d.id, ev).Scan(&exists)
		if !exists {
			a.notify(ctx, nil, d.user, ev, d.id, d.id+" "+d.title+" is "+strings.ToLower(ev))
		}
	}
}
