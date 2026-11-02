package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Sprints, milestones, time tracking and the timeline (DATA-MODEL.md 7.22
// to 7.24, SCREENS.md 20 and 21).

// ---------------------------------------------------------------------------
// Sprints

type Sprint struct {
	ID        string          `json:"id"`
	Space     string          `json:"space"`
	Name      string          `json:"name"`
	Goal      string          `json:"goal"`
	Start     string          `json:"start"`
	End       string          `json:"end"`
	State     string          `json:"state"`
	Committed json.RawMessage `json:"committed"`
	Completed json.RawMessage `json:"completed"`
	Items     int             `json:"items"`
	Done      int             `json:"done"`
	Estimate  float64         `json:"estimate"`
	Logged    float64         `json:"logged"`
}

const sprintSelect = `SELECT s.id, s.space_id, s.name, s.goal, s.start_date::text, s.end_date::text, s.state, s.committed, s.completed,
	(SELECT count(*) FROM work_items w WHERE w.sprint_id = s.id AND w.state_cat <> 'cancelled'),
	(SELECT count(*) FROM work_items w WHERE w.sprint_id = s.id AND w.state_cat = 'done'),
	(SELECT COALESCE(sum(w.estimate), 0) FROM work_items w WHERE w.sprint_id = s.id AND w.state_cat <> 'cancelled')::float8,
	(SELECT COALESCE(sum(t.hours), 0) FROM time_entries t JOIN work_items w ON w.id = t.item_id WHERE w.sprint_id = s.id)::float8
	FROM sprints s`

func scanSprint(row pgx.Row) (*Sprint, error) {
	s := &Sprint{}
	err := row.Scan(&s.ID, &s.Space, &s.Name, &s.Goal, &s.Start, &s.End, &s.State, &s.Committed, &s.Completed, &s.Items, &s.Done, &s.Estimate, &s.Logged)
	return s, err
}

func (a *App) apiSprints(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	space := r.PathValue("id")
	if !u.InSpace(space) {
		return errNotFound
	}
	rows, err := a.DB.Query(r.Context(), sprintSelect+` WHERE s.space_id = $1
		ORDER BY CASE s.state WHEN 'Active' THEN 0 WHEN 'Planned' THEN 1 ELSE 2 END, CASE WHEN s.state = 'Closed' THEN -extract(epoch FROM s.start_date) ELSE extract(epoch FROM s.start_date) END`, space)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []*Sprint{}
	for rows.Next() {
		s, err := scanSprint(rows)
		if err != nil {
			return err
		}
		out = append(out, s)
	}
	return ok(w, out)
}

func (a *App) sprintForUser(ctx context.Context, u *User, id string) (*Sprint, error) {
	s, err := scanSprint(a.DB.QueryRow(ctx, sprintSelect+` WHERE s.id = $1`, id))
	if err != nil || !u.InSpace(s.Space) {
		return nil, errNotFound
	}
	return s, nil
}

func (a *App) apiSprint(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	s, err := a.sprintForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	items, err := a.queryWork(ctx, u, `w.sprint_id = $1`, s.ID)
	if err != nil {
		return err
	}
	return ok(w, map[string]any{"sprint": s, "items": items, "canPlan": u.CanIn(s.Space, PermPlan)})
}

type sprintInput struct {
	Space string  `json:"space"`
	Name  *string `json:"name"`
	Goal  *string `json:"goal"`
	Start *string `json:"start"`
	End   *string `json:"end"`
}

func checkDates(start, end string) error {
	s, err1 := time.Parse("2006-01-02", start)
	e, err2 := time.Parse("2006-01-02", end)
	if err1 != nil || err2 != nil {
		return badRequest("Start and end dates are required.")
	}
	if !e.After(s) {
		return badRequest("The end date must be after the start date.")
	}
	return nil
}

func (a *App) apiCreateSprint(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	var in sprintInput
	if err := decode(r, &in); err != nil {
		return err
	}
	if !u.CanIn(in.Space, PermPlan) {
		return errForbidden
	}
	name := strings.TrimSpace(deref(in.Name))
	if name == "" || len([]rune(name)) > 60 {
		return badRequest("Name is required (at most 60 characters).")
	}
	if err := checkDates(deref(in.Start), deref(in.End)); err != nil {
		return err
	}
	var id string
	err := a.tx(ctx, func(tx pgx.Tx) error {
		var dup int
		tx.QueryRow(ctx, `SELECT count(*) FROM sprints WHERE space_id = $1 AND lower(name) = lower($2)`, in.Space, name).Scan(&dup)
		if dup > 0 {
			return badRequest("A sprint named %s already exists in this space.", name)
		}
		var err error
		id, err = a.nextPrefixed(ctx, tx, "SPR", "sprint", u.ID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO sprints (id, space_id, name, goal, start_date, end_date, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			id, in.Space, name, strings.TrimSpace(deref(in.Goal)), *in.Start, *in.End, u.ID); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, clientIP(r), "sprint.create", id, map[string]string{"name": name})
		return nil
	})
	if err != nil {
		return err
	}
	a.Events.Publish(Event{Type: "board", Space: in.Space})
	return ok(w, map[string]string{"id": id})
}

func (a *App) apiUpdateSprint(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	s, err := a.sprintForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !u.CanIn(s.Space, PermPlan) {
		return errForbidden
	}
	if s.State == "Closed" {
		return badRequest("Closed sprints cannot be changed.")
	}
	var in sprintInput
	if err := decode(r, &in); err != nil {
		return err
	}
	if in.Name != nil {
		s.Name = strings.TrimSpace(*in.Name)
		if s.Name == "" || len([]rune(s.Name)) > 60 {
			return badRequest("Name is required (at most 60 characters).")
		}
	}
	if in.Goal != nil {
		s.Goal = strings.TrimSpace(*in.Goal)
	}
	if in.Start != nil {
		s.Start = *in.Start
	}
	if in.End != nil {
		s.End = *in.End
	}
	if err := checkDates(s.Start, s.End); err != nil {
		return err
	}
	if _, err := a.DB.Exec(ctx, `UPDATE sprints SET name = $2, goal = $3, start_date = $4, end_date = $5 WHERE id = $1`, s.ID, s.Name, s.Goal, s.Start, s.End); err != nil {
		if strings.Contains(err.Error(), "sprints_name") {
			return badRequest("A sprint named %s already exists in this space.", s.Name)
		}
		return err
	}
	a.Events.Publish(Event{Type: "board", Space: s.Space})
	return ok(w, nil)
}

// apiStartSprint makes a Planned sprint Active and records what it commits to.
func (a *App) apiStartSprint(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	s, err := a.sprintForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !u.CanIn(s.Space, PermPlan) {
		return errForbidden
	}
	if s.State != "Planned" {
		return badRequest("Only a Planned sprint can be started.")
	}
	var active int
	a.DB.QueryRow(ctx, `SELECT count(*) FROM sprints WHERE space_id = $1 AND state = 'Active'`, s.Space).Scan(&active)
	if active > 0 {
		return badRequest("This space already has an active sprint. Close it first.")
	}
	items, err := a.queryWork(ctx, u, `w.sprint_id = $1 AND w.state_cat <> 'cancelled'`, s.ID)
	if err != nil {
		return err
	}
	ids := []string{}
	var est float64
	var assignees []string
	for _, it := range items {
		ids = append(ids, it.ID)
		if it.Estimate != nil {
			est += *it.Estimate
		}
		if it.AssigneeID != nil && !contains(assignees, *it.AssigneeID) {
			assignees = append(assignees, *it.AssigneeID)
		}
	}
	committed, _ := json.Marshal(map[string]any{"items": ids, "estimate": est, "at": time.Now().UTC()})
	if _, err := a.DB.Exec(ctx, `UPDATE sprints SET state = 'Active', committed = $2 WHERE id = $1 AND state = 'Planned'`, s.ID, committed); err != nil {
		return err
	}
	a.Audit(ctx, u, clientIP(r), "sprint.start", s.ID, map[string]any{"items": len(ids)})
	for _, id := range assignees {
		a.notify(ctx, u, id, "Sprint started", "", fmt.Sprintf("Sprint %s started: %s", s.Name, s.Goal))
	}
	a.Events.Publish(Event{Type: "board", Space: s.Space})
	return ok(w, nil)
}

// apiCloseSprint closes the Active sprint and moves unfinished items to the
// next Planned sprint or to no sprint.
func (a *App) apiCloseSprint(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	s, err := a.sprintForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !u.CanIn(s.Space, PermPlan) {
		return errForbidden
	}
	if s.State != "Active" {
		return badRequest("Only the active sprint can be closed.")
	}
	var in struct {
		MoveTo string `json:"moveTo"` // a Planned sprint id, or empty for no sprint
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	if in.MoveTo != "" {
		var space, state string
		if err := a.DB.QueryRow(ctx, `SELECT space_id, state FROM sprints WHERE id = $1`, in.MoveTo).Scan(&space, &state); err != nil || space != s.Space || state != "Planned" {
			return badRequest("Unfinished items can move only to a Planned sprint in this space.")
		}
	}
	var assignees []string
	err = a.tx(ctx, func(tx pgx.Tx) error {
		var done []string
		var logged float64
		rows, err := tx.Query(ctx, `SELECT id FROM work_items WHERE sprint_id = $1 AND state_cat = 'done'`, s.ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				done = append(done, id)
			}
		}
		rows.Close()
		tx.QueryRow(ctx, `SELECT COALESCE(sum(t.hours), 0)::float8 FROM time_entries t JOIN work_items w ON w.id = t.item_id WHERE w.sprint_id = $1`, s.ID).Scan(&logged)
		moved := []string{}
		rows, err = tx.Query(ctx, `UPDATE work_items SET sprint_id = $2, updated_at = now(), version = version + 1
			WHERE sprint_id = $1 AND state_cat IN ('open', 'active') RETURNING id, assignee_id::text`, s.ID, in.MoveTo)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id string
			var as *string
			if rows.Scan(&id, &as) == nil {
				moved = append(moved, id)
				if as != nil && !contains(assignees, *as) {
					assignees = append(assignees, *as)
				}
			}
		}
		rows.Close()
		cj, _ := json.Marshal(map[string]any{"sprint": map[string]string{"from": s.ID, "to": in.MoveTo}})
		for _, id := range moved {
			if _, err := tx.Exec(ctx, `INSERT INTO work_history (item_id, user_id, changes) VALUES ($1, $2, $3)`, id, u.ID, cj); err != nil {
				return err
			}
		}
		completed, _ := json.Marshal(map[string]any{"done": done, "logged": logged, "moved": moved, "movedTo": in.MoveTo, "at": time.Now().UTC()})
		if _, err := tx.Exec(ctx, `UPDATE sprints SET state = 'Closed', completed = $2 WHERE id = $1`, s.ID, completed); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, clientIP(r), "sprint.close", s.ID, map[string]any{"done": len(done), "moved": len(moved), "movedTo": in.MoveTo})
		return nil
	})
	if err != nil {
		return err
	}
	for _, id := range assignees {
		a.notify(ctx, u, id, "Sprint closed", "", fmt.Sprintf("Sprint %s closed. Your unfinished items moved on.", s.Name))
	}
	a.Events.Publish(Event{Type: "board", Space: s.Space})
	return ok(w, nil)
}

// apiPlanItems sets the sprint of several items at once (backlog planning).
func (a *App) apiPlanItems(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	var in struct {
		Items  []string `json:"items"`
		Sprint string   `json:"sprint"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	for _, id := range in.Items {
		wi, err := a.workForUser(ctx, u, id)
		if err != nil {
			return err
		}
		if !u.CanIn(wi.Space, PermWorkEdit) {
			return errForbidden
		}
		before := wi.Sprint
		sp := in.Sprint
		wi2 := *wi
		if err := a.applyWorkInput(ctx, &wi2, &workInput{Sprint: &sp}, false); err != nil {
			return err
		}
		if before == sp {
			continue
		}
		err = a.tx(ctx, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE work_items SET sprint_id = $2, updated_at = now(), version = version + 1 WHERE id = $1`, id, sp); err != nil {
				return err
			}
			cj, _ := json.Marshal(map[string]any{"sprint": map[string]string{"from": before, "to": sp}})
			_, err := tx.Exec(ctx, `INSERT INTO work_history (item_id, user_id, changes) VALUES ($1, $2, $3)`, id, u.ID, cj)
			return err
		})
		if err != nil {
			return err
		}
		a.Events.Publish(Event{Type: "board", ID: id, Space: wi.Space})
	}
	return ok(w, nil)
}

// ---------------------------------------------------------------------------
// Milestones

type Milestone struct {
	ID          string  `json:"id"`
	Space       string  `json:"space"`
	Name        string  `json:"name"`
	Target      string  `json:"target"`
	Description string  `json:"description"`
	OwnerID     string  `json:"ownerId"`
	Owner       string  `json:"owner"`
	CompletedOn *string `json:"completedOn"`
	Items       int     `json:"items"`
	Done        int     `json:"done"`
	Estimate    float64 `json:"estimate"`
	DoneEst     float64 `json:"doneEstimate"`
	Percent     int     `json:"percent"`
	Health      string  `json:"health"`
}

const milestoneSelect = `SELECT m.id, m.space_id, m.name, m.target_date::text, m.description, m.owner_id::text, us.display_name, m.completed_on::text,
	(SELECT count(*) FROM work_items w WHERE w.milestone_id = m.id AND w.state_cat <> 'cancelled'),
	(SELECT count(*) FROM work_items w WHERE w.milestone_id = m.id AND w.state_cat = 'done'),
	(SELECT COALESCE(sum(w.estimate), 0) FROM work_items w WHERE w.milestone_id = m.id AND w.state_cat <> 'cancelled')::float8,
	(SELECT COALESCE(sum(w.estimate), 0) FROM work_items w WHERE w.milestone_id = m.id AND w.state_cat = 'done')::float8
	FROM milestones m JOIN users us ON us.id = m.owner_id`

func scanMilestone(row pgx.Row) (*Milestone, error) {
	m := &Milestone{}
	if err := row.Scan(&m.ID, &m.Space, &m.Name, &m.Target, &m.Description, &m.OwnerID, &m.Owner, &m.CompletedOn, &m.Items, &m.Done, &m.Estimate, &m.DoneEst); err != nil {
		return nil, err
	}
	m.computeHealth(time.Now())
	return m, nil
}

// computeHealth sets progress and health (DATA-MODEL.md 7.23).
func (m *Milestone) computeHealth(now time.Time) {
	if m.Items > 0 {
		m.Percent = m.Done * 100 / m.Items
	}
	target, _ := time.Parse("2006-01-02", m.Target)
	today, _ := time.Parse("2006-01-02", now.Format("2006-01-02"))
	days := int(target.Sub(today).Hours() / 24)
	switch {
	case m.Items > 0 && m.Done == m.Items:
		m.Health = "Done"
	case days < 0:
		m.Health = "Late"
	case days <= 7 && m.Percent < 80:
		m.Health = "At risk"
	default:
		m.Health = "On track"
	}
}

func (a *App) apiMilestones(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	space := r.PathValue("id")
	if !u.InSpace(space) {
		return errNotFound
	}
	rows, err := a.DB.Query(r.Context(), milestoneSelect+` WHERE m.space_id = $1 ORDER BY m.target_date`, space)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []*Milestone{}
	for rows.Next() {
		m, err := scanMilestone(rows)
		if err != nil {
			return err
		}
		out = append(out, m)
	}
	return ok(w, out)
}

func (a *App) milestoneForUser(ctx context.Context, u *User, id string) (*Milestone, error) {
	m, err := scanMilestone(a.DB.QueryRow(ctx, milestoneSelect+` WHERE m.id = $1`, id))
	if err != nil || !u.InSpace(m.Space) {
		return nil, errNotFound
	}
	return m, nil
}

func (a *App) apiMilestone(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	m, err := a.milestoneForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	items, err := a.queryWork(ctx, u, `w.milestone_id = $1`, m.ID)
	if err != nil {
		return err
	}
	return ok(w, map[string]any{"milestone": m, "items": items, "canPlan": u.CanIn(m.Space, PermPlan)})
}

type milestoneInput struct {
	Space       string  `json:"space"`
	Name        *string `json:"name"`
	Target      *string `json:"target"`
	Description *string `json:"description"`
	OwnerID     *string `json:"ownerId"`
}

func (a *App) apiCreateMilestone(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	var in milestoneInput
	if err := decode(r, &in); err != nil {
		return err
	}
	if !u.CanIn(in.Space, PermPlan) {
		return errForbidden
	}
	name := strings.TrimSpace(deref(in.Name))
	if name == "" || len([]rune(name)) > 80 {
		return badRequest("Name is required (at most 80 characters).")
	}
	if _, err := time.Parse("2006-01-02", deref(in.Target)); err != nil {
		return badRequest("Target date is required.")
	}
	owner := u.ID
	if in.OwnerID != nil && *in.OwnerID != "" {
		owner = *in.OwnerID
	}
	var id string
	err := a.tx(ctx, func(tx pgx.Tx) error {
		var err error
		id, err = a.nextPrefixed(ctx, tx, "MS", "milestone", u.ID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO milestones (id, space_id, name, target_date, description, owner_id, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			id, in.Space, name, *in.Target, deref(in.Description), owner, u.ID); err != nil {
			return badRequest("Unknown owner.")
		}
		_, err = tx.Exec(ctx, `INSERT INTO records (id, kind, title, status, space_id, owner_id, origin) VALUES ($1, 'milestone', $2, 'On track', $3, $4, 'db')`, id, name, in.Space, owner)
		return err
	})
	if err != nil {
		return err
	}
	a.Audit(ctx, u, clientIP(r), "milestone.create", id, map[string]string{"name": name})
	return ok(w, map[string]string{"id": id})
}

func (a *App) apiUpdateMilestone(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	m, err := a.milestoneForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !u.CanIn(m.Space, PermPlan) {
		return errForbidden
	}
	var in milestoneInput
	if err := decode(r, &in); err != nil {
		return err
	}
	if in.Name != nil {
		m.Name = strings.TrimSpace(*in.Name)
		if m.Name == "" {
			return badRequest("Name is required.")
		}
	}
	if in.Target != nil {
		if _, err := time.Parse("2006-01-02", *in.Target); err != nil {
			return badRequest("Target date is required.")
		}
		m.Target = *in.Target
	}
	if in.Description != nil {
		m.Description = *in.Description
	}
	if in.OwnerID != nil && *in.OwnerID != "" {
		m.OwnerID = *in.OwnerID
	}
	if _, err := a.DB.Exec(ctx, `UPDATE milestones SET name = $2, target_date = $3, description = $4, owner_id = $5, risk_noticed = false WHERE id = $1`,
		m.ID, m.Name, m.Target, m.Description, m.OwnerID); err != nil {
		return badRequest("Unknown owner.")
	}
	a.DB.Exec(ctx, `UPDATE records SET title = $2, owner_id = $3 WHERE id = $1`, m.ID, m.Name, m.OwnerID)
	a.Events.Publish(Event{Type: "board", Space: m.Space})
	return ok(w, nil)
}

// milestoneChecks records completion and notifies owners of milestones that
// become at risk. Run hourly.
func (a *App) milestoneChecks(ctx context.Context) {
	rows, err := a.DB.Query(ctx, milestoneSelect+` WHERE m.completed_on IS NULL OR NOT m.risk_noticed`)
	if err != nil {
		return
	}
	var ms []*Milestone
	for rows.Next() {
		if m, err := scanMilestone(rows); err == nil {
			ms = append(ms, m)
		}
	}
	rows.Close()
	for _, m := range ms {
		a.DB.Exec(ctx, `UPDATE records SET status = $2 WHERE id = $1`, m.ID, m.Health)
		if m.Health == "Done" && m.CompletedOn == nil {
			a.DB.Exec(ctx, `UPDATE milestones SET completed_on = current_date WHERE id = $1`, m.ID)
		}
		if m.Health != "Done" && m.CompletedOn != nil {
			a.DB.Exec(ctx, `UPDATE milestones SET completed_on = NULL WHERE id = $1`, m.ID)
		}
		if m.Health == "At risk" {
			tag, err := a.DB.Exec(ctx, `UPDATE milestones SET risk_noticed = true WHERE id = $1 AND NOT risk_noticed`, m.ID)
			if err == nil && tag.RowsAffected() == 1 {
				a.notify(ctx, nil, m.OwnerID, "Milestone at risk", m.ID, fmt.Sprintf("%s %s is at risk: %d percent done, target %s", m.ID, m.Name, m.Percent, m.Target))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Time entries

type TimeEntry struct {
	ID       string     `json:"id"`
	ItemID   string     `json:"itemId"`
	Title    string     `json:"title,omitempty"`
	UserID   string     `json:"userId"`
	User     string     `json:"user"`
	Day      string     `json:"day"`
	Hours    float64    `json:"hours"`
	Note     string     `json:"note"`
	EditedAt *time.Time `json:"editedAt"`
	CanEdit  bool       `json:"canEdit"`
}

func (a *App) apiTimeEntries(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	wi, err := a.workForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	rows, err := a.DB.Query(ctx, `SELECT t.id::text, t.item_id, t.user_id::text, us.display_name, t.day::text, t.hours::float8, t.note, t.edited_at
		FROM time_entries t JOIN users us ON us.id = t.user_id WHERE t.item_id = $1 ORDER BY t.day DESC, t.created_at DESC`, wi.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []TimeEntry{}
	for rows.Next() {
		var t TimeEntry
		if err := rows.Scan(&t.ID, &t.ItemID, &t.UserID, &t.User, &t.Day, &t.Hours, &t.Note, &t.EditedAt); err != nil {
			return err
		}
		t.CanEdit = t.UserID == u.ID || u.Can(PermTimeAll)
		out = append(out, t)
	}
	return ok(w, map[string]any{"entries": out, "estimate": wi.Estimate, "logged": wi.Logged,
		"canLog": wi.Type != "epic" && u.CanIn(wi.Space, PermTimeLog) && !u.IsVendor()})
}

type timeInput struct {
	Item  string   `json:"item"`
	Day   string   `json:"day"`
	Hours *float64 `json:"hours"`
	Note  *string  `json:"note"`
}

func validTime(day string, hours float64) error {
	d, err := time.Parse("2006-01-02", day)
	if err != nil {
		return badRequest("Date is required.")
	}
	if d.After(time.Now().Add(14 * time.Hour)) {
		return badRequest("Time cannot be logged for a future date.")
	}
	if hours < 0.25 || hours > 24 || hours*4 != float64(int(hours*4)) {
		return badRequest("Hours must be between 0.25 and 24, in quarter-hour steps.")
	}
	return nil
}

func (a *App) apiLogTime(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	var in timeInput
	if err := decode(r, &in); err != nil {
		return err
	}
	wi, err := a.workForUser(ctx, u, in.Item)
	if err != nil {
		return err
	}
	if wi.Type == "epic" {
		return badRequest("Log time on the items of an epic, not on the epic.")
	}
	if !u.CanIn(wi.Space, PermTimeLog) || u.IsVendor() {
		return errForbidden
	}
	if in.Hours == nil {
		return badRequest("Hours are required.")
	}
	if err := validTime(in.Day, *in.Hours); err != nil {
		return err
	}
	var id string
	err = a.tx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO time_entries (item_id, user_id, day, hours, note) VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
			wi.ID, u.ID, in.Day, *in.Hours, strings.TrimSpace(deref(in.Note))).Scan(&id); err != nil {
			return err
		}
		cj, _ := json.Marshal(map[string]any{"time": map[string]any{"logged": *in.Hours, "day": in.Day}})
		_, err := tx.Exec(ctx, `INSERT INTO work_history (item_id, user_id, changes) VALUES ($1, $2, $3)`, wi.ID, u.ID, cj)
		return err
	})
	if err != nil {
		return err
	}
	a.Events.Publish(Event{Type: "record", ID: wi.ID, Space: wi.Space})
	return ok(w, map[string]string{"id": id})
}

func (a *App) timeEntryFor(ctx context.Context, u *User, id string) (*TimeEntry, *WorkItem, error) {
	var t TimeEntry
	if err := a.DB.QueryRow(ctx, `SELECT id::text, item_id, user_id::text, day::text, hours::float8, note FROM time_entries WHERE id = $1`, id).
		Scan(&t.ID, &t.ItemID, &t.UserID, &t.Day, &t.Hours, &t.Note); err != nil {
		return nil, nil, errNotFound
	}
	wi, err := a.workForUser(ctx, u, t.ItemID)
	if err != nil {
		return nil, nil, err
	}
	if t.UserID != u.ID && !u.Can(PermTimeAll) {
		return nil, nil, &Error{Status: http.StatusForbidden, Code: "forbidden", Message: "You can change only your own time entries."}
	}
	return &t, wi, nil
}

func (a *App) apiUpdateTime(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	t, wi, err := a.timeEntryFor(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	var in timeInput
	if err := decode(r, &in); err != nil {
		return err
	}
	before := *t
	if in.Day != "" {
		t.Day = in.Day
	}
	if in.Hours != nil {
		t.Hours = *in.Hours
	}
	if in.Note != nil {
		t.Note = strings.TrimSpace(*in.Note)
	}
	if err := validTime(t.Day, t.Hours); err != nil {
		return err
	}
	err = a.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE time_entries SET day = $2, hours = $3, note = $4, edited_at = now() WHERE id = $1`, t.ID, t.Day, t.Hours, t.Note); err != nil {
			return err
		}
		cj, _ := json.Marshal(map[string]any{"time": map[string]any{"from": map[string]any{"day": before.Day, "hours": before.Hours}, "to": map[string]any{"day": t.Day, "hours": t.Hours}}})
		_, err := tx.Exec(ctx, `INSERT INTO work_history (item_id, user_id, changes) VALUES ($1, $2, $3)`, wi.ID, u.ID, cj)
		return err
	})
	if err != nil {
		return err
	}
	a.Events.Publish(Event{Type: "record", ID: wi.ID, Space: wi.Space})
	return ok(w, nil)
}

func (a *App) apiDeleteTime(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	t, wi, err := a.timeEntryFor(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	err = a.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM time_entries WHERE id = $1`, t.ID); err != nil {
			return err
		}
		cj, _ := json.Marshal(map[string]any{"time": map[string]any{"removed": t.Hours, "day": t.Day}})
		_, err := tx.Exec(ctx, `INSERT INTO work_history (item_id, user_id, changes) VALUES ($1, $2, $3)`, wi.ID, u.ID, cj)
		return err
	})
	if err != nil {
		return err
	}
	a.Events.Publish(Event{Type: "record", ID: wi.ID, Space: wi.Space})
	return ok(w, nil)
}

// apiMyTime lists the user's entries between two dates.
func (a *App) apiMyTime(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if err := checkDatesInclusive(from, to); err != nil {
		return err
	}
	rows, err := a.DB.Query(r.Context(), `SELECT t.id::text, t.item_id, w.title, t.user_id::text, us.display_name, t.day::text, t.hours::float8, t.note, t.edited_at
		FROM time_entries t JOIN users us ON us.id = t.user_id JOIN work_items w ON w.id = t.item_id
		WHERE t.user_id = $1 AND t.day BETWEEN $2 AND $3 ORDER BY t.day, t.created_at`, u.ID, from, to)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []TimeEntry{}
	for rows.Next() {
		var t TimeEntry
		if err := rows.Scan(&t.ID, &t.ItemID, &t.Title, &t.UserID, &t.User, &t.Day, &t.Hours, &t.Note, &t.EditedAt); err != nil {
			return err
		}
		t.CanEdit = true
		out = append(out, t)
	}
	return ok(w, out)
}

func checkDatesInclusive(from, to string) error {
	f, err1 := time.Parse("2006-01-02", from)
	t, err2 := time.Parse("2006-01-02", to)
	if err1 != nil || err2 != nil || t.Before(f) {
		return badRequest("Choose a date range.")
	}
	if t.Sub(f) > 3*366*24*time.Hour {
		return badRequest("Choose a range of at most three years.")
	}
	return nil
}

type timeReportRow struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Estimate float64 `json:"estimate"`
	Logged   float64 `json:"logged"`
	Diff     float64 `json:"difference"`
	Entries  int     `json:"entries"`
}

// apiTimeReport totals time in a space by person, item, epic or sprint.
// Estimates count once per item, for items with time in the range.
func (a *App) apiTimeReport(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	space := r.PathValue("id")
	if !u.CanIn(space, PermTimeReport) {
		return errForbidden
	}
	q := r.URL.Query()
	from, to := q.Get("from"), q.Get("to")
	if err := checkDatesInclusive(from, to); err != nil {
		return err
	}
	group := q.Get("group")
	where := `w.space_id = $1 AND t.day BETWEEN $2 AND $3`
	args := []any{space, from, to}
	add := func(col, val string) {
		if val == "" {
			return
		}
		args = append(args, strings.Split(val, ","))
		where += fmt.Sprintf(` AND %s = ANY($%d)`, col, len(args))
	}
	add("t.user_id::text", q.Get("people"))
	add("w.epic_id", q.Get("epic"))
	add("w.sprint_id", q.Get("sprint"))
	add("w.milestone_id", q.Get("milestone"))
	add("w.type", q.Get("type"))
	keyCol, labelCol := "t.user_id::text", "us.display_name"
	switch group {
	case "item":
		keyCol, labelCol = "w.id", "w.id || ' ' || w.title"
	case "epic":
		keyCol, labelCol = "w.epic_id", "COALESCE((SELECT e.id || ' ' || e.title FROM work_items e WHERE e.id = w.epic_id), 'No epic')"
	case "sprint":
		keyCol, labelCol = "w.sprint_id", "COALESCE((SELECT s.name FROM sprints s WHERE s.id = w.sprint_id), 'No sprint')"
	default:
		group = "person"
	}
	locks, err := a.locks(ctx)
	if err != nil {
		return err
	}
	rows, err := a.DB.Query(ctx, `SELECT `+keyCol+`, `+labelCol+`, w.id, w.type, w.estimate::float8, t.hours::float8
		FROM time_entries t JOIN work_items w ON w.id = t.item_id JOIN users us ON us.id = t.user_id WHERE `+where, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	byKey := map[string]*timeReportRow{}
	order := []string{}
	estSeen := map[string]bool{}
	var total timeReportRow
	for rows.Next() {
		var key, label, item, typ string
		var est *float64
		var hours float64
		if err := rows.Scan(&key, &label, &item, &typ, &est, &hours); err != nil {
			return err
		}
		if !visible(u, item, typ, space, locks[item]) {
			continue
		}
		row := byKey[key]
		if row == nil {
			row = &timeReportRow{Key: key, Label: label}
			byKey[key] = row
			order = append(order, key)
		}
		row.Logged += hours
		row.Entries++
		total.Logged += hours
		total.Entries++
		if est != nil && !estSeen[key+"/"+item] {
			estSeen[key+"/"+item] = true
			row.Estimate += *est
		}
		if est != nil && !estSeen["total/"+item] {
			estSeen["total/"+item] = true
			total.Estimate += *est
		}
	}
	out := []timeReportRow{}
	for _, k := range order {
		row := byKey[k]
		row.Diff = row.Logged - row.Estimate
		out = append(out, *row)
	}
	total.Diff = total.Logged - total.Estimate
	total.Label = "Total"
	return ok(w, map[string]any{"rows": out, "total": total, "group": group})
}

// ---------------------------------------------------------------------------
// Timeline

func (a *App) apiTimeline(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	space := r.PathValue("id")
	if !u.InSpace(space) {
		return errNotFound
	}
	items, err := a.queryWork(ctx, u, `w.space_id = $1 AND w.state_cat <> 'cancelled'`, space)
	if err != nil {
		return err
	}
	type bar struct {
		workRow
		Start string `json:"start"`
		End   string `json:"end"`
	}
	bars := []bar{}
	starts := map[string]string{}
	rows, err := a.DB.Query(ctx, `SELECT id, created_at::date::text, COALESCE(fields->>'start_date', ''), COALESCE(fields->>'target_date', '') FROM work_items WHERE space_id = $1`, space)
	if err != nil {
		return err
	}
	ends := map[string]string{}
	for rows.Next() {
		var id, created, s, e string
		if rows.Scan(&id, &created, &s, &e) == nil {
			starts[id] = created
			if s != "" {
				starts[id] = s
			}
			if e != "" {
				ends[id] = e
			}
		}
	}
	rows.Close()
	for _, it := range items {
		b := bar{workRow: it, Start: starts[it.ID]}
		if it.Type == "epic" {
			b.End = ends[it.ID]
		} else if it.DueDate != nil {
			b.End = *it.DueDate
		}
		if b.End == "" {
			continue
		}
		if b.Start > b.End {
			b.Start = b.End
		}
		bars = append(bars, b)
	}
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	deps := []map[string]string{}
	rows, err = a.DB.Query(ctx, `SELECT l.src, l.dst FROM links l JOIN work_items s ON s.id = l.src WHERE l.type = 'blocks' AND s.space_id = $1`, space)
	if err != nil {
		return err
	}
	for rows.Next() {
		var s, d string
		if rows.Scan(&s, &d) == nil && vis(s) && vis(d) {
			deps = append(deps, map[string]string{"from": s, "to": d})
		}
	}
	rows.Close()
	ms := []*Milestone{}
	rows, err = a.DB.Query(ctx, milestoneSelect+` WHERE m.space_id = $1 ORDER BY m.target_date`, space)
	if err != nil {
		return err
	}
	for rows.Next() {
		if m, err := scanMilestone(rows); err == nil {
			ms = append(ms, m)
		}
	}
	rows.Close()
	sp := []*Sprint{}
	rows, err = a.DB.Query(ctx, sprintSelect+` WHERE s.space_id = $1 ORDER BY s.start_date`, space)
	if err != nil {
		return err
	}
	for rows.Next() {
		if s, err := scanSprint(rows); err == nil {
			sp = append(sp, s)
		}
	}
	rows.Close()
	return ok(w, map[string]any{"bars": bars, "milestones": ms, "sprints": sp, "dependencies": deps, "canEdit": u.CanIn(space, PermWorkEdit)})
}
