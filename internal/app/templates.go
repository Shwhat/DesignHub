package app

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Page templates (DATA-MODEL.md 7.25). A template is company-wide (empty
// space) or belongs to one space. Pages created from a template keep a
// reference to it; later template changes do not change existing pages.

type Template struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Space        string    `json:"space"`
	Description  string    `json:"description"`
	PageType     string    `json:"pageType"`
	TitlePattern string    `json:"titlePattern"`
	Body         string    `json:"body"`
	Active       bool      `json:"active"`
	ChangedBy    string    `json:"changedBy"`
	ChangedAt    time.Time `json:"changedAt"`
	Version      int       `json:"version"`
}

const templateSelect = `SELECT t.id, t.name, t.space_id, t.description, t.page_type, t.title_pattern, t.body, t.active,
	COALESCE(us.display_name, 'DesignHub'), t.changed_at, t.version FROM templates t LEFT JOIN users us ON us.id = t.changed_by`

func scanTemplate(row pgx.Row) (*Template, error) {
	t := &Template{}
	err := row.Scan(&t.ID, &t.Name, &t.Space, &t.Description, &t.PageType, &t.TitlePattern, &t.Body, &t.Active, &t.ChangedBy, &t.ChangedAt, &t.Version)
	return t, err
}

var initialTemplates = []Template{
	{Name: "Design review", PageType: "Design review", TitlePattern: "Design review {date}",
		Description: "Review of a design against its requirements, with findings and actions.",
		Body:        "# {title}\n\n| | |\n|---|---|\n| Date | {date} |\n| Facilitator | {user} |\n| Space | {space} |\n\n## Scope\n\nAssembly or item under review.\n\n## Attendees\n\n- \n\n## Requirements checked\n\n| Requirement | Result | Notes |\n|---|---|---|\n| | | |\n\n## Findings\n\n| # | Finding | Severity | Action |\n|---|---|---|---|\n| 1 | | | |\n\n## Decision\n\nApproved, approved with actions, or not approved.\n"},
	{Name: "Test report", PageType: "Test report", TitlePattern: "Test report {date}",
		Description: "Results of a test run against a procedure.",
		Body:        "# {title}\n\n| | |\n|---|---|\n| Date | {date} |\n| Tested by | {user} |\n| Procedure | |\n| Item and serial | |\n\n## Setup\n\nEquipment and conditions.\n\n## Results\n\n| Step | Expected | Measured | Pass or fail |\n|---|---|---|---|\n| | | | |\n\n## Nonconformances\n\nList any NCRs raised.\n\n## Conclusion\n\n"},
	{Name: "Meeting notes", PageType: "Meeting notes", TitlePattern: "Meeting notes {date}",
		Description: "Agenda, notes, decisions and actions of a meeting.",
		Body:        "# {title}\n\n| | |\n|---|---|\n| Date | {date} |\n| Notes by | {user} |\n\n## Attendees\n\n- \n\n## Agenda\n\n1. \n\n## Notes\n\n## Decisions\n\n- \n\n## Actions\n\n| Action | Owner | Due |\n|---|---|---|\n| | | |\n"},
	{Name: "Work instruction", PageType: "General", TitlePattern: "Work instruction: ",
		Description: "Step-by-step instruction for building or installing an item.",
		Body:        "# {title}\n\n| | |\n|---|---|\n| Written by | {user} |\n| Date | {date} |\n| Item | |\n\n## Safety\n\n## Tools and materials\n\n- \n\n## Steps\n\n1. \n\n## Checks\n\n- \n"},
	{Name: "Procedure", PageType: "General", TitlePattern: "Procedure: ",
		Description: "A repeatable process with purpose, steps and records.",
		Body:        "# {title}\n\n| | |\n|---|---|\n| Owner | {user} |\n| Date | {date} |\n\n## Purpose\n\n## Scope\n\n## Responsibilities\n\n## Steps\n\n1. \n\n## Records\n\n"},
	{Name: "Decision record", PageType: "Decision record", TitlePattern: "Decision: ",
		Description: "A decision with its context, options and consequences.",
		Body:        "# {title}\n\n| | |\n|---|---|\n| Date | {date} |\n| Decided by | {user} |\n| Status | Proposed |\n\n## Context\n\n## Options considered\n\n| Option | For | Against |\n|---|---|---|\n| | | |\n\n## Decision\n\n## Consequences\n\n"},
}

// ensureTemplates creates the initial company-wide templates on first start.
func (a *App) ensureTemplates(ctx context.Context) error {
	var n int
	if err := a.DB.QueryRow(ctx, `SELECT count(*) FROM templates`).Scan(&n); err != nil || n > 0 {
		return err
	}
	return a.tx(ctx, func(tx pgx.Tx) error {
		for _, t := range initialTemplates {
			id, err := a.nextPrefixed(ctx, tx, "TPL", "template", "")
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO templates (id, name, description, page_type, title_pattern, body) VALUES ($1, $2, $3, $4, $5, $6)`,
				id, t.Name, t.Description, t.PageType, t.TitlePattern, t.Body); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO template_versions (template_id, version, body) VALUES ($1, 1, $2)`, id, t.Body); err != nil {
				return err
			}
		}
		return nil
	})
}

func fillPlaceholders(s string, u *User, space, title string) string {
	return strings.NewReplacer("{date}", time.Now().Format("2006-01-02"), "{user}", u.DisplayName, "{space}", space, "{title}", title).Replace(s)
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
	Name         *string `json:"name"`
	Space        *string `json:"space"`
	Description  *string `json:"description"`
	PageType     *string `json:"pageType"`
	TitlePattern *string `json:"titlePattern"`
	Body         *string `json:"body"`
	Active       *bool   `json:"active"`
	Version      int     `json:"version"`
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
		if _, err := tx.Exec(ctx, `INSERT INTO templates (id, name, space_id, description, page_type, title_pattern, body, active, changed_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, id, t.Name, t.Space, t.Description, t.PageType, t.TitlePattern, t.Body, t.Active, u.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO template_versions (template_id, version, body, changed_by) VALUES ($1, 1, $2, $3)`, id, t.Body, u.ID); err != nil {
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
	oldBody := t.Body
	if err := a.applyTemplateInput(ctx, t, &in); err != nil {
		return err
	}
	err = a.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE templates SET name = $2, description = $3, page_type = $4, title_pattern = $5, body = $6, active = $7, changed_by = $8,
			changed_at = now(), version = version + 1 WHERE id = $1`, t.ID, t.Name, t.Description, t.PageType, t.TitlePattern, t.Body, t.Active, u.ID); err != nil {
			return err
		}
		if t.Body != oldBody {
			if _, err := tx.Exec(ctx, `INSERT INTO template_versions (template_id, version, body, changed_by) VALUES ($1, $2, $3, $4)`, t.ID, t.Version+1, t.Body, u.ID); err != nil {
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
