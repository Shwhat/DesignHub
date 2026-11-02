package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// Editable workflows (DATA-MODEL.md 4.7 and 7.21). One workflow per work
// item type, used in every space. Change orders keep their fixed states.

// State categories.
const (
	CatOpen      = "open"
	CatActive    = "active"
	CatDone      = "done"
	CatCancelled = "cancelled"
)

var stateCategories = []string{CatOpen, CatActive, CatDone, CatCancelled}
var stateColors = []string{"grey", "blue", "amber", "green", "red"}

// WState is one workflow state.
type WState struct {
	Name     string   `json:"name"`
	Category string   `json:"category"`
	Color    string   `json:"color"`
	Required []string `json:"required"`
	Rule     string   `json:"rule,omitempty"` // "disposition" on the NCR disposition state
}

// WTransition allows a move between two states.
type WTransition struct {
	From     string   `json:"from"` // state name or "*" for any
	To       string   `json:"to"`
	Roles    []string `json:"roles"` // space role names; empty means anyone who can edit work
	Assignee bool     `json:"assignee"`
}

// Workflow is the definition for one type.
type Workflow struct {
	Type        string        `json:"type"`
	Version     int           `json:"version"`
	Initial     string        `json:"initial"`
	States      []WState      `json:"states"`
	Transitions []WTransition `json:"transitions"`
	ChangedBy   string        `json:"changedBy,omitempty"`
	ChangedAt   *time.Time    `json:"changedAt,omitempty"`
}

var wfCache struct {
	sync.RWMutex
	m map[string]*Workflow
}

func (wf *Workflow) state(name string) *WState {
	for i := range wf.States {
		if wf.States[i].Name == name {
			return &wf.States[i]
		}
	}
	return nil
}

// category returns the category of a state, or open for unknown states.
func (wf *Workflow) category(name string) string {
	if s := wf.state(name); s != nil {
		return s.Category
	}
	return CatOpen
}

func (wf *Workflow) names() []string {
	out := make([]string, len(wf.States))
	for i, s := range wf.States {
		out[i] = s.Name
	}
	return out
}

func (wf *Workflow) ruleState(rule string) string {
	for _, s := range wf.States {
		if s.Rule == rule {
			return s.Name
		}
	}
	return ""
}

func transitionsFrom(m map[string][]string, order []string) []WTransition {
	var out []WTransition
	for _, from := range order {
		for _, to := range m[from] {
			out = append(out, WTransition{From: from, To: to, Assignee: true})
		}
	}
	return out
}

// defaultWorkflow returns the version 1 workflow of a type.
func defaultWorkflow(typ string) *Workflow {
	if typ == "ncr" {
		return &Workflow{Type: typ, Version: 1, Initial: "Reported",
			States: []WState{
				{Name: "Reported", Category: CatOpen, Color: "grey", Required: []string{}},
				{Name: "Dispositioned", Category: CatActive, Color: "blue", Required: []string{}, Rule: "disposition"},
				{Name: "Corrective Action", Category: CatActive, Color: "amber", Required: []string{}},
				{Name: "Verified", Category: CatActive, Color: "blue", Required: []string{"root_cause", "corrective_action"}},
				{Name: "Closed", Category: CatDone, Color: "green", Required: []string{"verification"}},
			},
			Transitions: transitionsFrom(map[string][]string{
				"Reported":          {"Dispositioned"},
				"Dispositioned":     {"Corrective Action", "Reported"},
				"Corrective Action": {"Verified", "Dispositioned"},
				"Verified":          {"Closed", "Corrective Action"},
			}, []string{"Reported", "Dispositioned", "Corrective Action", "Verified"})}
	}
	return &Workflow{Type: typ, Version: 1, Initial: "Open",
		States: []WState{
			{Name: "Open", Category: CatOpen, Color: "grey", Required: []string{}},
			{Name: "In Progress", Category: CatActive, Color: "blue", Required: []string{}},
			{Name: "In Review", Category: CatActive, Color: "amber", Required: []string{}},
			{Name: "Blocked", Category: CatActive, Color: "red", Required: []string{}},
			{Name: "Done", Category: CatDone, Color: "green", Required: []string{}},
			{Name: "Cancelled", Category: CatCancelled, Color: "grey", Required: []string{}},
		},
		Transitions: transitionsFrom(map[string][]string{
			"Open":        {"In Progress", "Done", "Blocked", "Cancelled"},
			"In Progress": {"Open", "In Review", "Done", "Blocked", "Cancelled"},
			"In Review":   {"In Progress", "Done", "Blocked", "Cancelled"},
			"Blocked":     {"Open", "In Progress", "Cancelled"},
			"Done":        {"Open"},
			"Cancelled":   {"Open"},
		}, []string{"Open", "In Progress", "In Review", "Blocked", "Done", "Cancelled"})}
}

// loadWorkflows reads the workflows, creating the defaults on first start.
func (a *App) loadWorkflows(ctx context.Context) error {
	m := map[string]*Workflow{}
	for typ := range workPrefix {
		var def []byte
		var ver int
		var by *string
		var at time.Time
		err := a.DB.QueryRow(ctx, `SELECT w.definition, w.version, us.display_name, w.changed_at FROM workflows w LEFT JOIN users us ON us.id = w.changed_by WHERE w.type = $1`, typ).
			Scan(&def, &ver, &by, &at)
		if err == pgx.ErrNoRows {
			wf := defaultWorkflow(typ)
			b, _ := json.Marshal(wf)
			if _, err := a.DB.Exec(ctx, `INSERT INTO workflows (type, version, definition) VALUES ($1, 1, $2) ON CONFLICT DO NOTHING`, typ, b); err != nil {
				return err
			}
			a.DB.Exec(ctx, `INSERT INTO workflow_versions (type, version, definition) VALUES ($1, 1, $2) ON CONFLICT DO NOTHING`, typ, b)
			m[typ] = wf
			continue
		}
		if err != nil {
			return err
		}
		wf := &Workflow{}
		if err := json.Unmarshal(def, wf); err != nil {
			return fmt.Errorf("workflow %s: %w", typ, err)
		}
		wf.Type, wf.Version, wf.ChangedAt = typ, ver, &at
		if by != nil {
			wf.ChangedBy = *by
		}
		m[typ] = wf
	}
	wfCache.Lock()
	wfCache.m = m
	wfCache.Unlock()
	return nil
}

func (a *App) workflow(typ string) *Workflow {
	wfCache.RLock()
	defer wfCache.RUnlock()
	if wf := wfCache.m[typ]; wf != nil {
		return wf
	}
	return defaultWorkflow(typ)
}

// closedStatusSQL is the condition for open items, by state category.
const openSQL = `w.state_cat IN ('open', 'active')`

// allowedNext lists the states u may move wi to.
func (a *App) allowedNext(u *User, wi *WorkItem) []string {
	wf := a.workflow(wi.Type)
	out := []string{}
	if !u.CanIn(wi.Space, PermWorkEdit) {
		return out
	}
	role := u.Spaces[wi.Space]
	isAssignee := wi.AssigneeID != nil && *wi.AssigneeID == u.ID
	for _, t := range wf.Transitions {
		if (t.From != wi.Status && t.From != "*") || t.To == wi.Status || contains(out, t.To) || wf.state(t.To) == nil {
			continue
		}
		okRole := len(t.Roles) == 0 || (role != nil && contains(t.Roles, role.Name)) || u.Can(PermSpaces) || (t.Assignee && isAssignee)
		if okRole {
			out = append(out, t.To)
		}
	}
	return out
}

// Fields that a state can require, by type. Common fields first.
var commonRequirable = []string{"assignee", "due_date", "description", "estimate", "sprint", "milestone", "labels"}

func requirableFields(typ string) []string {
	out := append([]string{}, commonRequirable...)
	out = append(out, workFields[typ]...)
	if typ == "ncr" {
		out = removeString(out, "disposition_approvers")
		out = removeString(out, "found_on_rev")
	}
	return out
}

var fieldLabels = map[string]string{
	"assignee": "Assignee", "due_date": "Due date", "description": "Description", "estimate": "Estimate", "sprint": "Sprint",
	"milestone": "Milestone", "labels": "Labels", "area": "Area", "severity": "Severity", "source": "Source", "start_date": "Start date",
	"target_date": "Target date", "found_on": "Found on", "serial": "Serial or lot number", "quantity": "Quantity affected",
	"where_found": "Where found", "manufacturer": "Manufacturer", "po": "PO or work order number", "requirement_not_met": "Requirement not met",
	"containment": "Containment", "disposition": "Disposition", "disposition_notes": "Disposition notes", "root_cause": "Root cause",
	"corrective_action": "Corrective action", "verification": "Verification of effectiveness", "requires_design_change": "Requires design change",
	"supplier": "Supplier",
}

func fieldFilled(wi *WorkItem, key string) bool {
	switch key {
	case "assignee":
		return wi.AssigneeID != nil
	case "due_date":
		return wi.DueDate != nil
	case "description":
		return strings.TrimSpace(wi.Description) != ""
	case "estimate":
		return wi.Estimate != nil
	case "sprint":
		return wi.Sprint != ""
	case "milestone":
		return wi.Milestone != ""
	case "labels":
		return len(wi.Labels) > 0
	}
	switch v := wi.Fields[key].(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case float64:
		return v != 0
	case bool:
		return true
	case nil:
		return false
	}
	return true
}

// missingFor lists the required fields of a state that wi lacks, including
// the fixed NCR rules.
func (a *App) missingFor(wi *WorkItem, state string) []string {
	wf := a.workflow(wi.Type)
	st := wf.state(state)
	var miss []string
	if st != nil {
		for _, f := range st.Required {
			if !fieldFilled(wi, f) {
				miss = append(miss, f)
			}
		}
		if wi.Type == "ncr" && st.Category == CatDone {
			for _, f := range []string{"root_cause", "corrective_action"} {
				if !fieldFilled(wi, f) && !contains(miss, f) {
					miss = append(miss, f)
				}
			}
		}
	}
	return miss
}

func labelList(keys []string) string {
	var ls []string
	for _, k := range keys {
		if l, ok := fieldLabels[k]; ok {
			ls = append(ls, strings.ToLower(l))
		} else {
			ls = append(ls, k)
		}
	}
	if len(ls) > 0 {
		ls[0] = strings.ToUpper(ls[0][:1]) + ls[0][1:]
	}
	return strings.Join(ls, ", ")
}

// ---------------------------------------------------------------------------
// Admin API

func (a *App) apiWorkflows(w http.ResponseWriter, r *http.Request) error {
	out := map[string]any{}
	for typ := range workPrefix {
		out[typ] = a.workflow(typ)
	}
	req := map[string][]string{}
	for typ := range workPrefix {
		req[typ] = requirableFields(typ)
	}
	return ok(w, map[string]any{"workflows": out, "fields": req, "labels": fieldLabels})
}

func (a *App) apiWorkflowHistory(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermRolesManage) {
		return errForbidden
	}
	rows, err := a.DB.Query(r.Context(), `SELECT v.version, COALESCE(us.display_name, 'DesignHub'), v.changed_at, v.definition FROM workflow_versions v
		LEFT JOIN users us ON us.id = v.changed_by WHERE v.type = $1 ORDER BY v.version DESC`, r.PathValue("type"))
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		Version    int             `json:"version"`
		By         string          `json:"by"`
		At         time.Time       `json:"at"`
		Definition json.RawMessage `json:"definition"`
	}
	out := []row{}
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.Version, &x.By, &x.At, &x.Definition); err != nil {
			return err
		}
		out = append(out, x)
	}
	return ok(w, out)
}

// validateWorkflow checks a new definition against the old one.
func validateWorkflow(old, wf *Workflow) error {
	if len(wf.States) == 0 {
		return badRequest("A workflow needs at least one state.")
	}
	seen := map[string]bool{}
	var hasOpen, hasDone bool
	for i := range wf.States {
		s := &wf.States[i]
		s.Name = strings.TrimSpace(s.Name)
		if s.Name == "" || len([]rune(s.Name)) > 30 || s.Name == "*" {
			return badRequest("State names are required (at most 30 characters).")
		}
		if seen[strings.ToLower(s.Name)] {
			return badRequest("State %s is listed twice.", s.Name)
		}
		seen[strings.ToLower(s.Name)] = true
		if !contains(stateCategories, s.Category) {
			return badRequest("State %s needs a category.", s.Name)
		}
		if !contains(stateColors, s.Color) {
			s.Color = "grey"
		}
		if s.Required == nil {
			s.Required = []string{}
		}
		for _, f := range s.Required {
			if !contains(requirableFields(wf.Type), f) {
				return badRequest("Unknown required field %s.", f)
			}
		}
		hasOpen = hasOpen || s.Category == CatOpen
		hasDone = hasDone || s.Category == CatDone
	}
	if !hasOpen || !hasDone {
		return badRequest("A workflow needs at least one Open state and one Done state.")
	}
	if wf.state(wf.Initial) == nil || wf.category(wf.Initial) != CatOpen {
		return badRequest("The initial state must be an Open state.")
	}
	if wf.Type == "ncr" {
		// The disposition state carries a fixed rule and cannot be removed.
		var n int
		for i := range wf.States {
			if old != nil && old.state(wf.States[i].Name) != nil && old.state(wf.States[i].Name).Rule == "disposition" {
				wf.States[i].Rule = "disposition"
			}
			if wf.States[i].Rule == "disposition" {
				n++
			}
		}
		if n != 1 {
			return badRequest("The Disposition state cannot be removed.")
		}
	} else {
		for i := range wf.States {
			wf.States[i].Rule = ""
		}
	}
	for i := range wf.Transitions {
		t := &wf.Transitions[i]
		if (t.From != "*" && wf.state(t.From) == nil) || wf.state(t.To) == nil {
			return badRequest("A transition refers to an unknown state.")
		}
		if t.Roles == nil {
			t.Roles = []string{}
		}
	}
	return nil
}

func (a *App) apiPutWorkflow(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	if !u.Can(PermRolesManage) {
		return errForbidden
	}
	typ := r.PathValue("type")
	if _, ok := workPrefix[typ]; !ok {
		return errNotFound
	}
	var in struct {
		Workflow Workflow          `json:"workflow"`
		Mapping  map[string]string `json:"mapping"` // removed state to new state
		Version  int               `json:"version"`
		Check    bool              `json:"check"` // only report affected items
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	old := a.workflow(typ)
	if in.Version != 0 && in.Version != old.Version {
		return conflict("Someone changed this workflow after you opened it. Reload to see their changes.", nil)
	}
	wf := in.Workflow
	wf.Type = typ
	for _, role := range collectRoles(wf.Transitions) {
		if ro := a.roleByName(role); ro == nil || ro.AppliesTo == "library" {
			return badRequest("Unknown space role %s.", role)
		}
	}
	if err := validateWorkflow(old, &wf); err != nil {
		return err
	}
	// Items in removed states must be mapped to a new state.
	counts := map[string]int{}
	rows, err := a.DB.Query(ctx, `SELECT status, count(*) FROM work_items WHERE type = $1 GROUP BY status`, typ)
	if err != nil {
		return err
	}
	for rows.Next() {
		var s string
		var n int
		if rows.Scan(&s, &n) == nil && wf.state(s) == nil {
			counts[s] = n
		}
	}
	rows.Close()
	if in.Check {
		return ok(w, map[string]any{"removed": counts})
	}
	for s := range counts {
		to := in.Mapping[s]
		if wf.state(to) == nil {
			return &Error{Status: http.StatusBadRequest, Code: "mapping", Message: "Choose a new state for the items in removed states.", Data: counts}
		}
	}
	wf.Version = old.Version + 1
	wf.ChangedBy, wf.ChangedAt = "", nil
	def, _ := json.Marshal(wf)
	err = a.tx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE workflows SET version = $2, definition = $3, changed_by = $4, changed_at = now() WHERE type = $1 AND version = $5`,
			typ, wf.Version, def, u.ID, old.Version)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return conflict("Someone changed this workflow after you opened it. Reload to see their changes.", nil)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO workflow_versions (type, version, definition, changed_by) VALUES ($1, $2, $3, $4)`, typ, wf.Version, def, u.ID); err != nil {
			return err
		}
		for from := range counts {
			to := in.Mapping[from]
			ids := []string{}
			rs, err := tx.Query(ctx, `UPDATE work_items SET status = $3, updated_at = now(), version = version + 1 WHERE type = $1 AND status = $2 RETURNING id`, typ, from, to)
			if err != nil {
				return err
			}
			for rs.Next() {
				var id string
				if rs.Scan(&id) == nil {
					ids = append(ids, id)
				}
			}
			rs.Close()
			cj, _ := json.Marshal(map[string]any{"status": map[string]string{"from": from, "to": to}, "workflow": fmt.Sprintf("version %d", wf.Version)})
			for _, id := range ids {
				if _, err := tx.Exec(ctx, `INSERT INTO work_history (item_id, user_id, changes) VALUES ($1, $2, $3)`, id, u.ID, cj); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `UPDATE records SET status = $2 WHERE id = $1`, id, to); err != nil {
					return err
				}
			}
		}
		// Keep every item's category in step with the new definition.
		for _, s := range wf.States {
			if _, err := tx.Exec(ctx, `UPDATE work_items SET state_cat = $3 WHERE type = $1 AND status = $2`, typ, s.Name, s.Category); err != nil {
				return err
			}
		}
		a.auditQ(ctx, tx, u, clientIP(r), "workflow.update", "", map[string]any{"type": typ, "version": wf.Version, "moved": counts})
		return nil
	})
	if err != nil {
		return err
	}
	if err := a.loadWorkflows(ctx); err != nil {
		return err
	}
	a.Events.Publish(Event{Type: "board"})
	return ok(w, map[string]int{"version": wf.Version})
}

func collectRoles(ts []WTransition) []string {
	var out []string
	for _, t := range ts {
		for _, r := range t.Roles {
			if !contains(out, r) {
				out = append(out, r)
			}
		}
	}
	return out
}
