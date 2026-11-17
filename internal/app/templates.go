package app

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"designhub/internal/model"
)

// Page templates (DATA-MODEL.md 7.25). A template is company-wide (empty
// space) or belongs to one space. Pages created from a template keep a
// reference to it; later template changes do not change existing pages.

type Template struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Space        string            `json:"space"`
	Description  string            `json:"description"`
	PageType     string            `json:"pageType"`
	TitlePattern string            `json:"titlePattern"`
	Body         string            `json:"body"`
	Form         []model.FormField `json:"form"`
	Active       bool              `json:"active"`
	ChangedBy    string            `json:"changedBy"`
	ChangedAt    time.Time         `json:"changedAt"`
	Version      int               `json:"version"`
}

const templateSelect = `SELECT t.id, t.name, t.space_id, t.description, t.page_type, t.title_pattern, t.body, t.form, t.active,
	COALESCE(us.display_name, 'DesignHub'), t.changed_at, t.version FROM templates t LEFT JOIN users us ON us.id = t.changed_by`

func scanTemplate(row pgx.Row) (*Template, error) {
	t := &Template{}
	var form []byte
	err := row.Scan(&t.ID, &t.Name, &t.Space, &t.Description, &t.PageType, &t.TitlePattern, &t.Body, &form, &t.Active, &t.ChangedBy, &t.ChangedAt, &t.Version)
	json.Unmarshal(form, &t.Form)
	if t.Form == nil {
		t.Form = []model.FormField{}
	}
	return t, err
}

// Field helpers for the initial templates.
func fText(key, label, help string, req bool) model.FormField {
	return model.FormField{Key: key, Label: label, Type: "text", Help: help, Required: req}
}
func fArea(key, label, help string, req bool) model.FormField {
	return model.FormField{Key: key, Label: label, Type: "textarea", Help: help, Required: req}
}
func fDate(key, label string) model.FormField {
	return model.FormField{Key: key, Label: label, Type: "date", Default: "{date}", Required: true}
}
func fMe(key, label string) model.FormField {
	return model.FormField{Key: key, Label: label, Type: "user", Default: "{user}", Required: true}
}
func fSelect(key, label, def string, req bool, options ...string) model.FormField {
	return model.FormField{Key: key, Label: label, Type: "select", Default: def, Required: req, Options: options}
}
func fRecord(key, label, kinds, help string, req bool) model.FormField {
	return model.FormField{Key: key, Label: label, Type: "record", Kinds: kinds, Help: help, Required: req}
}
func fList(key, label, help string, req bool) model.FormField {
	return model.FormField{Key: key, Label: label, Type: "list", Help: help, Required: req}
}
func fTable(key, label, help string, req bool, cols ...model.FormField) model.FormField {
	return model.FormField{Key: key, Label: label, Type: "table", Help: help, Required: req, Columns: cols}
}

var initialTemplates = []Template{
	{Name: "Design review", PageType: "Design review", TitlePattern: "Design review {date}",
		Description: "Review of a design against its requirements, with findings and actions.",
		Body: "# {title}\n\n| | |\n|---|---|\n| Date | {{date}} |\n| Facilitator | {{facilitator}} |\n| Space | {space} |\n\n## Scope\n\n{{scope}}\n\n{{scope-notes}}\n\n" +
			"## Attendees\n\n{{attendees}}\n\n## Requirements checked\n\n{{requirements}}\n\n## Findings\n\n{{findings}}\n\n## Decision\n\n{{decision}}\n\n{{decision-notes}}\n\nFollow-up review needed: {{follow-up}}\n",
		Form: []model.FormField{
			fDate("date", "Date"),
			fMe("facilitator", "Facilitator"),
			fRecord("scope", "Item under review", "part,assembly,requirement,procedure,document", "Assembly or item under review.", true),
			fArea("scope-notes", "Scope notes", "What part of the design is covered.", false),
			{Key: "attendees", Label: "Attendees", Type: "users", Required: true},
			fTable("requirements", "Requirements checked", "", true,
				fRecord("requirement", "Requirement", "requirement", "", true),
				fSelect("result", "Result", "", true, "Met", "Partly met", "Not met", "Not checked"),
				fText("notes", "Notes", "", false)),
			fTable("findings", "Findings", "", false,
				model.FormField{Key: "n", Label: "#", Type: "index"},
				fText("finding", "Finding", "", true),
				fSelect("severity", "Severity", "", true, "Critical", "Major", "Minor", "Observation"),
				fText("action", "Action", "", false)),
			fSelect("decision", "Decision", "", true, "Approved", "Approved with actions", "Not approved"),
			fArea("decision-notes", "Decision notes", "Conditions or reasons for the decision.", false),
			{Key: "follow-up", Label: "Follow-up review needed", Type: "toggle"},
		}},
	{Name: "Test report", PageType: "Test report", TitlePattern: "Test report {date}",
		Description: "Results of a test run against a procedure.",
		Body: "# {title}\n\n| | |\n|---|---|\n| Date | {{date}} |\n| Tested by | {{tested-by}} |\n| Procedure | {{procedure}} |\n| Item | {{item}} |\n| Serial | {{serial}} |\n\n" +
			"## Setup\n\n{{setup}}\n\n## Results\n\n{{results}}\n\n## Nonconformances\n\n{{ncrs}}\n\n## Conclusion\n\n{{overall}}\n\n{{conclusion}}\n",
		Form: []model.FormField{
			fDate("date", "Date"),
			fMe("tested-by", "Tested by"),
			fRecord("procedure", "Procedure", "procedure", "", true),
			fRecord("item", "Item", "part,assembly", "", true),
			fText("serial", "Serial", "", false),
			fArea("setup", "Setup", "Equipment and conditions.", true),
			fTable("results", "Results", "", true,
				model.FormField{Key: "step", Label: "Step", Type: "index"},
				fText("expected", "Expected", "", true),
				fText("measured", "Measured", "", true),
				fSelect("result", "Pass or fail", "", true, "Pass", "Fail")),
			fTable("ncrs", "Nonconformances", "NCRs raised during the test.", false,
				fRecord("ncr", "NCR", "ncr", "", true),
				fText("notes", "Notes", "", false)),
			fSelect("overall", "Overall result", "", true, "Pass", "Pass with deviations", "Fail"),
			fArea("conclusion", "Conclusion", "", false),
		}},
	{Name: "Meeting notes", PageType: "Meeting notes", TitlePattern: "Meeting notes {date}",
		Description: "Agenda, notes, decisions and actions of a meeting.",
		Body: "# {title}\n\n| | |\n|---|---|\n| Date | {{date}} |\n| Notes by | {{notes-by}} |\n\n## Attendees\n\n{{attendees}}\n\n## Agenda\n\n{{agenda}}\n\n" +
			"## Notes\n\n{{notes}}\n\n## Decisions\n\n{{decisions}}\n\n## Actions\n\n{{actions}}\n",
		Form: []model.FormField{
			fDate("date", "Date"),
			fMe("notes-by", "Notes by"),
			{Key: "attendees", Label: "Attendees", Type: "users", Required: true},
			fList("agenda", "Agenda", "", true),
			fArea("notes", "Notes", "", false),
			fList("decisions", "Decisions", "", false),
			fTable("actions", "Actions", "", false,
				fText("action", "Action", "", true),
				model.FormField{Key: "owner", Label: "Owner", Type: "user", Required: true},
				model.FormField{Key: "due", Label: "Due", Type: "date"}),
		}},
	{Name: "Work instruction", PageType: "General", TitlePattern: "Work instruction: ",
		Description: "Step-by-step instruction for building or installing an item.",
		Body: "# {title}\n\n| | |\n|---|---|\n| Written by | {{written-by}} |\n| Date | {{date}} |\n| Item | {{item}} |\n| Lockout and tagout required | {{loto}} |\n\n" +
			"## Safety\n\n{{safety}}\n\n## Tools and materials\n\n{{tools}}\n\n## Steps\n\n{{steps}}\n\n## Checks\n\n{{checks}}\n",
		Form: []model.FormField{
			fMe("written-by", "Written by"),
			fDate("date", "Date"),
			fRecord("item", "Item", "part,assembly", "", true),
			{Key: "loto", Label: "Lockout and tagout required", Type: "toggle"},
			fArea("safety", "Safety", "Hazards and protective equipment.", true),
			fList("tools", "Tools and materials", "", true),
			fTable("steps", "Steps", "", true,
				model.FormField{Key: "n", Label: "Step", Type: "index"},
				fText("instruction", "Instruction", "", true),
				fText("check", "Acceptance", "", false)),
			fList("checks", "Checks", "Checks after the last step.", false),
		}},
	{Name: "Procedure", PageType: "General", TitlePattern: "Procedure: ",
		Description: "A repeatable process with purpose, steps and records.",
		Body: "# {title}\n\n| | |\n|---|---|\n| Owner | {{owner}} |\n| Date | {{date}} |\n\n## Purpose\n\n{{purpose}}\n\n## Scope\n\n{{scope}}\n\n" +
			"## Responsibilities\n\n{{responsibilities}}\n\n## Steps\n\n{{steps}}\n\n## Records\n\n{{records}}\n",
		Form: []model.FormField{
			fMe("owner", "Owner"),
			fDate("date", "Date"),
			fArea("purpose", "Purpose", "", true),
			fArea("scope", "Scope", "", true),
			fTable("responsibilities", "Responsibilities", "", true,
				fText("role", "Role", "", true),
				fText("responsibility", "Responsibility", "", true)),
			fTable("steps", "Steps", "", true,
				model.FormField{Key: "n", Label: "Step", Type: "index"},
				fText("step", "Step", "", true),
				fText("by", "Responsible", "", false)),
			fList("records", "Records", "Records kept when the procedure is followed.", false),
		}},
	{Name: "Decision record", PageType: "Decision record", TitlePattern: "Decision: ",
		Description: "A decision with its context, options and consequences.",
		Body: "# {title}\n\n| | |\n|---|---|\n| Date | {{date}} |\n| Decided by | {{decided-by}} |\n| Status | {{status}} |\n\n## Context\n\n{{context}}\n\n" +
			"## Options considered\n\n{{options}}\n\n## Decision\n\n{{decision}}\n\n## Consequences\n\n{{consequences}}\n",
		Form: []model.FormField{
			fDate("date", "Date"),
			fMe("decided-by", "Decided by"),
			fSelect("status", "Status", "Proposed", true, "Proposed", "Accepted", "Rejected", "Superseded"),
			fArea("context", "Context", "", true),
			fTable("options", "Options considered", "", true,
				fText("option", "Option", "", true),
				fText("for", "For", "", false),
				fText("against", "Against", "", false)),
			fArea("decision", "Decision", "", true),
			fArea("consequences", "Consequences", "", false),
		}},
}

// formJSON encodes a field list for the form column.
func formJSON(f []model.FormField) []byte {
	if f == nil {
		f = []model.FormField{}
	}
	b, _ := json.Marshal(f)
	return b
}

// ensureTemplates creates the initial company-wide templates on first start.
// On an existing installation it gives the initial templates their forms,
// but only those never changed since they were created.
func (a *App) ensureTemplates(ctx context.Context) error {
	var n int
	if err := a.DB.QueryRow(ctx, `SELECT count(*) FROM templates`).Scan(&n); err != nil {
		return err
	}
	return a.tx(ctx, func(tx pgx.Tx) error {
		for _, t := range initialTemplates {
			if n > 0 {
				var id string
				var version int
				err := tx.QueryRow(ctx, `UPDATE templates SET body = $2, form = $3, changed_at = now(), version = version + 1
					WHERE name = $1 AND space_id = '' AND version = 1 AND changed_by IS NULL AND form = '[]'::jsonb RETURNING id, version`,
					t.Name, t.Body, formJSON(t.Form)).Scan(&id, &version)
				if err == pgx.ErrNoRows {
					continue
				}
				if err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO template_versions (template_id, version, body, form) VALUES ($1, $2, $3, $4)`, id, version, t.Body, formJSON(t.Form)); err != nil {
					return err
				}
				continue
			}
			id, err := a.nextPrefixed(ctx, tx, "TPL", "template", "")
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO templates (id, name, description, page_type, title_pattern, body, form) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
				id, t.Name, t.Description, t.PageType, t.TitlePattern, t.Body, formJSON(t.Form)); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO template_versions (template_id, version, body, form) VALUES ($1, 1, $2, $3)`, id, t.Body, formJSON(t.Form)); err != nil {
				return err
			}
		}
		return nil
	})
}

var placeholder = regexp.MustCompile(`\{?\{(date|user|space|title)\}\}?`)

// fillPlaceholders fills {date}, {user}, {space} and {title}. Form field
// tokens such as {{date}} are left alone.
func fillPlaceholders(s string, u *User, space, title string) string {
	vals := map[string]string{"date": time.Now().Format("2006-01-02"), "user": u.DisplayName, "space": space, "title": title}
	return placeholder.ReplaceAllStringFunc(s, func(m string) string {
		if strings.HasPrefix(m, "{{") || strings.HasSuffix(m, "}}") {
			return m
		}
		return vals[strings.Trim(m, "{}")]
	})
}

// fillLayout fills the placeholders of a form layout except {title}, which
// the page fills from its current title each time the body is generated.
func fillLayout(s string, u *User, space string) string {
	return fillPlaceholders(s, u, space, "{title}")
}

// templateFor returns an active template that may be used in a space.
func (a *App) templateFor(ctx context.Context, id, space string) (*Template, error) {
	t, err := scanTemplate(a.DB.QueryRow(ctx, templateSelect+` WHERE t.id = $1`, id))
	if err != nil || !t.Active || (t.Space != "" && t.Space != space) {
		return nil, badRequest("Unknown template.")
	}
	return t, nil
}

// canManageTemplate: company-wide templates need Manage templates in the
// library role; space templates also allow the space's Admins.
func canManageTemplate(u *User, space string) bool {
	if space == "" {
		return u.Can(PermTemplates)
	}
	return u.Can(PermTemplates) || u.CanIn(space, PermSpaceAdmin)
}

// apiTemplates lists templates: for one space (company-wide and the
// space's), or every template for Admins.
func (a *App) apiTemplates(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	space := r.URL.Query().Get("space")
	where := ` WHERE t.space_id = '' OR t.space_id = $1`
	args := []any{space}
	if space != "" && !u.InSpace(space) {
		return errNotFound
	}
	if space == "" {
		if !u.Can(PermTemplates) {
			return errForbidden
		}
		where, args = "", nil
	}
	rows, err := a.DB.Query(r.Context(), templateSelect+where+` ORDER BY t.space_id <> '', lower(t.name)`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []*Template{}
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return err
		}
		if r.URL.Query().Get("active") == "1" && !t.Active {
			continue
		}
		if space != "" {
			t.Body = "" // the list does not need bodies
		}
		out = append(out, t)
	}
	return ok(w, out)
}

func (a *App) apiTemplate(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	t, err := scanTemplate(a.DB.QueryRow(r.Context(), templateSelect+` WHERE t.id = $1`, r.PathValue("id")))
	if err != nil || (t.Space != "" && !u.InSpace(t.Space)) {
		return errNotFound
	}
	return ok(w, map[string]any{"template": t, "canEdit": canManageTemplate(u, t.Space)})
}

type templateInput struct {
	Name         *string            `json:"name"`
	Space        *string            `json:"space"`
	Description  *string            `json:"description"`
	PageType     *string            `json:"pageType"`
	TitlePattern *string            `json:"titlePattern"`
	Body         *string            `json:"body"`
	Form         *[]model.FormField `json:"form"`
	Active       *bool              `json:"active"`
	Version      int                `json:"version"`
}

func (a *App) applyTemplateInput(ctx context.Context, t *Template, in *templateInput) error {
	if in.Name != nil {
		t.Name = strings.TrimSpace(*in.Name)
	}
	if t.Name == "" || len([]rune(t.Name)) > 60 {
		return badRequest("Name is required (at most 60 characters).")
	}
	if in.Description != nil {
		t.Description = strings.TrimSpace(*in.Description)
	}
	if in.PageType != nil {
		t.PageType = *in.PageType
	}
	var n int
	a.DB.QueryRow(ctx, `SELECT count(*) FROM admin_lists WHERE list = 'page_types' AND value = $1 AND active`, t.PageType).Scan(&n)
	if n == 0 {
		return badRequest("Choose a page type.")
	}
	if in.TitlePattern != nil {
		t.TitlePattern = strings.TrimSpace(*in.TitlePattern)
	}
	if in.Body != nil {
		t.Body = *in.Body
	}
	if strings.TrimSpace(t.Body) == "" {
		return badRequest("The template body is required.")
	}
	if in.Form != nil {
		t.Form = *in.Form
	}
	if t.Form == nil {
		t.Form = []model.FormField{}
	}
	for i := range t.Form {
		t.Form[i].Label = strings.TrimSpace(t.Form[i].Label)
	}
	if len(t.Form) > 0 {
		if err := model.ValidateFields(t.Form, t.Body); err != nil {
			return badRequest("%s.", err.Error())
		}
	}
	if in.Active != nil {
		t.Active = *in.Active
	}
	return nil
}

func (a *App) apiCreateTemplate(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	var in templateInput
	if err := decode(r, &in); err != nil {
		return err
	}
	t := &Template{Active: true, Space: deref(in.Space)}
	if !canManageTemplate(u, t.Space) {
		return errForbidden
	}
	if t.Space != "" {
		var exists bool
		a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM spaces WHERE id = $1)`, t.Space).Scan(&exists)
		if !exists {
			return badRequest("Unknown space.")
		}
	}
	if err := a.applyTemplateInput(ctx, t, &in); err != nil {
		return err
	}
	err := a.tx(ctx, func(tx pgx.Tx) error {
		id, err := a.nextPrefixed(ctx, tx, "TPL", "template", u.ID)
		if err != nil {
			return err
		}
		t.ID = id
		if _, err := tx.Exec(ctx, `INSERT INTO templates (id, name, space_id, description, page_type, title_pattern, body, form, active, changed_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, id, t.Name, t.Space, t.Description, t.PageType, t.TitlePattern, t.Body, formJSON(t.Form), t.Active, u.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO template_versions (template_id, version, body, form, changed_by) VALUES ($1, 1, $2, $3, $4)`, id, t.Body, formJSON(t.Form), u.ID); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, clientIP(r), "template.create", id, map[string]string{"name": t.Name, "space": t.Space})
		return nil
	})
	if err != nil {
		return err
	}
	return ok(w, map[string]string{"id": t.ID})
}

func (a *App) apiUpdateTemplate(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	t, err := scanTemplate(a.DB.QueryRow(ctx, templateSelect+` WHERE t.id = $1`, r.PathValue("id")))
	if err != nil {
		return errNotFound
	}
	if !canManageTemplate(u, t.Space) {
		return errForbidden
	}
	var in templateInput
	if err := decode(r, &in); err != nil {
		return err
	}
	if in.Version != 0 && in.Version != t.Version {
		return conflict("Someone changed this template after you opened it. Reload to see their changes.", nil)
	}
	oldBody, oldForm := t.Body, string(formJSON(t.Form))
	if err := a.applyTemplateInput(ctx, t, &in); err != nil {
		return err
	}
	err = a.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE templates SET name = $2, description = $3, page_type = $4, title_pattern = $5, body = $6, form = $9, active = $7, changed_by = $8,
			changed_at = now(), version = version + 1 WHERE id = $1`, t.ID, t.Name, t.Description, t.PageType, t.TitlePattern, t.Body, t.Active, u.ID, formJSON(t.Form)); err != nil {
			return err
		}
		if t.Body != oldBody || string(formJSON(t.Form)) != oldForm {
			if _, err := tx.Exec(ctx, `INSERT INTO template_versions (template_id, version, body, form, changed_by) VALUES ($1, $2, $3, $4, $5)`, t.ID, t.Version+1, t.Body, formJSON(t.Form), u.ID); err != nil {
				return err
			}
		}
		a.auditQ(ctx, tx, u, clientIP(r), "template.update", t.ID, map[string]any{"name": t.Name, "active": t.Active})
		return nil
	})
	if err != nil {
		return err
	}
	return ok(w, map[string]int{"version": t.Version + 1})
}
