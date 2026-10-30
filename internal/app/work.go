package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"designhub/internal/model"
)

var workPrefix = map[string]string{"task": "TSK", "issue": "ISS", "action": "ACT", "epic": "EPC", "ncr": "NCR"}

// Default workflow for tasks, issues, actions and epics.
var defaultFlow = map[string][]string{
	"Open":        {"In Progress", "Done", "Blocked", "Cancelled"},
	"In Progress": {"Open", "In Review", "Done", "Blocked", "Cancelled"},
	"In Review":   {"In Progress", "Done", "Blocked", "Cancelled"},
	"Blocked":     {"Open", "In Progress", "Cancelled"},
	"Done":        {"Open"},
	"Cancelled":   {"Open"},
}

var defaultStates = []string{"Open", "In Progress", "In Review", "Blocked", "Done", "Cancelled"}

// Nonconformance workflow.
var ncrFlow = map[string][]string{
	"Reported":          {"Dispositioned"},
	"Dispositioned":     {"Corrective Action", "Reported"},
	"Corrective Action": {"Verified", "Dispositioned"},
	"Verified":          {"Closed", "Corrective Action"},
	"Closed":            {},
}

var ncrStates = []string{"Reported", "Dispositioned", "Corrective Action", "Verified", "Closed"}

func flowFor(typ string) map[string][]string {
	if typ == "ncr" {
		return ncrFlow
	}
	return defaultFlow
}

func closedState(s string) bool { return s == "Done" || s == "Cancelled" || s == "Closed" }

// WorkItem is a task, issue, action, epic or nonconformance.
type WorkItem struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Space       string          `json:"space"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	Priority    string          `json:"priority"`
	AssigneeID  *string         `json:"assigneeId"`
	Assignee    string          `json:"assignee"`
	ReporterID  *string         `json:"reporterId"`
	Reporter    string          `json:"reporter"`
	CreatedBy   string          `json:"createdBy"`
	DueDate     *string         `json:"dueDate"`
	Epic        string          `json:"epic"`
	Labels      []string        `json:"labels"`
	Fields      map[string]any  `json:"fields"`
	Attachments []model.File    `json:"attachments"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
	Version     int             `json:"version"`
	Raw         json.RawMessage `json:"-"`
}

const workSelect = `SELECT w.id, w.type, w.space_id, w.title, w.description, w.status, w.priority, w.assignee_id::text, COALESCE(a.display_name, ''),
	w.reporter_id::text, COALESCE(rp.display_name, ''), cb.display_name, to_char(w.due_date, 'YYYY-MM-DD'), w.epic_id, w.labels, w.fields, w.attachments,
	w.created_at, w.updated_at, w.version
	FROM work_items w LEFT JOIN users a ON a.id = w.assignee_id LEFT JOIN users rp ON rp.id = w.reporter_id JOIN users cb ON cb.id = w.created_by`

func scanWork(row pgx.Row) (*WorkItem, error) {
	w := &WorkItem{}
	var fields, att []byte
	err := row.Scan(&w.ID, &w.Type, &w.Space, &w.Title, &w.Description, &w.Status, &w.Priority, &w.AssigneeID, &w.Assignee,
		&w.ReporterID, &w.Reporter, &w.CreatedBy, &w.DueDate, &w.Epic, &w.Labels, &fields, &att, &w.CreatedAt, &w.UpdatedAt, &w.Version)
	if err != nil {
		return nil, err
	}
	w.Fields = map[string]any{}
	json.Unmarshal(fields, &w.Fields)
	json.Unmarshal(att, &w.Attachments)
	if w.Attachments == nil {
		w.Attachments = []model.File{}
	}
	return w, nil
}

func (a *App) loadWork(ctx context.Context, q querier, id string) (*WorkItem, error) {
	return scanWork(q.QueryRow(ctx, workSelect+` WHERE w.id = $1`, id))
}

func (a *App) workForUser(ctx context.Context, u *User, id string) (*WorkItem, error) {
	w, err := a.loadWork(ctx, a.DB, id)
	if err != nil {
		return nil, errNotFound
	}
	lock, _ := a.lockOf(ctx, a.DB, id)
	if !visible(u, w.Type, w.Space, lock) {
		return nil, errNotFound
	}
	return w, nil
}

// indexWork writes the search row and links of a work item.
func (a *App) indexWork(ctx context.Context, q querier, w *WorkItem) error {
	var text []string
	text = append(text, w.Description)
	for _, k := range []string{"nonconformance", "containment", "root_cause", "corrective_action", "verification", "serial", "po", "disposition_notes", "area"} {
		if s, ok := w.Fields[k].(string); ok {
			text = append(text, s)
		}
	}
	_, err := q.Exec(ctx, `INSERT INTO records (id, kind, title, status, space_id, owner_id, labels, body, origin, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'db', now())
		ON CONFLICT (id) DO UPDATE SET kind = $2, title = $3, status = $4, space_id = $5, owner_id = $6, labels = $7, body = $8, updated_at = now()`,
		w.ID, w.Type, w.Title, w.Status, w.Space, w.AssigneeID, nz(w.Labels), strings.Join(text, "\n"))
	if err != nil {
		return err
	}
	// Found-on and epic membership are fields; keep the link graph in step.
	if _, err := q.Exec(ctx, `DELETE FROM links WHERE src = $1 AND origin = 'db' AND type IN ('found on', 'in epic')`, w.ID); err != nil {
		return err
	}
	if f, _ := w.Fields["found_on"].(string); f != "" {
		rev, _ := w.Fields["found_on_rev"].(string)
		if _, err := q.Exec(ctx, `INSERT INTO links (src, type, dst, dst_rev, origin) VALUES ($1, 'found on', $2, $3, 'db') ON CONFLICT DO NOTHING`, w.ID, f, rev); err != nil {
			return err
		}
	}
	if w.Epic != "" {
		if _, err := q.Exec(ctx, `INSERT INTO links (src, type, dst, origin) VALUES ($1, 'in epic', $2, 'db') ON CONFLICT DO NOTHING`, w.ID, w.Epic); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Lists

type workRow struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Space    string   `json:"space"`
	Title    string   `json:"title"`
	Status   string   `json:"status"`
	Priority string   `json:"priority"`
	Assignee string   `json:"assignee"`
	AssigneeID *string `json:"assigneeId"`
	DueDate  *string  `json:"dueDate"`
	Epic     string   `json:"epic"`
	Labels   []string `json:"labels"`
	Overdue  bool     `json:"overdue"`
	Updated  time.Time `json:"updated"`
	FoundOn  string   `json:"foundOn,omitempty"`
	Severity string   `json:"severity,omitempty"`
	Blocked  bool     `json:"blocked,omitempty"`
}

func toRow(w *WorkItem, today string) workRow {
	r := workRow{ID: w.ID, Type: w.Type, Space: w.Space, Title: w.Title, Status: w.Status, Priority: w.Priority, Assignee: w.Assignee,
		AssigneeID: w.AssigneeID, DueDate: w.DueDate, Epic: w.Epic, Labels: nz(w.Labels), Updated: w.UpdatedAt}
	if w.DueDate != nil && *w.DueDate < today && !closedState(w.Status) {
		r.Overdue = true
	}
	r.FoundOn, _ = w.Fields["found_on"].(string)
	r.Severity, _ = w.Fields["severity"].(string)
	return r
}

func (a *App) queryWork(ctx context.Context, u *User, where string, args ...any) ([]workRow, error) {
	rows, err := a.DB.Query(ctx, workSelect+` WHERE `+where+` ORDER BY w.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	locks, err := a.locks(ctx)
	if err != nil {
		return nil, err
	}
	blocked := map[string]bool{}
	br, err := a.DB.Query(ctx, `SELECT l.dst FROM links l JOIN work_items s ON s.id = l.src WHERE l.type = 'blocks' AND s.status NOT IN ('Done', 'Cancelled', 'Closed')`)
	if err == nil {
		for br.Next() {
			var d string
			if br.Scan(&d) == nil {
				blocked[d] = true
			}
		}
		br.Close()
	}
	today := time.Now().Format("2006-01-02")
	out := []workRow{}
	for rows.Next() {
		w, err := scanWork(rows)
		if err != nil {
			return nil, err
		}
		if !visible(u, w.Type, w.Space, locks[w.ID]) {
			continue
		}
		r := toRow(w, today)
		r.Blocked = blocked[w.ID]
		out = append(out, r)
	}
	return out, rows.Err()
}

func (a *App) apiSpaceWork(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	space := r.PathValue("id")
	if !u.InSpace(space) {
		return errNotFound
	}
	q := r.URL.Query()
	where := `w.space_id = $1`
	args := []any{space}
	if t := q.Get("type"); t != "" {
		types := strings.Split(t, ",")
		args = append(args, types)
		where += fmt.Sprintf(` AND w.type = ANY($%d)`, len(args))
	}
	if q.Get("open") == "1" {
		where += ` AND w.status NOT IN ('Done', 'Cancelled', 'Closed')`
	}
	rows, err := a.queryWork(r.Context(), u, where, args...)
	if err != nil {
		return err
	}
	return ok(w, rows)
}

// apiWorkQuery lists work items across spaces for a live block filter.
func (a *App) apiWorkQuery(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	q := r.URL.Query()
	where := `true`
	args := []any{}
	if s := q.Get("space"); s != "" {
		args = append(args, s)
		where += fmt.Sprintf(` AND w.space_id = $%d`, len(args))
	}
	if t := q.Get("type"); t != "" {
		args = append(args, strings.Split(t, ","))
		where += fmt.Sprintf(` AND w.type = ANY($%d)`, len(args))
	}
	if q.Get("open") != "0" {
		where += ` AND w.status NOT IN ('Done', 'Cancelled', 'Closed')`
	}
	if item := q.Get("item"); item != "" {
		args = append(args, item)
		where += fmt.Sprintf(` AND EXISTS (SELECT 1 FROM links l WHERE l.src = w.id AND l.dst = $%d AND l.type IN ('affects', 'found on'))`, len(args))
	}
	if st := q.Get("status"); st != "" {
		args = append(args, strings.Split(st, ","))
		where += fmt.Sprintf(` AND w.status = ANY($%d)`, len(args))
	}
	rows, err := a.queryWork(r.Context(), u, where, args...)
	if err != nil {
		return err
	}
	return ok(w, rows)
}

func (a *App) apiMyWork(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	rows, err := a.queryWork(r.Context(), u, `w.assignee_id = $1 AND w.status NOT IN ('Done', 'Cancelled', 'Closed')`, u.ID)
	if err != nil {
		return err
	}
	sortMyWork(rows)
	return ok(w, rows)
}

// sortMyWork puts overdue items first, then by due date, then priority.
func sortMyWork(rows []workRow) {
	prio := map[string]int{"Urgent": 0, "High": 1, "Normal": 2, "Low": 3}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Overdue != b.Overdue {
			return a.Overdue
		}
		ad, bd := "9999", "9999"
		if a.DueDate != nil {
			ad = *a.DueDate
		}
		if b.DueDate != nil {
			bd = *b.DueDate
		}
		if ad != bd {
			return ad < bd
		}
		if prio[a.Priority] != prio[b.Priority] {
			return prio[a.Priority] < prio[b.Priority]
		}
		return a.ID < b.ID
	})
}

// ---------------------------------------------------------------------------
// Create and update

type workInput struct {
	Type        string          `json:"type"`
	Space       string          `json:"space"`
	Title       *string         `json:"title"`
	Description *string         `json:"description"`
	Priority    *string         `json:"priority"`
	AssigneeID  *string         `json:"assigneeId"`
	ReporterID  *string         `json:"reporterId"`
	DueDate     *string         `json:"dueDate"`
	Epic        *string         `json:"epic"`
	Labels      *[]string       `json:"labels"`
	Fields      map[string]any  `json:"fields"`
	Attachments *[]model.File   `json:"attachments"`
	Links       []model.Link    `json:"links"`
	Version     int             `json:"version"`
}

// Fields allowed per type, with whether each is free text.
var workFields = map[string][]string{
	"issue":  {"area", "severity"},
	"action": {"source"},
	"epic":   {"start_date", "target_date"},
	"ncr": {"found_on", "found_on_rev", "serial", "quantity", "where_found", "manufacturer", "po", "requirement_not_met", "severity",
		"containment", "disposition", "disposition_notes", "root_cause", "corrective_action", "verification", "requires_design_change", "disposition_approvers"},
}

var ncrDispositions = []string{"", "Use as is", "Rework", "Repair", "Scrap", "Return to vendor"}
var whereFound = []string{"Receiving inspection", "In process", "Final test", "Field"}
var severities = []string{"Minor", "Major", "Critical"}

func (a *App) apiCreateWork(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	var in workInput
	if err := decode(r, &in); err != nil {
		return err
	}
	id, err := a.CreateWork(r.Context(), u, clientIP(r), in)
	if err != nil {
		return err
	}
	return ok(w, map[string]string{"id": id})
}

// CreateWork creates a work item.
func (a *App) CreateWork(ctx context.Context, u *User, ip string, in workInput) (string, error) {
	prefix, okT := workPrefix[in.Type]
	if !okT {
		return "", badRequest("Unknown work item type.")
	}
	if !u.CanIn(in.Space, PermWorkEdit) {
		return "", errForbidden
	}
	var archived bool
	if err := a.DB.QueryRow(ctx, `SELECT archived FROM spaces WHERE id = $1`, in.Space).Scan(&archived); err != nil {
		return "", badRequest("Unknown space.")
	}
	if archived {
		return "", badRequest("This space is archived.")
	}
	wi := &WorkItem{Type: in.Type, Space: in.Space, Status: "Open", Priority: "Normal", Fields: map[string]any{}, Labels: []string{}, Attachments: []model.File{}}
	if in.Type == "ncr" {
		wi.Status = "Reported"
	}
	rep := u.ID
	wi.ReporterID = &rep
	if err := a.applyWorkInput(ctx, wi, &in, true); err != nil {
		return "", err
	}
	if err := checkWorkRequired(wi); err != nil {
		return "", err
	}
	fields, _ := json.Marshal(wi.Fields)
	att, _ := json.Marshal(wi.Attachments)
	err := a.tx(ctx, func(tx pgx.Tx) error {
		id, err := a.nextPrefixed(ctx, tx, prefix, in.Type, u.ID)
		if err != nil {
			return err
		}
		wi.ID = id
		var due any
		if wi.DueDate != nil {
			due = *wi.DueDate
		}
		if _, err := tx.Exec(ctx, `INSERT INTO work_items (id, type, space_id, title, description, status, priority, assignee_id, reporter_id, created_by, due_date, epic_id, labels, fields, attachments)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
			id, wi.Type, wi.Space, wi.Title, wi.Description, wi.Status, wi.Priority, wi.AssigneeID, wi.ReporterID, u.ID, due, wi.Epic, nz(wi.Labels), fields, att); err != nil {
			return err
		}
		for _, l := range in.Links {
			if err := a.addWorkLink(ctx, tx, u, id, l); err != nil {
				return err
			}
		}
		if err := a.syncNCRApprovers(ctx, tx, wi); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO watchers (record_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, u.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO work_history (item_id, user_id, changes) VALUES ($1, $2, '{"created": true}')`, id, u.ID); err != nil {
			return err
		}
		if err := a.indexWork(ctx, tx, wi); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, ip, "work.create", id, map[string]any{"title": wi.Title})
		return nil
	})
	if err != nil {
		return "", err
	}
	if wi.AssigneeID != nil && *wi.AssigneeID != u.ID {
		a.notify(ctx, u, *wi.AssigneeID, "Assigned", wi.ID, fmt.Sprintf("%s assigned you %s %s", u.DisplayName, wi.ID, wi.Title))
		a.DB.Exec(ctx, `INSERT INTO watchers (record_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, wi.ID, *wi.AssigneeID)
	}
	a.notifyMentions(ctx, u, wi.ID, wi.Title, wi.Description)
	a.Events.Publish(Event{Type: "board", ID: wi.ID, Space: wi.Space})
	return wi.ID, nil
}

// applyWorkInput copies validated input onto a work item.
func (a *App) applyWorkInput(ctx context.Context, wi *WorkItem, in *workInput, creating bool) error {
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if t == "" || len([]rune(t)) > 150 {
			return badRequest("Title is required (at most 150 characters).")
		}
		wi.Title = t
	}
	if in.Description != nil {
		wi.Description = *in.Description
	}
	if in.Priority != nil {
		if !contains(priorities, *in.Priority) {
			return badRequest("Unknown priority.")
		}
		wi.Priority = *in.Priority
	}
	if in.AssigneeID != nil {
		if *in.AssigneeID == "" {
			wi.AssigneeID = nil
		} else {
			au, err := a.loadUser(ctx, *in.AssigneeID)
			if err != nil || !au.InSpace(wi.Space) {
				return badRequest("The assignee must be a member of the space.")
			}
			id := *in.AssigneeID
			wi.AssigneeID = &id
		}
	}
	if in.ReporterID != nil && *in.ReporterID != "" {
		id := *in.ReporterID
		wi.ReporterID = &id
	}
	if in.DueDate != nil {
		if *in.DueDate == "" {
			wi.DueDate = nil
		} else {
			if _, err := time.Parse("2006-01-02", *in.DueDate); err != nil {
				return badRequest("Due date must be a date.")
			}
			d := *in.DueDate
			wi.DueDate = &d
		}
	}
	if in.Epic != nil {
		if *in.Epic != "" {
			if wi.Type == "epic" {
				return badRequest("An epic cannot belong to an epic.")
			}
			var t, s string
			if err := a.DB.QueryRow(ctx, `SELECT type, space_id FROM work_items WHERE id = $1`, *in.Epic).Scan(&t, &s); err != nil || t != "epic" || s != wi.Space {
				return badRequest("Unknown epic in this space.")
			}
		}
		wi.Epic = *in.Epic
	}
	if in.Labels != nil {
		wi.Labels = cleanLabels(*in.Labels)
	}
	if in.Attachments != nil {
		for _, f := range *in.Attachments {
			var n int
			a.DB.QueryRow(ctx, `SELECT count(*) FROM files WHERE sha256 = $1`, f.SHA256).Scan(&n)
			if n == 0 {
				return badRequest("Attachment %s was not uploaded.", f.Name)
			}
		}
		wi.Attachments = *in.Attachments
	}
	for k, v := range in.Fields {
		if !contains(workFields[wi.Type], k) {
			return badRequest("Unknown field %s.", k)
		}
		wi.Fields[k] = v
	}
	// Field checks.
	str := func(k string) string { s, _ := wi.Fields[k].(string); return s }
	if wi.Type == "issue" && in.Fields != nil {
		if s := str("severity"); s != "" && !contains(severities, s) {
			return badRequest("Unknown severity.")
		}
	}
	if wi.Type == "ncr" {
		if s := str("severity"); s != "" && !contains(severities, s) {
			return badRequest("Unknown severity.")
		}
		if s := str("where_found"); s != "" && !contains(whereFound, s) {
			return badRequest("Unknown place found.")
		}
		if s := str("disposition"); !contains(ncrDispositions, s) {
			return badRequest("Unknown disposition.")
		}
		if f := str("found_on"); f != "" {
			e := cache.get(f)
			if e == nil || e.Item.Type != model.TypePart {
				return badRequest("Found on must be a part or assembly.")
			}
			if str("found_on_rev") == "" || creating {
				if str("found_on_rev") == "" {
					wi.Fields["found_on_rev"] = e.Item.Revision
				}
			}
		}
	}
	return nil
}

func cleanLabels(ls []string) []string {
	out := []string{}
	for _, l := range ls {
		l = strings.TrimSpace(l)
		if l != "" && !contains(out, l) && len(l) <= 40 {
			out = append(out, l)
		}
	}
	return out
}

// checkWorkRequired enforces the required fields for a type and state.
func checkWorkRequired(wi *WorkItem) error {
	str := func(k string) string { s, _ := wi.Fields[k].(string); return strings.TrimSpace(s) }
	if wi.Title == "" {
		return badRequest("Title is required.")
	}
	switch wi.Type {
	case "issue":
		if str("area") == "" || str("severity") == "" {
			return badRequest("Area and severity are required for issues.")
		}
	case "ncr":
		if str("found_on") == "" {
			return badRequest("Found on is required.")
		}
		if q, okq := wi.Fields["quantity"].(float64); !okq || q <= 0 {
			return badRequest("Quantity affected is required.")
		}
		if str("where_found") == "" || str("severity") == "" || strings.TrimSpace(wi.Description) == "" {
			return badRequest("Where found, severity and the nonconformance description are required.")
		}
		if _, okd := wi.Fields["requires_design_change"].(bool); !okd {
			wi.Fields["requires_design_change"] = false
		}
	}
	return nil
}

// ncrStateRequirement checks the fields needed to enter an NCR state.
func (a *App) ncrStateRequirement(ctx context.Context, q querier, wi *WorkItem, state string) error {
	str := func(k string) string { s, _ := wi.Fields[k].(string); return strings.TrimSpace(s) }
	switch state {
	case "Dispositioned":
		if str("disposition") == "" {
			return badRequest("Choose a disposition first.")
		}
		appr, err := a.ncrApprovals(ctx, q, wi.ID)
		if err != nil {
			return err
		}
		if len(appr) == 0 {
			return badRequest("Add disposition approvers first.")
		}
		for _, ap := range appr {
			if ap.Decision != "Approved" {
				return badRequest("Every disposition approver must sign first.")
			}
		}
	case "Verified":
		if str("root_cause") == "" || str("corrective_action") == "" {
			return badRequest("Root cause and corrective action are required.")
		}
	case "Closed":
		if str("verification") == "" {
			return badRequest("Verification of effectiveness is required.")
		}
		if b, _ := wi.Fields["requires_design_change"].(bool); b {
			var n int
			q.QueryRow(ctx, `SELECT count(*) FROM links WHERE src = $1 AND type = 'fixed by'`, wi.ID).Scan(&n)
			if n == 0 {
				return badRequest("This nonconformance requires a design change. Link a change order first.")
			}
		}
	}
	return nil
}

func (a *App) apiUpdateWork(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	wi, err := a.workForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !u.CanIn(wi.Space, PermWorkEdit) {
		return errForbidden
	}
	var in workInput
	if err := decode(r, &in); err != nil {
		return err
	}
	if in.Version != 0 && in.Version != wi.Version {
		return conflict("Someone changed this item after you opened it. Reload to see their changes.", nil)
	}
	before := *wi
	beforeFields := map[string]any{}
	for k, v := range wi.Fields {
		beforeFields[k] = v
	}
	if err := a.applyWorkInput(ctx, wi, &in, false); err != nil {
		return err
	}
	if err := checkWorkRequired(wi); err != nil {
		return err
	}
	// NCR fields of later stages may not be edited once the stage closed.
	if wi.Type == "ncr" && wi.Status == "Closed" {
		return badRequest("Closed nonconformances cannot be changed.")
	}
	changes := map[string]any{}
	cmp := func(name string, a, b any) {
		ab, _ := json.Marshal(a)
		bb, _ := json.Marshal(b)
		if string(ab) != string(bb) {
			changes[name] = map[string]any{"from": a, "to": b}
		}
	}
	cmp("title", before.Title, wi.Title)
	cmp("description", before.Description, wi.Description)
	cmp("priority", before.Priority, wi.Priority)
	cmp("assignee", before.AssigneeID, wi.AssigneeID)
	cmp("dueDate", before.DueDate, wi.DueDate)
	cmp("epic", before.Epic, wi.Epic)
	cmp("labels", before.Labels, wi.Labels)
	for k := range wi.Fields {
		cmp(k, beforeFields[k], wi.Fields[k])
	}
	if len(changes) == 0 {
		return ok(w, map[string]int{"version": wi.Version})
	}
	fields, _ := json.Marshal(wi.Fields)
	att, _ := json.Marshal(wi.Attachments)
	var due any
	if wi.DueDate != nil {
		due = *wi.DueDate
	}
	err = a.tx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE work_items SET title = $2, description = $3, priority = $4, assignee_id = $5, reporter_id = $6, due_date = $7, epic_id = $8,
			labels = $9, fields = $10, attachments = $11, updated_at = now(), version = version + 1 WHERE id = $1 AND version = $12`,
			wi.ID, wi.Title, wi.Description, wi.Priority, wi.AssigneeID, wi.ReporterID, due, wi.Epic, nz(wi.Labels), fields, att, before.Version)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return conflict("Someone changed this item after you opened it. Reload to see their changes.", nil)
		}
		cj, _ := json.Marshal(changes)
		if _, err := tx.Exec(ctx, `INSERT INTO work_history (item_id, user_id, changes) VALUES ($1, $2, $3)`, wi.ID, u.ID, cj); err != nil {
			return err
		}
		if _, ok := changes["disposition"]; ok {
			// A changed disposition needs fresh signatures.
			if _, err := tx.Exec(ctx, `UPDATE ncr_approvals SET decision = '', decided_at = NULL, comment = '' WHERE ncr_id = $1`, wi.ID); err != nil {
				return err
			}
		}
		if err := a.syncNCRApprovers(ctx, tx, wi); err != nil {
			return err
		}
		return a.indexWork(ctx, tx, wi)
	})
	if err != nil {
		return err
	}
	if _, ch := changes["assignee"]; ch && wi.AssigneeID != nil {
		a.notify(ctx, u, *wi.AssigneeID, "Assigned", wi.ID, fmt.Sprintf("%s assigned you %s %s", u.DisplayName, wi.ID, wi.Title))
		a.DB.Exec(ctx, `INSERT INTO watchers (record_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, wi.ID, *wi.AssigneeID)
	}
	if _, ch := changes["description"]; ch {
		a.notifyMentions(ctx, u, wi.ID, wi.Title, wi.Description)
	}
	a.notifyWatchers(ctx, u, wi.ID, "Watched record changed", fmt.Sprintf("%s changed %s %s", u.DisplayName, wi.ID, wi.Title))
	a.Events.Publish(Event{Type: "board", ID: wi.ID, Space: wi.Space})
	return ok(w, map[string]int{"version": wi.Version + 1})
}

// syncNCRApprovers keeps the disposition approver rows in step with the
// disposition_approvers field.
func (a *App) syncNCRApprovers(ctx context.Context, tx pgx.Tx, wi *WorkItem) error {
	if wi.Type != "ncr" {
		return nil
	}
	raw, _ := wi.Fields["disposition_approvers"].([]any)
	want := map[string]bool{}
	for _, x := range raw {
		if s, ok := x.(string); ok && s != "" {
			want[s] = true
		}
	}
	rows, err := tx.Query(ctx, `SELECT user_id::text FROM ncr_approvals WHERE ncr_id = $1`, wi.ID)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for rows.Next() {
		var s string
		if rows.Scan(&s) == nil {
			have[s] = true
		}
	}
	rows.Close()
	for id := range want {
		if !have[id] {
			au, err := a.loadUser(ctx, id)
			if err != nil || !au.Can(PermCOApprove) {
				return badRequest("Disposition approvers must have approval permission.")
			}
			if _, err := tx.Exec(ctx, `INSERT INTO ncr_approvals (ncr_id, user_id) VALUES ($1, $2)`, wi.ID, id); err != nil {
				return err
			}
		}
	}
	for id := range have {
		if !want[id] {
			if _, err := tx.Exec(ctx, `DELETE FROM ncr_approvals WHERE ncr_id = $1 AND user_id = $2`, wi.ID, id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *App) ncrApprovals(ctx context.Context, q querier, id string) ([]approverRow, error) {
	rows, err := q.Query(ctx, `SELECT ap.user_id::text, us.username, us.display_name, ap.decision, ap.decided_at, ap.comment
		FROM ncr_approvals ap JOIN users us ON us.id = ap.user_id WHERE ap.ncr_id = $1 ORDER BY us.display_name`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []approverRow{}
	for rows.Next() {
		var x approverRow
		if err := rows.Scan(&x.UserID, &x.Username, &x.Name, &x.Decision, &x.DecidedAt, &x.Comment); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Status

func (a *App) apiWorkStatus(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	wi, err := a.workForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !u.CanIn(wi.Space, PermWorkEdit) {
		return errForbidden
	}
	var in struct {
		Status string `json:"status"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	if err := a.setWorkStatus(ctx, u, clientIP(r), wi, in.Status); err != nil {
		return err
	}
	return ok(w, nil)
}

func (a *App) setWorkStatus(ctx context.Context, u *User, ip string, wi *WorkItem, status string) error {
	if status == wi.Status {
		return nil
	}
	if !contains(flowFor(wi.Type)[wi.Status], status) {
		return badRequest("%s cannot move from %s to %s.", wi.ID, wi.Status, status)
	}
	err := a.tx(ctx, func(tx pgx.Tx) error {
		if wi.Type == "ncr" {
			if err := a.ncrStateRequirement(ctx, tx, wi, status); err != nil {
				return err
			}
		}
		tag, err := tx.Exec(ctx, `UPDATE work_items SET status = $2, updated_at = now(), version = version + 1 WHERE id = $1 AND status = $3`, wi.ID, status, wi.Status)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return conflict("The status changed after you opened this item.", nil)
		}
		cj, _ := json.Marshal(map[string]any{"status": map[string]string{"from": wi.Status, "to": status}})
		if _, err := tx.Exec(ctx, `INSERT INTO work_history (item_id, user_id, changes) VALUES ($1, $2, $3)`, wi.ID, u.ID, cj); err != nil {
			return err
		}
		from := wi.Status
		wi.Status = status
		if err := a.indexWork(ctx, tx, wi); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, ip, "work.status", wi.ID, map[string]string{"from": from, "to": status})
		a.notifyWatchersQ(ctx, tx, u, wi.ID, "Status changed", fmt.Sprintf("%s %s moved from %s to %s", wi.ID, wi.Title, from, status))
		return nil
	})
	if err != nil {
		return err
	}
	a.Events.Publish(Event{Type: "board", ID: wi.ID, Space: wi.Space, Status: status})
	return nil
}

// apiSignNCR records a disposition approver's electronic signature and moves
// the NCR to Dispositioned when every approver has approved.
func (a *App) apiSignNCR(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	wi, err := a.workForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if wi.Type != "ncr" || wi.Status != "Reported" {
		return badRequest("Disposition can be signed only while the nonconformance is Reported.")
	}
	var in struct {
		Decision string `json:"decision"`
		Password string `json:"password"`
		Comment  string `json:"comment"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	if in.Decision != "Approved" && in.Decision != "Rejected" {
		return badRequest("Decision must be Approved or Rejected.")
	}
	if d, _ := wi.Fields["disposition"].(string); d == "" {
		return badRequest("Choose a disposition first.")
	}
	if !u.Can(PermCOApprove) {
		return errForbidden
	}
	if !a.checkUserPassword(ctx, u.ID, in.Password) {
		a.Audit(ctx, u, clientIP(r), "signature.failed", wi.ID, nil)
		return &Error{Status: http.StatusUnauthorized, Code: "signature", Message: "Password is incorrect. The decision was not recorded."}
	}
	err = a.tx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE ncr_approvals SET decision = $3, decided_at = now(), comment = $4 WHERE ncr_id = $1 AND user_id = $2`, wi.ID, u.ID, in.Decision, strings.TrimSpace(in.Comment))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return badRequest("You are not a disposition approver of %s.", wi.ID)
		}
		a.auditQ(ctx, tx, u, clientIP(r), "ncr.disposition.sign", wi.ID, map[string]any{"decision": in.Decision, "disposition": wi.Fields["disposition"], "signature": "password"})
		return nil
	})
	if err != nil {
		return err
	}
	if in.Decision == "Approved" {
		if a.ncrStateRequirement(ctx, a.DB, wi, "Dispositioned") == nil {
			if err := a.setWorkStatus(ctx, u, clientIP(r), wi, "Dispositioned"); err != nil {
				return err
			}
		}
	}
	a.Events.Publish(Event{Type: "record", ID: wi.ID, Space: wi.Space})
	return ok(w, nil)
}

// ---------------------------------------------------------------------------
// Links

var workLinkTypes = []string{"blocks", "duplicates", "relates to", "affects", "fixed by"}

func (a *App) addWorkLink(ctx context.Context, tx pgx.Tx, u *User, src string, l model.Link) error {
	if !contains(workLinkTypes, l.Type) {
		return badRequest("Unknown link type %q.", l.Type)
	}
	if l.To == src {
		return badRequest("An item cannot link to itself.")
	}
	t := model.TypeOf(l.To)
	switch l.Type {
	case "blocks", "duplicates":
		if _, ok := workPrefix[t]; !ok {
			return badRequest("%s links must point to work items.", l.Type)
		}
	case "affects":
		if cache.get(l.To) == nil {
			return badRequest("Affects must point to an engineering item.")
		}
	case "fixed by":
		if t != "change_order" {
			return badRequest("Fixed by must point to a change order.")
		}
	}
	var exists bool
	tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM records WHERE id = $1)`, l.To).Scan(&exists)
	if !exists {
		return badRequest("%s does not exist.", l.To)
	}
	_, err := tx.Exec(ctx, `INSERT INTO links (src, type, dst, dst_rev, origin) VALUES ($1, $2, $3, $4, 'db') ON CONFLICT DO NOTHING`, src, l.Type, l.To, l.Rev)
	return err
}

func (a *App) apiWorkLinks(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	wi, err := a.workForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !u.CanIn(wi.Space, PermWorkEdit) {
		return errForbidden
	}
	var in struct {
		Add    []model.Link `json:"add"`
		Remove []model.Link `json:"remove"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	err = a.tx(ctx, func(tx pgx.Tx) error {
		for _, l := range in.Remove {
			if _, err := tx.Exec(ctx, `DELETE FROM links WHERE src = $1 AND type = $2 AND dst = $3 AND origin = 'db'`, wi.ID, l.Type, l.To); err != nil {
				return err
			}
		}
		for _, l := range in.Add {
			if !a.canSee(ctx, u, l.To) {
				return badRequest("%s does not exist.", l.To)
			}
			if err := a.addWorkLink(ctx, tx, u, wi.ID, l); err != nil {
				return err
			}
		}
		cj, _ := json.Marshal(map[string]any{"links": map[string]any{"added": in.Add, "removed": in.Remove}})
		_, err := tx.Exec(ctx, `INSERT INTO work_history (item_id, user_id, changes) VALUES ($1, $2, $3)`, wi.ID, u.ID, cj)
		return err
	})
	if err != nil {
		return err
	}
	a.Events.Publish(Event{Type: "record", ID: wi.ID, Space: wi.Space})
	return ok(w, nil)
}

// ---------------------------------------------------------------------------
// View

type workLink struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Kind   string `json:"kind"`
	Rev    string `json:"rev,omitempty"`
	Out    bool   `json:"out"`
}

type workHistoryRow struct {
	At      time.Time       `json:"at"`
	User    string          `json:"user"`
	Changes json.RawMessage `json:"changes"`
}

func (a *App) apiWork(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	wi, err := a.workForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	links := []workLink{}
	rows, err := a.DB.Query(ctx, `SELECT l.type, l.dst, l.dst_rev, COALESCE(r.title, ''), COALESCE(r.status, ''), COALESCE(r.kind, ''), true
		FROM links l LEFT JOIN records r ON r.id = l.dst WHERE l.src = $1 AND l.type <> 'in epic'
		UNION ALL SELECT l.type, l.src, '', COALESCE(r.title, ''), COALESCE(r.status, ''), COALESCE(r.kind, ''), false
		FROM links l LEFT JOIN records r ON r.id = l.src WHERE l.dst = $1 AND l.type <> 'in epic'`, wi.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var l workLink
		if err := rows.Scan(&l.Type, &l.ID, &l.Rev, &l.Title, &l.Status, &l.Kind, &l.Out); err != nil {
			rows.Close()
			return err
		}
		if !vis(l.ID) {
			continue
		}
		if !l.Out {
			if n, ok := reverseNames[l.Type]; ok {
				l.Type = n
			}
		}
		links = append(links, l)
	}
	rows.Close()
	hist := []workHistoryRow{}
	rows, err = a.DB.Query(ctx, `SELECT h.at, COALESCE(us.display_name, ''), h.changes FROM work_history h LEFT JOIN users us ON us.id = h.user_id
		WHERE h.item_id = $1 ORDER BY h.at DESC LIMIT 300`, wi.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var h workHistoryRow
		if err := rows.Scan(&h.At, &h.User, &h.Changes); err != nil {
			rows.Close()
			return err
		}
		hist = append(hist, h)
	}
	rows.Close()
	out := map[string]any{"item": wi, "links": links, "history": hist, "transitions": flowFor(wi.Type)[wi.Status],
		"canEdit": u.CanIn(wi.Space, PermWorkEdit), "states": map[bool][]string{true: ncrStates, false: defaultStates}[wi.Type == "ncr"]}
	if wi.Type == "epic" {
		var total, done int
		cr, err := a.DB.Query(ctx, `SELECT id, status FROM work_items WHERE epic_id = $1`, wi.ID)
		if err == nil {
			children := []map[string]string{}
			for cr.Next() {
				var id, st string
				if cr.Scan(&id, &st) == nil {
					if st == "Cancelled" {
						continue
					}
					total++
					if st == "Done" {
						done++
					}
					if vis(id) {
						children = append(children, map[string]string{"id": id, "status": st})
					}
				}
			}
			cr.Close()
			pct := 0
			if total > 0 {
				pct = done * 100 / total
			}
			out["progress"] = map[string]int{"done": done, "total": total, "percent": pct}
			out["children"] = children
		}
	}
	if wi.Type == "ncr" {
		appr, err := a.ncrApprovals(ctx, a.DB, wi.ID)
		if err != nil {
			return err
		}
		out["approvals"] = appr
		var canSign bool
		for _, ap := range appr {
			if ap.UserID == u.ID && ap.Decision == "" && wi.Status == "Reported" {
				canSign = true
			}
		}
		out["canSign"] = canSign
	}
	lock, _ := a.lockOf(ctx, a.DB, wi.ID)
	out["lock"], _ = a.userRefs(ctx, lock)
	var watching bool
	a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM watchers WHERE record_id = $1 AND user_id = $2)`, wi.ID, u.ID).Scan(&watching)
	out["watching"] = watching
	a.DB.Exec(ctx, `INSERT INTO recent_views (user_id, record_id) VALUES ($1, $2) ON CONFLICT (user_id, record_id) DO UPDATE SET viewed_at = now()`, u.ID, wi.ID)
	return ok(w, out)
}

// apiNCRCreateCO creates a change order prefilled from a nonconformance and
// links it as "fixed by".
func (a *App) apiNCRCreateCO(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	wi, err := a.workForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !u.Can(PermCOCreate) {
		return errForbidden
	}
	found, _ := wi.Fields["found_on"].(string)
	title := truncate("Fix "+wi.ID+": "+wi.Title, 100)
	reason := fmt.Sprintf("Nonconformance %s: %s", wi.ID, wi.Description)
	ct := "Design change"
	in := coInput{Title: &title, ChangeType: &ct, Reason: &reason, HomeSpace: &wi.Space, Links: []string{wi.ID}}
	id, err := a.CreateCO(ctx, u, clientIP(r), in)
	if err != nil {
		return err
	}
	if found != "" {
		if e := cache.get(found); e != nil && e.Item.Status == model.StatusReleased {
			if err := a.AddCOItem(ctx, u, clientIP(r), id, found, "Revise"); err != nil {
				return err
			}
		}
	}
	a.Events.Publish(Event{Type: "record", ID: wi.ID, Space: wi.Space})
	return ok(w, map[string]string{"id": id})
}
