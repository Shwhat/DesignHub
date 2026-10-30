package app

import (
	"context"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Admin lists ----------------------------------------------------------------

// Lists that Admins can edit, with their display names.
var adminLists = []struct{ Key, Name string }{
	{"units", "Units of measure"},
	{"commodity_codes", "Commodity codes"},
	{"requirement_categories", "Requirement categories"},
	{"procedure_types", "Procedure types"},
	{"document_types", "Document types"},
	{"page_types", "Page types"},
	{"issue_areas", "Issue areas"},
	{"file_roles", "File roles"},
}

type listValue struct {
	Value      string   `json:"value"`
	SortOrder  int      `json:"sortOrder"`
	Active     bool     `json:"active"`
	ExpectedOn []string `json:"expectedOn,omitempty"`
	InUse      bool     `json:"inUse,omitempty"`
}

// ListValues returns a list's values in order.
func (a *App) ListValues(ctx context.Context, list string, activeOnly bool) ([]listValue, error) {
	rows, err := a.DB.Query(ctx, `SELECT value, sort_order, active, expected_on FROM admin_lists WHERE list = $1 AND (active OR NOT $2) ORDER BY sort_order, value`, list, activeOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []listValue{}
	for rows.Next() {
		var v listValue
		if err := rows.Scan(&v.Value, &v.SortOrder, &v.Active, &v.ExpectedOn); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (a *App) apiLists(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	type list struct {
		Key    string      `json:"key"`
		Name   string      `json:"name"`
		Values []listValue `json:"values"`
	}
	out := []list{}
	for _, l := range adminLists {
		vs, err := a.ListValues(ctx, l.Key, false)
		if err != nil {
			return err
		}
		out = append(out, list{l.Key, l.Name, vs})
	}
	return ok(w, out)
}

// apiPutList replaces a list's values. Values that exist in the database but
// are missing from the request are deactivated, never deleted.
func (a *App) apiPutList(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermLists) {
		return errForbidden
	}
	key := r.PathValue("list")
	found := false
	for _, l := range adminLists {
		found = found || l.Key == key
	}
	if !found {
		return errNotFound
	}
	var in []listValue
	if err := decode(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	seen := map[string]bool{}
	err := a.tx(ctx, func(tx pgx.Tx) error {
		for i, v := range in {
			v.Value = strings.TrimSpace(v.Value)
			if v.Value == "" || len(v.Value) > 80 {
				return badRequest("List values must be 1 to 80 characters.")
			}
			if seen[strings.ToLower(v.Value)] {
				return badRequest("%q appears twice.", v.Value)
			}
			seen[strings.ToLower(v.Value)] = true
			if v.ExpectedOn == nil {
				v.ExpectedOn = []string{}
			}
			if _, err := tx.Exec(ctx, `INSERT INTO admin_lists (list, value, sort_order, active, expected_on) VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT (list, value) DO UPDATE SET sort_order = $3, active = $4, expected_on = $5`, key, v.Value, i+1, v.Active, v.ExpectedOn); err != nil {
				return err
			}
		}
		rows, err := tx.Query(ctx, `SELECT value FROM admin_lists WHERE list = $1`, key)
		if err != nil {
			return err
		}
		var missing []string
		for rows.Next() {
			var v string
			if err := rows.Scan(&v); err != nil {
				rows.Close()
				return err
			}
			if !seen[strings.ToLower(v)] {
				missing = append(missing, v)
			}
		}
		rows.Close()
		for _, v := range missing {
			if _, err := tx.Exec(ctx, `UPDATE admin_lists SET active = false WHERE list = $1 AND value = $2`, key, v); err != nil {
				return err
			}
		}
		a.auditQ(ctx, tx, u, clientIP(r), "list.update", "", map[string]any{"list": key, "values": in})
		return nil
	})
	if err != nil {
		return err
	}
	vs, err := a.ListValues(ctx, key, false)
	if err != nil {
		return err
	}
	return ok(w, vs)
}

// Manufacturers --------------------------------------------------------------

type manufacturer struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Website string `json:"website"`
	Active  bool   `json:"active"`
	Used    bool   `json:"used"`
}

var mfrCodeRe = regexp.MustCompile(`^[A-Z]{2,5}$`)

func (a *App) apiManufacturers(w http.ResponseWriter, r *http.Request) error {
	rows, err := a.DB.Query(r.Context(), `SELECT code, name, website, active, used FROM manufacturers ORDER BY code`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []manufacturer{}
	for rows.Next() {
		var m manufacturer
		if err := rows.Scan(&m.Code, &m.Name, &m.Website, &m.Active, &m.Used); err != nil {
			return err
		}
		out = append(out, m)
	}
	return ok(w, out)
}

func (a *App) apiCreateManufacturer(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermLists) {
		return errForbidden
	}
	var m manufacturer
	if err := decode(r, &m); err != nil {
		return err
	}
	m.Code = strings.ToUpper(strings.TrimSpace(m.Code))
	m.Name = strings.TrimSpace(m.Name)
	if !mfrCodeRe.MatchString(m.Code) {
		return badRequest("Manufacturer code must be 2 to 5 capital letters.")
	}
	if m.Name == "" {
		return badRequest("Manufacturer name is required.")
	}
	if _, err := a.DB.Exec(r.Context(), `INSERT INTO manufacturers (code, name, website) VALUES ($1, $2, $3)`, m.Code, m.Name, strings.TrimSpace(m.Website)); err != nil {
		return err
	}
	a.Audit(r.Context(), u, clientIP(r), "manufacturer.create", "", m)
	return ok(w, nil)
}

func (a *App) apiUpdateManufacturer(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermLists) {
		return errForbidden
	}
	code := r.PathValue("code")
	var in struct {
		Code    *string `json:"code"`
		Name    *string `json:"name"`
		Website *string `json:"website"`
		Active  *bool   `json:"active"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	var used bool
	if err := a.DB.QueryRow(ctx, `SELECT used FROM manufacturers WHERE code = $1`, code).Scan(&used); err != nil {
		return err
	}
	if in.Code != nil && *in.Code != code {
		if used {
			return badRequest("Code %s is used in part numbers and cannot change.", code)
		}
		nc := strings.ToUpper(strings.TrimSpace(*in.Code))
		if !mfrCodeRe.MatchString(nc) {
			return badRequest("Manufacturer code must be 2 to 5 capital letters.")
		}
		if _, err := a.DB.Exec(ctx, `UPDATE manufacturers SET code = $2 WHERE code = $1`, code, nc); err != nil {
			return err
		}
		code = nc
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		if _, err := a.DB.Exec(ctx, `UPDATE manufacturers SET name = $2 WHERE code = $1`, code, strings.TrimSpace(*in.Name)); err != nil {
			return err
		}
	}
	if in.Website != nil {
		if _, err := a.DB.Exec(ctx, `UPDATE manufacturers SET website = $2 WHERE code = $1`, code, strings.TrimSpace(*in.Website)); err != nil {
			return err
		}
	}
	if in.Active != nil {
		if _, err := a.DB.Exec(ctx, `UPDATE manufacturers SET active = $2 WHERE code = $1`, code, *in.Active); err != nil {
			return err
		}
	}
	a.Audit(ctx, u, clientIP(r), "manufacturer.update", "", map[string]any{"code": code, "changes": in})
	return ok(w, nil)
}

// System status (Super Admin) ------------------------------------------------

func (a *App) apiSystemStatus(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermSystem) {
		return errForbidden
	}
	ctx := r.Context()
	st := map[string]any{"version": a.Cfg.Version}
	var pgVersion string
	var dbSize int64
	if err := a.DB.QueryRow(ctx, `SELECT version(), pg_database_size(current_database())`).Scan(&pgVersion, &dbSize); err != nil {
		st["database"] = map[string]any{"connected": false, "error": err.Error()}
	} else {
		var migs []string
		rows, err := a.DB.Query(ctx, `SELECT lpad(version::text, 4, '0') || '_' || name FROM schema_migrations ORDER BY version`)
		if err == nil {
			for rows.Next() {
				var m string
				if rows.Scan(&m) == nil {
					migs = append(migs, m)
				}
			}
			rows.Close()
		}
		st["database"] = map[string]any{"connected": true, "server": pgVersion, "sizeBytes": dbSize, "migrations": migs}
	}
	head, _ := a.Repo.Head("main")
	tags, _ := a.Repo.Tags("baseline/")
	st["repository"] = map[string]any{"head": head, "baselines": len(tags)}
	counts := map[string]int{}
	rows, err := a.DB.Query(ctx, `SELECT kind, count(*) FROM records GROUP BY kind`)
	if err == nil {
		for rows.Next() {
			var k string
			var n int
			if rows.Scan(&k, &n) == nil {
				counts[k] = n
			}
		}
		rows.Close()
	}
	st["records"] = counts
	backups, _ := a.listBackups()
	sort.Slice(backups, func(i, j int) bool { return backups[i].Name > backups[j].Name })
	st["backups"] = backups
	return ok(w, st)
}

func (a *App) apiReindex(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermUsers) && !u.Can(PermSystem) {
		return errForbidden
	}
	if err := a.RebuildIndex(r.Context()); err != nil {
		return err
	}
	a.Audit(r.Context(), u, clientIP(r), "index.rebuild", "", nil)
	return ok(w, nil)
}
