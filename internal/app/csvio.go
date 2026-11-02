package app

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"designhub/internal/model"
)

// CSV export and import (DATA-MODEL.md 7.26, SCREENS.md 24).
//
// Table exports are built in the browser from the rows on screen; the
// browser reports each one here for the audit log. Full exports with every
// field are built here. Imports are previewed first and save only when no
// row has an error.

// writeCSV sends a CSV file. A byte order mark lets spreadsheet programs
// read UTF-8 correctly.
func writeCSV(w http.ResponseWriter, name string, header []string, rows [][]string) error {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte("\xEF\xBB\xBF"))
	cw := csv.NewWriter(w)
	cw.Write(header)
	for _, r := range rows {
		for i, v := range r {
			r[i] = csvSafe(v)
		}
		cw.Write(r)
	}
	cw.Flush()
	return cw.Error()
}

// csvSafe stops a spreadsheet from treating a value as a formula.
func csvSafe(v string) string {
	if v != "" && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return "'" + v
		}
	}
	return v
}

// apiLogExport records a table export made in the browser.
func (a *App) apiLogExport(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	var in struct {
		View string `json:"view"`
		Rows int    `json:"rows"`
		Kind string `json:"kind"` // csv or pdf
		ID   string `json:"id"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	if in.Kind == "" {
		in.Kind = "csv"
	}
	a.Audit(r.Context(), u, clientIP(r), "export."+in.Kind, in.ID, map[string]any{"view": truncate(in.View, 100), "rows": in.Rows})
	return ok(w, nil)
}

func joinOr(list []string) string { return strings.Join(list, "; ") }

func fnum(f float64) string {
	if f == 0 {
		return "0"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// Column sets shared by export and import, so an exported file imports back.
var csvColumns = map[string][]string{
	"parts": {"ID", "Kind", "Name", "Description", "Revision", "Status", "Unit", "Tracking", "Commodity code", "Home space", "Owner",
		"Manufacturer", "Order number", "Type number", "Lifecycle", "Unit cost", "Lead time days", "Labels", "Suppliers"},
	"bom":          {"Assembly", "Find", "Item", "Quantity", "Reference designators", "Notes"},
	"requirements": {"ID", "Title", "Statement", "Rationale", "Category", "Source", "Priority", "Revision", "Status", "Home space", "Owner", "Labels"},
	"procedures":   {"ID", "Title", "Procedure type", "Purpose", "Revision", "Status", "Home space", "Owner", "Labels"},
	"documents":    {"ID", "Title", "Document type", "Summary", "External reference", "Revision", "Status", "Home space", "Owner", "Labels"},
	"work": {"ID", "Type", "Title", "Description", "Status", "Priority", "Assignee", "Reporter", "Due date", "Epic", "Labels", "Estimate", "Logged",
		"Sprint", "Milestone", "Space", "Created", "Updated"},
	"change-orders": {"ID", "Title", "Change type", "State", "Priority", "Owner", "Need date", "Home space", "Affected items", "Released on", "Baseline"},
	"companies":     {"ID", "Name", "Kind", "Supplier status", "Manufacturer codes", "Address", "Phone", "Website", "Notes", "Active"},
	"manufacturers": {"Code", "Name", "Website", "Active"},
	"time":          {"Date", "Person", "Item", "Title", "Hours", "Note", "Space"},
	"users":         {"Username", "Display name", "Email", "Library role", "Company", "Account expiry", "Active", "Last sign-in"},
	"audit":         {"Time", "User", "Action", "Record", "Detail", "Address"},
}

// apiFullExport writes every field of one record type.
func (a *App) apiFullExport(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	typ := r.PathValue("type")
	header, okT := csvColumns[typ]
	if !okT {
		return errNotFound
	}
	q := r.URL.Query()
	var rows [][]string
	var err error
	switch typ {
	case "parts", "requirements", "procedures", "documents", "bom":
		rows, err = a.exportLibrary(ctx, u, typ)
	case "work":
		rows, err = a.exportWork(ctx, u, q.Get("space"), q.Get("type"))
	case "change-orders":
		rows, err = a.exportCOs(ctx, u)
	case "companies":
		rows, err = a.exportCompanies(ctx, u)
	case "manufacturers":
		rows, err = a.exportManufacturers(ctx, u)
	case "time":
		rows, err = a.exportTime(ctx, u, q.Get("space"), q.Get("from"), q.Get("to"))
	case "users":
		if !u.Can(PermUsers) {
			return errForbidden
		}
		rows, err = a.exportUsers(ctx)
	case "audit":
		if !u.Can(PermAudit) {
			return errForbidden
		}
		rows, err = a.exportAudit(ctx)
	}
	if err != nil {
		return err
	}
	a.Audit(ctx, u, clientIP(r), "export.csv", "", map[string]any{"view": "full " + typ, "rows": len(rows)})
	return writeCSV(w, fmt.Sprintf("designhub-%s-%s.csv", typ, time.Now().Format("2006-01-02")), header, rows)
}

func (a *App) exportLibrary(ctx context.Context, u *User, typ string) ([][]string, error) {
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return nil, err
	}
	status := map[string]string{}
	rs, err := a.DB.Query(ctx, `SELECT id, status FROM records WHERE origin = 'git'`)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var id, s string
		if rs.Scan(&id, &s) == nil {
			status[id] = s
		}
	}
	rs.Close()
	suppliers := map[string][]string{}
	rs, err = a.DB.Query(ctx, `SELECT l.src, c.name FROM links l JOIN companies c ON c.id = l.dst WHERE l.type = 'supplied by' ORDER BY c.name`)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var id, n string
		if rs.Scan(&id, &n) == nil {
			suppliers[id] = append(suppliers[id], n)
		}
	}
	rs.Close()
	modelType := map[string]string{"parts": model.TypePart, "bom": model.TypePart, "requirements": model.TypeRequirement,
		"procedures": model.TypeProcedure, "documents": model.TypeDocument}[typ]
	var out [][]string
	for _, it := range cache.all(modelType) {
		if !vis(it.ID) {
			continue
		}
		if u.IsVendor() {
			it = vendorItem(it)
		}
		st := status[it.ID]
		if st == "" {
			st = it.Status
		}
		switch typ {
		case "parts":
			out = append(out, []string{it.ID, it.Kind, it.Title, it.Description, it.Revision, st, it.Unit, it.Tracking, it.Commodity, it.Space, it.Owner,
				it.Manufacturer, it.OrderNumber, it.TypeNumber, it.Lifecycle, it.UnitCost, itoa0(it.LeadTimeDays), joinOr(it.Labels), joinOr(suppliers[it.ID])})
		case "bom":
			for _, l := range it.BOM {
				if !vis(l.Item) {
					continue
				}
				out = append(out, []string{it.ID, strconv.Itoa(l.Find), l.Item, fnum(l.Qty), strings.Join(l.RefDes, " "), l.Notes})
			}
		case "requirements":
			out = append(out, []string{it.ID, it.Title, it.Statement, it.Rationale, it.Category, it.Source, it.Priority, it.Revision, st, it.Space, it.Owner, joinOr(it.Labels)})
		case "procedures":
			out = append(out, []string{it.ID, it.Title, it.ProcedureType, it.Purpose, it.Revision, st, it.Space, it.Owner, joinOr(it.Labels)})
		case "documents":
			out = append(out, []string{it.ID, it.Title, it.DocumentType, it.Summary, it.ExternalRef, it.Revision, st, it.Space, it.Owner, joinOr(it.Labels)})
		}
	}
	return out, nil
}

func itoa0(n int) string {
	if n == 0 {
		return ""
	}
	return strconv.Itoa(n)
}

func (a *App) exportWork(ctx context.Context, u *User, space, types string) ([][]string, error) {
	where, args := `true`, []any{}
	if space != "" {
		args = append(args, space)
		where += fmt.Sprintf(` AND w.space_id = $%d`, len(args))
	}
	if types != "" {
		args = append(args, strings.Split(types, ","))
		where += fmt.Sprintf(` AND w.type = ANY($%d)`, len(args))
	}
	rows, err := a.DB.Query(ctx, workSelect+` WHERE `+where+` ORDER BY w.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	locks, err := a.locks(ctx)
	if err != nil {
		return nil, err
	}
	names := a.namesOf(ctx)
	var out [][]string
	for rows.Next() {
		wi, err := scanWork(rows)
		if err != nil {
			return nil, err
		}
		if !visible(u, wi.ID, wi.Type, wi.Space, locks[wi.ID]) {
			continue
		}
		est := ""
		if wi.Estimate != nil {
			est = fnum(*wi.Estimate)
		}
		due := ""
		if wi.DueDate != nil {
			due = *wi.DueDate
		}
		out = append(out, []string{wi.ID, wi.Type, wi.Title, wi.Description, wi.Status, wi.Priority, wi.Assignee, wi.Reporter, due, wi.Epic,
			joinOr(wi.Labels), est, fnum(wi.Logged), names[wi.Sprint], names[wi.Milestone], wi.Space,
			wi.CreatedAt.Format(time.RFC3339), wi.UpdatedAt.Format(time.RFC3339)})
	}
	return out, rows.Err()
}

// namesOf maps sprint and milestone ids to names.
func (a *App) namesOf(ctx context.Context) map[string]string {
	m := map[string]string{}
	rows, err := a.DB.Query(ctx, `SELECT id, name FROM sprints UNION ALL SELECT id, name FROM milestones`)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		var id, n string
		if rows.Scan(&id, &n) == nil {
			m[id] = n
		}
	}
	return m
}

func (a *App) exportCOs(ctx context.Context, u *User) ([][]string, error) {
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return nil, err
	}
	rows, err := a.DB.Query(ctx, `SELECT c.id, c.title, c.change_type, c.state, c.priority, us.display_name, COALESCE(c.need_date::text, ''), c.home_space_id,
		COALESCE((SELECT string_agg(i.item_id || ' ' || i.action, '; ' ORDER BY i.item_id) FROM co_items i WHERE i.co_id = c.id), ''),
		COALESCE(to_char(c.released_at, 'YYYY-MM-DD"T"HH24:MI:SSOF'), ''), c.baseline
		FROM change_orders c JOIN users us ON us.id = c.owner_id ORDER BY c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		r := make([]string, 11)
		p := make([]any, 11)
		for i := range r {
			p[i] = &r[i]
		}
		if err := rows.Scan(p...); err != nil {
			return nil, err
		}
		if vis(r[0]) {
			out = append(out, r)
		}
	}
	return out, rows.Err()
}

func (a *App) exportCompanies(ctx context.Context, u *User) ([][]string, error) {
	if !visible(u, "", "company", "", nil) {
		return nil, errForbidden
	}
	rows, err := a.DB.Query(ctx, companySelect+` ORDER BY lower(c.name)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		c, err := scanCompany(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, []string{c.ID, c.Name, joinOr(c.Kinds), c.SupplierStatus, joinOr(c.ManufacturerCodes), c.Address, c.Phone, c.Website, c.Notes, yesNo(c.Active)})
	}
	return out, rows.Err()
}

func yesNo(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func (a *App) exportManufacturers(ctx context.Context, u *User) ([][]string, error) {
	if !u.Can(PermLibView) || u.IsVendor() {
		return nil, errForbidden
	}
	rows, err := a.DB.Query(ctx, `SELECT code, name, website, active FROM manufacturers ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		var c, n, ws string
		var act bool
		if err := rows.Scan(&c, &n, &ws, &act); err != nil {
			return nil, err
		}
		out = append(out, []string{c, n, ws, yesNo(act)})
	}
	return out, rows.Err()
}

func (a *App) exportTime(ctx context.Context, u *User, space, from, to string) ([][]string, error) {
	if !u.CanIn(space, PermTimeReport) {
		return nil, errForbidden
	}
	if err := checkDatesInclusive(from, to); err != nil {
		return nil, err
	}
	rows, err := a.DB.Query(ctx, `SELECT t.day::text, us.display_name, w.id, w.title, t.hours::float8, t.note, w.space_id, w.type
		FROM time_entries t JOIN work_items w ON w.id = t.item_id JOIN users us ON us.id = t.user_id
		WHERE w.space_id = $1 AND t.day BETWEEN $2 AND $3 ORDER BY t.day, us.display_name`, space, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	locks, _ := a.locks(ctx)
	var out [][]string
	for rows.Next() {
		var day, person, id, title, note, sp, typ string
		var h float64
		if err := rows.Scan(&day, &person, &id, &title, &h, &note, &sp, &typ); err != nil {
			return nil, err
		}
		if visible(u, id, typ, sp, locks[id]) {
			out = append(out, []string{day, person, id, title, fnum(h), note, sp})
		}
	}
	return out, rows.Err()
}

func (a *App) exportUsers(ctx context.Context) ([][]string, error) {
	rows, err := a.DB.Query(ctx, `SELECT us.username, us.display_name, us.email, ro.name, COALESCE(c.name, ''), COALESCE(us.expires_on::text, ''),
		us.active, COALESCE(to_char(us.last_sign_in, 'YYYY-MM-DD"T"HH24:MI:SSOF'), '')
		FROM users us JOIN roles ro ON ro.id = us.library_role_id LEFT JOIN companies c ON c.id = us.company_id ORDER BY lower(us.username)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		var x [7]string
		var act bool
		if err := rows.Scan(&x[0], &x[1], &x[2], &x[3], &x[4], &x[5], &act, &x[6]); err != nil {
			return nil, err
		}
		out = append(out, []string{x[0], x[1], x[2], x[3], x[4], x[5], yesNo(act), x[6]})
	}
	return out, rows.Err()
}

func (a *App) exportAudit(ctx context.Context) ([][]string, error) {
	rows, err := a.DB.Query(ctx, `SELECT to_char(at, 'YYYY-MM-DD"T"HH24:MI:SSOF'), username, action, record_id, detail::text, ip FROM audit_log ORDER BY at DESC LIMIT 100000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]string
	for rows.Next() {
		r := make([]string, 6)
		if err := rows.Scan(&r[0], &r[1], &r[2], &r[3], &r[4], &r[5]); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Import

var importTypes = map[string]string{"parts": "parts", "bom": "bom", "requirements": "requirements", "work": "work",
	"manufacturers": "manufacturers", "companies": "companies"}

// Columns a file must have, by type. Other export columns are ignored.
var importRequired = map[string][]string{
	"parts":         {"Kind", "Name"},
	"bom":           {"Item", "Quantity"},
	"requirements":  {"Title", "Statement"},
	"work":          {"Type", "Title"},
	"manufacturers": {"Code", "Name"},
	"companies":     {"Name", "Kind"},
}

type importInput struct {
	Type   string `json:"type"`
	Mode   string `json:"mode"`   // create, update (create and update), add, replace
	Target string `json:"target"` // space for work items and new library items; draft assembly for BOM lines
	SHA256 string `json:"sha256"`
	Name   string `json:"name"`
}

type importRow struct {
	Row     int               `json:"row"`
	Outcome string            `json:"outcome"` // Create, Update, Error
	ID      string            `json:"id"`
	Error   string            `json:"error,omitempty"`
	Values  map[string]string `json:"values"`
}

// readImportFile parses an uploaded CSV file into header-keyed rows.
func (a *App) readImportFile(sum string) ([]map[string]string, []string, error) {
	f, err := a.Files.Open(sum)
	if err != nil {
		return nil, nil, badRequest("The file was not uploaded.")
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 20<<20))
	if err != nil {
		return nil, nil, err
	}
	b = bytes.TrimPrefix(b, []byte("\xEF\xBB\xBF"))
	if !utf8.Valid(b) {
		return nil, nil, badRequest("Save the file as CSV UTF-8 and try again.")
	}
	cr := csv.NewReader(bytes.NewReader(b))
	cr.FieldsPerRecord = -1
	cr.TrimLeadingSpace = true
	recs, err := cr.ReadAll()
	if err != nil {
		return nil, nil, badRequest("The file is not valid CSV: %s.", err.Error())
	}
	if len(recs) < 2 {
		return nil, nil, badRequest("The file has no rows below the header.")
	}
	if len(recs) > 5001 {
		return nil, nil, badRequest("Import at most 5000 rows at a time.")
	}
	header := make([]string, len(recs[0]))
	for i, h := range recs[0] {
		header[i] = strings.TrimSpace(h)
	}
	var out []map[string]string
	for _, rec := range recs[1:] {
		m := map[string]string{}
		empty := true
		for i, v := range rec {
			if i < len(header) && header[i] != "" {
				v = strings.TrimSpace(strings.TrimPrefix(v, "'"))
				m[header[i]] = v
				if v != "" {
					empty = false
				}
			}
		}
		if !empty {
			out = append(out, m)
		}
	}
	return out, header, nil
}

func (a *App) canImport(ctx context.Context, u *User, in *importInput) error {
	if _, okT := importTypes[in.Type]; !okT {
		return badRequest("Unknown import type.")
	}
	if !u.Can(PermImport) || u.IsVendor() {
		return errForbidden
	}
	switch in.Type {
	case "work":
		if !u.CanIn(in.Target, PermWorkEdit) {
			return badRequest("Choose a space you can add work items to.")
		}
	case "bom":
		e := cache.get(in.Target)
		if e == nil || e.Item.Type != model.TypePart || !a.canSee(ctx, u, in.Target) {
			return badRequest("Choose a draft assembly.")
		}
		if co := a.openCOFor(ctx, a.DB, in.Target); co != nil || e.Item.Status != model.StatusDraft {
			return badRequest("BOM lines can be imported only into an assembly that has never been released.")
		}
		if !u.Can(PermLibEdit) {
			return errForbidden
		}
	case "manufacturers":
		if !u.Can(PermLists) {
			return errForbidden
		}
	case "companies":
		if !u.Can(PermCompanies) {
			return errForbidden
		}
	default:
		if !u.Can(PermLibEdit) {
			return errForbidden
		}
	}
	return nil
}

// apiImportPreview checks every row and reports what would happen.
func (a *App) apiImportPreview(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	var in importInput
	if err := decode(r, &in); err != nil {
		return err
	}
	rows, errs, err := a.planImport(ctx, u, &in)
	if err != nil {
		return err
	}
	return ok(w, map[string]any{"rows": rows, "errors": errs})
}

func (a *App) planImport(ctx context.Context, u *User, in *importInput) ([]importRow, int, error) {
	if err := a.canImport(ctx, u, in); err != nil {
		return nil, 0, err
	}
	recs, header, err := a.readImportFile(in.SHA256)
	if err != nil {
		return nil, 0, err
	}
	for _, c := range importRequired[in.Type] {
		if !contains(header, c) {
			return nil, 0, badRequest("The file needs a %q column. Download a blank file for the column names.", c)
		}
	}
	var out []importRow
	errs := 0
	seen := map[string]int{}
	for i, rec := range recs {
		row := importRow{Row: i + 2, Values: rec}
		row.Outcome, row.ID, row.Error = a.checkImportRow(ctx, u, in, rec)
		if row.Error == "" && row.ID != "" {
			if first, dup := seen[row.ID]; dup {
				row.Error = fmt.Sprintf("Same record as row %d.", first)
			} else {
				seen[row.ID] = row.Row
			}
		}
		if row.Error != "" {
			row.Outcome = "Error"
			errs++
		}
		out = append(out, row)
	}
	return out, errs, nil
}

// checkImportRow validates one row and returns its outcome and record id.
func (a *App) checkImportRow(ctx context.Context, u *User, in *importInput, v map[string]string) (string, string, string) {
	listHas := func(list, val string) bool {
		if val == "" {
			return true
		}
		var n int
		a.DB.QueryRow(ctx, `SELECT count(*) FROM admin_lists WHERE list = $1 AND value = $2 AND active`, list, val).Scan(&n)
		return n > 0
	}
	update := in.Mode == "update"
	switch in.Type {
	case "parts", "requirements":
		id := v["ID"]
		if id != "" {
			if e := cache.get(id); e != nil {
				if !update {
					return "", id, fmt.Sprintf("%s already exists. Choose Create and update to change it.", id)
				}
				if e.Item.Status != model.StatusDraft || a.openCOFor(ctx, a.DB, id) != nil {
					return "", id, "Released items change only through a change order."
				}
				if !a.canSee(ctx, u, id) {
					return "", id, "Unknown item."
				}
				return "Update", id, a.checkItemValues(ctx, in, v, listHas)
			}
		}
		if msg := a.checkItemValues(ctx, in, v, listHas); msg != "" {
			return "", id, msg
		}
		if in.Type == "parts" {
			kind := strings.ToLower(v["Kind"])
			switch kind {
			case model.KindProduction:
				if id != "" {
					if _, _, err := model.ParseProduction(id); err != nil {
						return "", id, capitalize(err.Error()) + "."
					}
					if a.idTaken(ctx, id) {
						return "", id, fmt.Sprintf("%s is already used.", id)
					}
				}
			case model.KindReference:
				if id != "" && (model.PartKindOf(id) != model.KindReference || a.idTaken(ctx, id)) {
					return "", id, "Reference numbers have 7 digits and must be unused."
				}
			case model.KindCOTS:
				var active bool
				if err := a.DB.QueryRow(ctx, `SELECT active FROM manufacturers WHERE code = $1`, v["Manufacturer"]).Scan(&active); err != nil || !active {
					return "", id, "Manufacturer must be an active manufacturer code."
				}
				cid, err := model.COTSNumber(v["Manufacturer"], v["Order number"])
				if err != nil {
					return "", id, capitalize(err.Error()) + "."
				}
				if a.idTaken(ctx, cid) {
					return "", cid, fmt.Sprintf("%s is already used.", cid)
				}
				id = cid
			default:
				return "", id, "Kind must be Production, Reference or COTS."
			}
		} else if id != "" {
			return "", id, fmt.Sprintf("%s does not exist. Leave ID blank to create a requirement.", id)
		}
		return "Create", id, ""
	case "bom":
		item := v["Item"]
		if !model.CanBeBOMLine(item) {
			return "", item, fmt.Sprintf("%s cannot be a BOM line.", item)
		}
		if cache.lookup(item) == nil || !a.canSee(ctx, u, item) {
			return "", item, fmt.Sprintf("%s does not exist.", item)
		}
		if item == in.Target {
			return "", item, "An assembly cannot contain itself."
		}
		q, err := strconv.ParseFloat(v["Quantity"], 64)
		if err != nil || q <= 0 {
			return "", item, "Quantity must be a number above zero."
		}
		if f := v["Find"]; f != "" {
			if n, err := strconv.Atoi(f); err != nil || n <= 0 {
				return "", item, "Find must be a whole number."
			}
		}
		return "Create", item, ""
	case "work":
		typ := strings.ToLower(v["Type"])
		if _, okT := workPrefix[typ]; !okT {
			return "", "", "Type must be task, issue, action, epic or ncr."
		}
		if typ == "ncr" {
			return "", "", "Nonconformances are reported one at a time and cannot be imported."
		}
		if strings.TrimSpace(v["Title"]) == "" || len([]rune(v["Title"])) > 150 {
			return "", "", "Title is required (at most 150 characters)."
		}
		if p := v["Priority"]; p != "" && !contains(priorities, p) {
			return "", "", "Priority must be Low, Normal, High or Urgent."
		}
		if d := v["Due date"]; d != "" {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				return "", "", "Due date must be YYYY-MM-DD."
			}
		}
		if typ == "issue" && (!listHas("issue_areas", v["Area"]) || v["Area"] == "" || !contains(severities, v["Severity"])) {
			return "", "", "Issues need an Area and a Severity column with known values."
		}
		if e := v["Estimate"]; e != "" {
			h, err := strconv.ParseFloat(e, 64)
			if err != nil || h < 0 || h*4 != float64(int(h*4)) {
				return "", "", "Estimate must be hours in quarter-hour steps."
			}
		}
		if as := v["Assignee"]; as != "" {
			if a.userIDByName(ctx, as) == "" {
				return "", "", fmt.Sprintf("Unknown assignee %s.", as)
			}
		}
		if id := v["ID"]; id != "" {
			if !update {
				return "", id, "Leave ID blank to create items, or choose Create and update."
			}
			wi, err := a.workForUser(ctx, u, id)
			if err != nil || wi.Space != in.Target || wi.Type != typ {
				return "", id, fmt.Sprintf("%s is not a %s in this space.", id, typ)
			}
			return "Update", id, ""
		}
		return "Create", "", ""
	case "manufacturers":
		code := strings.ToUpper(v["Code"])
		if !mfrCodeRe.MatchString(code) {
			return "", code, "Code must be 2 to 5 capital letters."
		}
		if strings.TrimSpace(v["Name"]) == "" {
			return "", code, "Name is required."
		}
		var n int
		a.DB.QueryRow(ctx, `SELECT count(*) FROM manufacturers WHERE code = $1`, code).Scan(&n)
		if n > 0 {
			if !update {
				return "", code, fmt.Sprintf("%s already exists. Choose Create and update to change it.", code)
			}
			return "Update", code, ""
		}
		return "Create", code, ""
	case "companies":
		name := strings.TrimSpace(v["Name"])
		if name == "" {
			return "", "", "Name is required."
		}
		for _, k := range splitList(v["Kind"]) {
			if !contains(companyKinds, k) {
				return "", "", fmt.Sprintf("Unknown kind %s. Use Vendor, Contractor, Customer or Other.", k)
			}
		}
		if len(splitList(v["Kind"])) == 0 {
			return "", "", "Kind is required."
		}
		if s := v["Supplier status"]; !contains(supplierStatuses, s) {
			return "", "", "Supplier status must be Approved, Conditional or Not approved."
		}
		var id string
		a.DB.QueryRow(ctx, `SELECT id FROM companies WHERE id = $1 OR lower(name) = lower($2) LIMIT 1`, v["ID"], name).Scan(&id)
		if id != "" {
			if !update {
				return "", id, fmt.Sprintf("%s already exists. Choose Create and update to change it.", name)
			}
			return "Update", id, ""
		}
		if v["ID"] != "" {
			return "", v["ID"], fmt.Sprintf("%s does not exist. Leave ID blank to create a company.", v["ID"])
		}
		return "Create", "", ""
	}
	return "", "", "Unknown import type."
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.FieldsFunc(s, func(r rune) bool { return r == ';' || r == ',' }) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func (a *App) idTaken(ctx context.Context, id string) bool {
	var n int
	a.DB.QueryRow(ctx, `SELECT count(*) FROM identifiers WHERE id = $1`, id).Scan(&n)
	return n > 0
}

func (a *App) userIDByName(ctx context.Context, name string) string {
	var id string
	a.DB.QueryRow(ctx, `SELECT id::text FROM users WHERE active AND (lower(username) = lower($1) OR lower(display_name) = lower($1)) LIMIT 1`, name).Scan(&id)
	return id
}

// checkItemValues validates list values and the home space of a library row.
func (a *App) checkItemValues(ctx context.Context, in *importInput, v map[string]string, listHas func(string, string) bool) string {
	space := v["Home space"]
	if space == "" {
		space = in.Target
	}
	var ok bool
	a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM spaces WHERE id = $1 AND NOT archived)`, space).Scan(&ok)
	if !ok {
		return "Home space must be a space ID, or choose a space for the import."
	}
	if in.Type == "parts" {
		if strings.TrimSpace(v["Name"]) == "" || len([]rune(v["Name"])) > 50 {
			return "Name is required (at most 50 characters)."
		}
		if !listHas("units", v["Unit"]) {
			return fmt.Sprintf("Unknown unit %s.", v["Unit"])
		}
		if !listHas("commodity_codes", v["Commodity code"]) {
			return fmt.Sprintf("Unknown commodity code %s.", v["Commodity code"])
		}
		if t := v["Tracking"]; t != "" && !contains([]string{"Serial", "Lot", "None"}, t) {
			return "Tracking must be Serial, Lot or None."
		}
		if l := v["Lifecycle"]; l != "" && !contains([]string{"Active", "Not Recommended for New Design", "Obsolete"}, l) {
			return "Unknown lifecycle status."
		}
	} else {
		if strings.TrimSpace(v["Title"]) == "" || strings.TrimSpace(v["Statement"]) == "" {
			return "Title and statement are required."
		}
		if !listHas("requirement_categories", v["Category"]) {
			return fmt.Sprintf("Unknown category %s.", v["Category"])
		}
	}
	if o := v["Owner"]; o != "" {
		var n int
		a.DB.QueryRow(ctx, `SELECT count(*) FROM users WHERE lower(username) = lower($1) AND active`, o).Scan(&n)
		if n == 0 {
			return fmt.Sprintf("Unknown owner %s (use the username).", o)
		}
	}
	return ""
}

// apiImportCommit runs an import after checking it again. It saves nothing
// while any row has an error.
func (a *App) apiImportCommit(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	var in importInput
	if err := decode(r, &in); err != nil {
		return err
	}
	rows, errs, err := a.planImport(ctx, u, &in)
	if err != nil {
		return err
	}
	if errs > 0 {
		return &Error{Status: http.StatusBadRequest, Code: "import_errors", Message: fmt.Sprintf("%d rows have errors. Fix them and try again.", errs), Data: rows}
	}
	created, updated := 0, 0
	ip := clientIP(r)
	if in.Type == "bom" {
		if err := a.importBOM(ctx, u, ip, &in, rows); err != nil {
			return err
		}
		created = len(rows)
	} else {
		for _, row := range rows {
			if err := a.importRow(ctx, u, ip, &in, row); err != nil {
				msg := err.Error()
				return &Error{Status: http.StatusBadRequest, Code: "import_stopped",
					Message: fmt.Sprintf("Row %d: %s %d rows before it were saved.", row.Row, strings.TrimSuffix(msg, ".")+".", created+updated)}
			}
			if row.Outcome == "Update" {
				updated++
			} else {
				created++
			}
		}
	}
	var id string
	a.DB.QueryRow(ctx, `INSERT INTO csv_imports (record_type, target, mode, file_sha256, file_name, rows, created, updated, imported_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id::text`, in.Type, in.Target, in.Mode, in.SHA256, sanitizeFileName(in.Name), len(rows), created, updated, u.ID).Scan(&id)
	a.Audit(ctx, u, ip, "import.csv", in.Target, map[string]any{"type": in.Type, "file": in.Name, "rows": len(rows), "created": created, "updated": updated})
	a.notify(ctx, nil, u.ID, "Import finished", "", fmt.Sprintf("Import of %s finished: %d created, %d updated.", in.Name, created, updated))
	a.Events.Publish(Event{Type: "record"})
	return ok(w, map[string]any{"id": id, "created": created, "updated": updated})
}

func (a *App) importRow(ctx context.Context, u *User, ip string, in *importInput, row importRow) error {
	v := row.Values
	switch in.Type {
	case "parts", "requirements":
		var it model.Item
		var e *entry
		if row.Outcome == "Update" {
			e = cache.get(row.ID)
			c := *e.Item
			it = c
		} else {
			it.Type = model.TypePart
			if in.Type == "requirements" {
				it.Type = model.TypeRequirement
			}
		}
		set := func(col string, dst *string) {
			if val, okv := v[col]; okv && (val != "" || row.Outcome == "Update") {
				*dst = val
			}
		}
		if s := v["Home space"]; s != "" {
			it.Space = s
		} else if it.Space == "" {
			it.Space = in.Target
		}
		set("Owner", &it.Owner)
		if l, okv := v["Labels"]; okv {
			it.Labels = cleanLabels(splitList(l))
		}
		if in.Type == "parts" {
			if row.Outcome != "Update" {
				it.Kind = strings.ToLower(v["Kind"])
			}
			set("Name", &it.Title)
			set("Description", &it.Description)
			set("Unit", &it.Unit)
			set("Tracking", &it.Tracking)
			set("Commodity code", &it.Commodity)
			set("Type number", &it.TypeNumber)
			set("Lifecycle", &it.Lifecycle)
			set("Unit cost", &it.UnitCost)
			if d := v["Lead time days"]; d != "" {
				it.LeadTimeDays, _ = strconv.Atoi(d)
			}
			if row.Outcome != "Update" {
				it.Manufacturer = v["Manufacturer"]
			}
		} else {
			set("Title", &it.Title)
			set("Statement", &it.Statement)
			set("Rationale", &it.Rationale)
			set("Category", &it.Category)
			set("Source", &it.Source)
			set("Priority", &it.Priority)
		}
		if row.Outcome == "Update" {
			_, err := a.SaveItem(ctx, u, ip, e, &it, e.Hash, "Updated by CSV import")
			return err
		}
		num := v["ID"]
		_, err := a.CreateItem(ctx, u, ip, createInput{Item: it, Number: num, OrderNumber: v["Order number"], Comment: "Created by CSV import"})
		return err
	case "work":
		typ := strings.ToLower(v["Type"])
		title, desc, prio, due := v["Title"], v["Description"], v["Priority"], v["Due date"]
		wi := workInput{Type: typ, Space: in.Target, Title: &title}
		if _, okv := v["Description"]; okv {
			wi.Description = &desc
		}
		if prio != "" {
			wi.Priority = &prio
		}
		if due != "" {
			wi.DueDate = &due
		}
		if as := v["Assignee"]; as != "" {
			id := a.userIDByName(ctx, as)
			wi.AssigneeID = &id
		}
		if l, okv := v["Labels"]; okv {
			ls := splitList(l)
			wi.Labels = &ls
		}
		if e := v["Estimate"]; e != "" {
			wi.Estimate = []byte(e)
		}
		if typ == "issue" {
			wi.Fields = map[string]any{"area": v["Area"], "severity": v["Severity"]}
		}
		if row.Outcome == "Update" {
			return a.updateWorkFromImport(ctx, u, row.ID, wi)
		}
		_, err := a.CreateWork(ctx, u, ip, wi)
		return err
	case "manufacturers":
		code := strings.ToUpper(v["Code"])
		active := !strings.EqualFold(v["Active"], "no")
		if row.Outcome == "Update" {
			_, err := a.DB.Exec(ctx, `UPDATE manufacturers SET name = $2, website = $3, active = $4 WHERE code = $1`, code, v["Name"], v["Website"], active)
			return err
		}
		_, err := a.DB.Exec(ctx, `INSERT INTO manufacturers (code, name, website, active) VALUES ($1, $2, $3, $4)`, code, v["Name"], v["Website"], active)
		return err
	case "companies":
		name, kinds, status, addr, phone, web, notes := v["Name"], splitList(v["Kind"]), v["Supplier status"], v["Address"], v["Phone"], v["Website"], v["Notes"]
		ci := companyInput{Name: &name, Kinds: &kinds, SupplierStatus: &status, Address: &addr, Phone: &phone, Website: &web, Notes: &notes}
		if codes := splitList(v["Manufacturer codes"]); len(codes) > 0 {
			ci.ManufacturerCodes = &codes
		}
		if act, okv := v["Active"]; okv && act != "" {
			b := !strings.EqualFold(act, "no")
			ci.Active = &b
		}
		if row.Outcome == "Update" {
			c, err := a.companyForUser(ctx, u, row.ID)
			if err != nil {
				return err
			}
			if err := a.applyCompanyInput(ctx, c, &ci); err != nil {
				return err
			}
			return a.saveCompany(ctx, u, ip, c)
		}
		_, err := a.CreateCompany(ctx, u, ip, ci)
		return err
	}
	return nil
}

// saveCompany writes a company changed by an import.
func (a *App) saveCompany(ctx context.Context, u *User, ip string, c *Company) error {
	_, err := a.DB.Exec(ctx, `UPDATE companies SET name = $2, kinds = $3, supplier_status = $4, manufacturer_codes = $5, address = $6, phone = $7, website = $8,
		notes = $9, active = $10, updated_at = now(), version = version + 1 WHERE id = $1`,
		c.ID, c.Name, c.Kinds, c.SupplierStatus, c.ManufacturerCodes, c.Address, c.Phone, c.Website, c.Notes, c.Active)
	if err != nil {
		return err
	}
	a.DB.Exec(ctx, `INSERT INTO company_history (company_id, user_id, changes) VALUES ($1, $2, '{"import": true}')`, c.ID, u.ID)
	a.Audit(ctx, u, ip, "company.update", c.ID, map[string]any{"import": true})
	return a.indexCompany(ctx, a.DB, c)
}

// updateWorkFromImport applies an imported row to an existing work item
// through the same rules as the work item view.
func (a *App) updateWorkFromImport(ctx context.Context, u *User, id string, in workInput) error {
	wi, err := a.workForUser(ctx, u, id)
	if err != nil {
		return err
	}
	in.Type, in.Space = "", ""
	before := wi.Version
	if err := a.applyWorkInput(ctx, wi, &in, false); err != nil {
		return err
	}
	if err := checkWorkRequired(wi); err != nil {
		return err
	}
	var due any
	if wi.DueDate != nil {
		due = *wi.DueDate
	}
	tag, err := a.DB.Exec(ctx, `UPDATE work_items SET title = $2, description = $3, priority = $4, assignee_id = $5, due_date = $6, labels = $7, estimate = $8, fields = $9,
		updated_at = now(), version = version + 1 WHERE id = $1 AND version = $10`, wi.ID, wi.Title, wi.Description, wi.Priority, wi.AssigneeID, due, nz(wi.Labels), wi.Estimate,
		mustJSON(wi.Fields), before)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return badRequest("%s changed during the import.", id)
	}
	a.DB.Exec(ctx, `INSERT INTO work_history (item_id, user_id, changes) VALUES ($1, $2, '{"import": true}')`, wi.ID, u.ID)
	return a.indexWork(ctx, a.DB, wi)
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// importBOM adds lines to, or replaces the BOM of, a draft assembly in one save.
func (a *App) importBOM(ctx context.Context, u *User, ip string, in *importInput, rows []importRow) error {
	e := cache.get(in.Target)
	it := *e.Item
	var lines []model.BOMLine
	if in.Mode != "replace" {
		lines = append(lines, it.BOM...)
	}
	maxFind := 0
	for _, l := range lines {
		if l.Find > maxFind {
			maxFind = l.Find
		}
	}
	for _, row := range rows {
		v := row.Values
		q, _ := strconv.ParseFloat(v["Quantity"], 64)
		find, _ := strconv.Atoi(v["Find"])
		if find == 0 {
			maxFind += 10
			find = maxFind
		} else if find > maxFind {
			maxFind = find
		}
		lines = append(lines, model.BOMLine{Find: find, Item: v["Item"], Qty: q, RefDes: strings.Fields(v["Reference designators"]), Notes: v["Notes"]})
	}
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Find < lines[j].Find })
	it.BOM = lines
	_, err := a.SaveItem(ctx, u, ip, e, &it, e.Hash, "BOM lines imported from CSV")
	return err
}

// apiImports lists past imports for Admins.
func (a *App) apiImports(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermUsers) && !u.Can(PermImport) {
		return errForbidden
	}
	where := ""
	args := []any{}
	if !u.Can(PermUsers) {
		where = ` WHERE i.imported_by = $1`
		args = append(args, u.ID)
	}
	rows, err := a.DB.Query(r.Context(), `SELECT i.id::text, i.record_type, i.target, i.mode, i.file_sha256, i.file_name, i.rows, i.created, i.updated, us.display_name, i.imported_at
		FROM csv_imports i JOIN users us ON us.id = i.imported_by`+where+` ORDER BY i.imported_at DESC LIMIT 500`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		ID      string    `json:"id"`
		Type    string    `json:"type"`
		Target  string    `json:"target"`
		Mode    string    `json:"mode"`
		SHA256  string    `json:"sha256"`
		File    string    `json:"file"`
		Rows    int       `json:"rows"`
		Created int       `json:"created"`
		Updated int       `json:"updated"`
		By      string    `json:"by"`
		At      time.Time `json:"at"`
	}
	out := []row{}
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.ID, &x.Type, &x.Target, &x.Mode, &x.SHA256, &x.File, &x.Rows, &x.Created, &x.Updated, &x.By, &x.At); err != nil {
			return err
		}
		out = append(out, x)
	}
	var samples []map[string]string
	a.Setting("import_samples", &samples)
	return ok(w, map[string]any{"imports": out, "samples": nzMaps(samples)})
}

func nzMaps(m []map[string]string) []map[string]string {
	if m == nil {
		return []map[string]string{}
	}
	return m
}

// apiImportSample downloads one of the seed sample files.
func (a *App) apiImportSample(w http.ResponseWriter, r *http.Request) error {
	var samples []map[string]string
	a.Setting("import_samples", &samples)
	for _, smp := range samples {
		if smp["sha256"] == r.PathValue("sha") {
			f, err := a.Files.Open(smp["sha256"])
			if err != nil {
				return errNotFound
			}
			defer f.Close()
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, smp["name"]))
			_, err = io.Copy(w, f)
			return err
		}
	}
	return errNotFound
}

// apiImportFile downloads the file of a past import.
func (a *App) apiImportFile(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	var sum, name, by string
	if err := a.DB.QueryRow(ctx, `SELECT file_sha256, file_name, imported_by::text FROM csv_imports WHERE id = $1`, r.PathValue("id")).Scan(&sum, &name, &by); err != nil {
		return errNotFound
	}
	if !u.Can(PermUsers) && by != u.ID {
		return errForbidden
	}
	f, err := a.Files.Open(sum)
	if err != nil {
		return errNotFound
	}
	defer f.Close()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	_, err = io.Copy(w, f)
	return err
}

// apiImportTemplate gives a blank file with the import columns.
func (a *App) apiImportTemplate(w http.ResponseWriter, r *http.Request) error {
	typ := r.PathValue("type")
	cols, okT := map[string][]string{
		"parts":         {"ID", "Kind", "Name", "Description", "Unit", "Tracking", "Commodity code", "Home space", "Owner", "Manufacturer", "Order number", "Type number", "Lifecycle", "Unit cost", "Lead time days", "Labels"},
		"bom":           {"Find", "Item", "Quantity", "Reference designators", "Notes"},
		"requirements":  {"ID", "Title", "Statement", "Rationale", "Category", "Source", "Priority", "Home space", "Owner", "Labels"},
		"work":          {"ID", "Type", "Title", "Description", "Priority", "Assignee", "Due date", "Labels", "Estimate", "Area", "Severity"},
		"manufacturers": {"Code", "Name", "Website", "Active"},
		"companies":     {"ID", "Name", "Kind", "Supplier status", "Manufacturer codes", "Address", "Phone", "Website", "Notes", "Active"},
	}[typ]
	if !okT {
		return errNotFound
	}
	return writeCSV(w, "designhub-"+typ+"-import.csv", cols, nil)
}
