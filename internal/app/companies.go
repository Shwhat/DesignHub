package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"designhub/internal/model"
)

// Companies (DATA-MODEL.md 7.20) and the access of company-linked accounts
// (section 5.5).

var companyKinds = []string{"Vendor", "Contractor", "Customer", "Other"}
var supplierStatuses = []string{"", "Approved", "Conditional", "Not approved"}

// Files a Vendor may see on supplied items, unless native files are allowed.
var vendorFileRoles = []string{"Drawing", "Datasheet", "3D model", "Schematic", "BOM file"}

func removeString(list []string, s string) []string {
	out := list[:0:0]
	for _, x := range list {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

// loadVendorScope fills the set of records a Vendor may see: released items
// supplied by the company, NCRs with the company as supplier, and actions
// assigned to the vendor.
func (a *App) loadVendorScope(ctx context.Context, u *User) error {
	u.VendorSees = map[string]bool{}
	if u.Company == "" {
		return nil
	}
	rows, err := a.DB.Query(ctx, `SELECT l.src FROM links l WHERE l.type = 'supplied by' AND l.dst = $1
			AND EXISTS (SELECT 1 FROM item_revisions r WHERE r.item_id = l.src)
		UNION SELECT l.src FROM links l WHERE l.type = 'supplier' AND l.dst = $1
		UNION SELECT id FROM work_items WHERE type = 'action' AND assignee_id = $2`, u.Company, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		u.VendorSees[id] = true
	}
	return rows.Err()
}

// vendorItem returns the view of an item a Vendor gets: the released
// revision, with only the files Vendors may download.
func vendorItem(it *model.Item) *model.Item {
	c := *it
	c.Files = nil
	for _, f := range it.Files {
		if contains(vendorFileRoles, f.Role) || (it.VendorDownload && f.Role == "Native design files") {
			c.Files = append(c.Files, f)
		}
	}
	return &c
}

// ---------------------------------------------------------------------------
// Company records

type Contact struct {
	Name  string `json:"name"`
	Role  string `json:"role"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type Company struct {
	ID                string       `json:"id"`
	Name              string       `json:"name"`
	Kinds             []string     `json:"kinds"`
	SupplierStatus    string       `json:"supplierStatus"`
	ManufacturerCodes []string     `json:"manufacturerCodes"`
	Address           string       `json:"address"`
	Phone             string       `json:"phone"`
	Website           string       `json:"website"`
	Contacts          []Contact    `json:"contacts"`
	Notes             string       `json:"notes"`
	Labels            []string     `json:"labels"`
	Attachments       []model.File `json:"attachments"`
	Active            bool         `json:"active"`
	CreatedAt         time.Time    `json:"createdAt"`
	UpdatedAt         time.Time    `json:"updatedAt"`
	Version           int          `json:"version"`
	Supplied          int          `json:"supplied"`
}

const companySelect = `SELECT c.id, c.name, c.kinds, c.supplier_status, c.manufacturer_codes, c.address, c.phone, c.website, c.contacts, c.notes,
	c.labels, c.attachments, c.active, c.created_at, c.updated_at, c.version,
	(SELECT count(*) FROM links l WHERE l.type = 'supplied by' AND l.dst = c.id) FROM companies c`

func scanCompany(row pgx.Row) (*Company, error) {
	c := &Company{}
	var contacts, att []byte
	if err := row.Scan(&c.ID, &c.Name, &c.Kinds, &c.SupplierStatus, &c.ManufacturerCodes, &c.Address, &c.Phone, &c.Website, &contacts, &c.Notes,
		&c.Labels, &att, &c.Active, &c.CreatedAt, &c.UpdatedAt, &c.Version, &c.Supplied); err != nil {
		return nil, err
	}
	json.Unmarshal(contacts, &c.Contacts)
	json.Unmarshal(att, &c.Attachments)
	if c.Contacts == nil {
		c.Contacts = []Contact{}
	}
	if c.Attachments == nil {
		c.Attachments = []model.File{}
	}
	c.Kinds, c.ManufacturerCodes, c.Labels = nz(c.Kinds), nz(c.ManufacturerCodes), nz(c.Labels)
	return c, nil
}

func (a *App) indexCompany(ctx context.Context, q querier, c *Company) error {
	status := "Active"
	if !c.Active {
		status = "Inactive"
	}
	_, err := q.Exec(ctx, `INSERT INTO records (id, kind, title, status, labels, body, origin, updated_at)
		VALUES ($1, 'company', $2, $3, $4, $5, 'db', now())
		ON CONFLICT (id) DO UPDATE SET title = $2, status = $3, labels = $4, body = $5, updated_at = now()`,
		c.ID, c.Name, status, nz(c.Labels), c.Notes+"\n"+c.Address+"\n"+strings.Join(c.Kinds, " "))
	return err
}

func (a *App) companyForUser(ctx context.Context, u *User, id string) (*Company, error) {
	if !visible(u, id, "company", "", nil) {
		return nil, errNotFound
	}
	c, err := scanCompany(a.DB.QueryRow(ctx, companySelect+` WHERE c.id = $1`, id))
	if err != nil {
		return nil, errNotFound
	}
	return c, nil
}

func (a *App) apiCompanies(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !visible(u, "", "company", "", nil) {
		return errForbidden
	}
	rows, err := a.DB.Query(r.Context(), companySelect+` ORDER BY lower(c.name)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []*Company{}
	for rows.Next() {
		c, err := scanCompany(rows)
		if err != nil {
			return err
		}
		if r.URL.Query().Get("active") == "1" && !c.Active {
			continue
		}
		out = append(out, c)
	}
	return ok(w, out)
}

type companyInput struct {
	Name              *string       `json:"name"`
	Kinds             *[]string     `json:"kinds"`
	SupplierStatus    *string       `json:"supplierStatus"`
	ManufacturerCodes *[]string     `json:"manufacturerCodes"`
	Address           *string       `json:"address"`
	Phone             *string       `json:"phone"`
	Website           *string       `json:"website"`
	Contacts          *[]Contact    `json:"contacts"`
	Notes             *string       `json:"notes"`
	Labels            *[]string     `json:"labels"`
	Attachments       *[]model.File `json:"attachments"`
	Active            *bool         `json:"active"`
	Version           int           `json:"version"`
}

func (a *App) applyCompanyInput(ctx context.Context, c *Company, in *companyInput) error {
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" || len([]rune(n)) > 100 {
			return badRequest("Name is required (at most 100 characters).")
		}
		c.Name = n
	}
	if in.Kinds != nil {
		c.Kinds = []string{}
		for _, k := range *in.Kinds {
			if !contains(companyKinds, k) {
				return badRequest("Unknown company kind %q.", k)
			}
			if !contains(c.Kinds, k) {
				c.Kinds = append(c.Kinds, k)
			}
		}
	}
	if len(c.Kinds) == 0 {
		return badRequest("Choose at least one kind.")
	}
	if in.SupplierStatus != nil {
		if !contains(supplierStatuses, *in.SupplierStatus) {
			return badRequest("Unknown supplier status.")
		}
		c.SupplierStatus = *in.SupplierStatus
	}
	if in.ManufacturerCodes != nil {
		c.ManufacturerCodes = []string{}
		for _, m := range *in.ManufacturerCodes {
			var n int
			a.DB.QueryRow(ctx, `SELECT count(*) FROM manufacturers WHERE code = $1`, m).Scan(&n)
			if n == 0 {
				return badRequest("Unknown manufacturer code %s.", m)
			}
			c.ManufacturerCodes = append(c.ManufacturerCodes, m)
		}
	}
	str := func(p *string, dst *string) {
		if p != nil {
			*dst = strings.TrimSpace(*p)
		}
	}
	str(in.Address, &c.Address)
	str(in.Phone, &c.Phone)
	str(in.Website, &c.Website)
	str(in.Notes, &c.Notes)
	if in.Contacts != nil {
		c.Contacts = []Contact{}
		for _, ct := range *in.Contacts {
			if strings.TrimSpace(ct.Name) == "" {
				continue
			}
			c.Contacts = append(c.Contacts, ct)
		}
	}
	if in.Labels != nil {
		c.Labels = cleanLabels(*in.Labels)
	}
	if in.Attachments != nil {
		c.Attachments = *in.Attachments
	}
	if in.Active != nil {
		c.Active = *in.Active
	}
	return nil
}

func (a *App) apiCreateCompany(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	var in companyInput
	if err := decode(r, &in); err != nil {
		return err
	}
	id, err := a.CreateCompany(r.Context(), u, clientIP(r), in)
	if err != nil {
		return err
	}
	return ok(w, map[string]string{"id": id})
}

// CreateCompany creates a company record.
func (a *App) CreateCompany(ctx context.Context, u *User, ip string, in companyInput) (string, error) {
	if !u.Can(PermCompanies) {
		return "", errForbidden
	}
	c := &Company{Active: true, Kinds: []string{}, Contacts: []Contact{}, Labels: []string{}, Attachments: []model.File{}, ManufacturerCodes: []string{}}
	if in.Kinds == nil {
		k := []string{"Vendor"}
		in.Kinds = &k
	}
	if err := a.applyCompanyInput(ctx, c, &in); err != nil {
		return "", err
	}
	if c.Name == "" {
		return "", badRequest("Name is required.")
	}
	err := a.tx(ctx, func(tx pgx.Tx) error {
		var dup int
		tx.QueryRow(ctx, `SELECT count(*) FROM companies WHERE lower(name) = lower($1)`, c.Name).Scan(&dup)
		if dup > 0 {
			return badRequest("A company named %s already exists.", c.Name)
		}
		id, err := a.nextPrefixed(ctx, tx, "COMP", "company", u.ID)
		if err != nil {
			return err
		}
		c.ID = id
		contacts, _ := json.Marshal(c.Contacts)
		att, _ := json.Marshal(c.Attachments)
		if _, err := tx.Exec(ctx, `INSERT INTO companies (id, name, kinds, supplier_status, manufacturer_codes, address, phone, website, contacts, notes, labels, attachments, active, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
			id, c.Name, c.Kinds, c.SupplierStatus, c.ManufacturerCodes, c.Address, c.Phone, c.Website, contacts, c.Notes, c.Labels, att, c.Active, u.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO company_history (company_id, user_id, changes) VALUES ($1, $2, '{"created": true}')`, id, u.ID); err != nil {
			return err
		}
		if err := a.indexCompany(ctx, tx, c); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, ip, "company.create", id, map[string]any{"name": c.Name})
		return nil
	})
	return c.ID, err
}

func (a *App) apiUpdateCompany(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	if !u.Can(PermCompanies) {
		return errForbidden
	}
	c, err := a.companyForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	var in companyInput
	if err := decode(r, &in); err != nil {
		return err
	}
	if in.Version != 0 && in.Version != c.Version {
		return conflict("Someone changed this company after you opened it. Reload to see their changes.", nil)
	}
	before, _ := json.Marshal(c)
	beforeName, beforeActive := c.Name, c.Active
	if err := a.applyCompanyInput(ctx, c, &in); err != nil {
		return err
	}
	after, _ := json.Marshal(c)
	if string(before) == string(after) {
		return ok(w, map[string]int{"version": c.Version})
	}
	changes := diffJSON(before, after)
	err = a.tx(ctx, func(tx pgx.Tx) error {
		if c.Name != beforeName {
			var dup int
			tx.QueryRow(ctx, `SELECT count(*) FROM companies WHERE lower(name) = lower($1) AND id <> $2`, c.Name, c.ID).Scan(&dup)
			if dup > 0 {
				return badRequest("A company named %s already exists.", c.Name)
			}
		}
		contacts, _ := json.Marshal(c.Contacts)
		att, _ := json.Marshal(c.Attachments)
		tag, err := tx.Exec(ctx, `UPDATE companies SET name = $2, kinds = $3, supplier_status = $4, manufacturer_codes = $5, address = $6, phone = $7, website = $8,
			contacts = $9, notes = $10, labels = $11, attachments = $12, active = $13, updated_at = now(), version = version + 1 WHERE id = $1 AND version = $14`,
			c.ID, c.Name, c.Kinds, c.SupplierStatus, c.ManufacturerCodes, c.Address, c.Phone, c.Website, contacts, c.Notes, c.Labels, att, c.Active, c.Version)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return conflict("Someone changed this company after you opened it. Reload to see their changes.", nil)
		}
		cj, _ := json.Marshal(changes)
		if _, err := tx.Exec(ctx, `INSERT INTO company_history (company_id, user_id, changes) VALUES ($1, $2, $3)`, c.ID, u.ID, cj); err != nil {
			return err
		}
		if beforeActive && !c.Active {
			// Inactive companies have no working Vendor or Contractor accounts.
			if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id IN (SELECT id FROM users WHERE company_id = $1)`, c.ID); err != nil {
				return err
			}
		}
		if err := a.indexCompany(ctx, tx, c); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, clientIP(r), "company.update", c.ID, changes)
		return nil
	})
	if err != nil {
		return err
	}
	a.Events.Publish(Event{Type: "record", ID: c.ID})
	return ok(w, map[string]int{"version": c.Version + 1})
}

// diffJSON lists the top-level fields that differ between two JSON objects.
func diffJSON(before, after []byte) map[string]any {
	var b, a map[string]any
	json.Unmarshal(before, &b)
	json.Unmarshal(after, &a)
	out := map[string]any{}
	for k, v := range a {
		bv, _ := json.Marshal(b[k])
		av, _ := json.Marshal(v)
		if string(bv) != string(av) && k != "updatedAt" && k != "version" {
			out[k] = map[string]any{"from": b[k], "to": v}
		}
	}
	return out
}

func (a *App) apiCompany(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	c, err := a.companyForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	type ref struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Status   string `json:"status"`
		Revision string `json:"revision"`
		Kind     string `json:"kind"`
	}
	supplied, ncrs := []ref{}, []ref{}
	rows, err := a.DB.Query(ctx, `SELECT l.src, l.type, COALESCE(r.title, ''), COALESCE(r.status, ''), COALESCE(r.revision, ''), COALESCE(r.kind, '')
		FROM links l LEFT JOIN records r ON r.id = l.src WHERE l.dst = $1 AND l.type IN ('supplied by', 'supplier') ORDER BY l.src`, c.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var x ref
		var typ string
		if err := rows.Scan(&x.ID, &typ, &x.Title, &x.Status, &x.Revision, &x.Kind); err != nil {
			rows.Close()
			return err
		}
		if !vis(x.ID) {
			continue
		}
		if typ == "supplied by" {
			supplied = append(supplied, x)
		} else {
			ncrs = append(ncrs, x)
		}
	}
	rows.Close()
	type person struct {
		ID          string     `json:"id"`
		DisplayName string     `json:"displayName"`
		Role        string     `json:"role"`
		ExpiresOn   *time.Time `json:"expiresOn"`
		Active      bool       `json:"active"`
	}
	people := []person{}
	rows, err = a.DB.Query(ctx, `SELECT us.id::text, us.display_name, ro.name, us.expires_on::timestamptz, us.active FROM users us JOIN roles ro ON ro.id = us.library_role_id
		WHERE us.company_id = $1 ORDER BY lower(us.display_name)`, c.ID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var p person
		if err := rows.Scan(&p.ID, &p.DisplayName, &p.Role, &p.ExpiresOn, &p.Active); err != nil {
			rows.Close()
			return err
		}
		people = append(people, p)
	}
	rows.Close()
	hist := []workHistoryRow{}
	rows, err = a.DB.Query(ctx, `SELECT h.at, COALESCE(us.display_name, ''), h.changes FROM company_history h LEFT JOIN users us ON us.id = h.user_id
		WHERE h.company_id = $1 ORDER BY h.at DESC LIMIT 300`, c.ID)
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
	var watching bool
	a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM watchers WHERE record_id = $1 AND user_id = $2)`, c.ID, u.ID).Scan(&watching)
	return ok(w, map[string]any{"company": c, "supplied": supplied, "ncrs": ncrs, "people": people, "history": hist,
		"canEdit": u.Can(PermCompanies), "canManagePeople": u.Can(PermUsers), "watching": watching})
}

// apiSupplied adds or removes supplied-by links between a company and items.
// The link is a business relationship and needs no change order.
func (a *App) apiSupplied(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	if !u.Can(PermCompanies) && !u.Can(PermLibEdit) {
		return errForbidden
	}
	c, err := a.companyForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	var in struct {
		Add    []string `json:"add"`
		Remove []string `json:"remove"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	if err := a.setSupplied(ctx, u, clientIP(r), c, in.Add, in.Remove); err != nil {
		return err
	}
	return ok(w, nil)
}

func (a *App) setSupplied(ctx context.Context, u *User, ip string, c *Company, add, remove []string) error {
	return a.tx(ctx, func(tx pgx.Tx) error {
		for _, id := range add {
			e := cache.get(id)
			if e == nil || e.Item.Type != model.TypePart || !a.canSee(ctx, u, id) {
				return badRequest("%s is not a part or assembly.", id)
			}
			if !c.Active {
				return badRequest("%s is inactive and cannot be linked.", c.Name)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO links (src, type, dst, origin) VALUES ($1, 'supplied by', $2, 'db') ON CONFLICT DO NOTHING`, id, c.ID); err != nil {
				return err
			}
		}
		for _, id := range remove {
			if _, err := tx.Exec(ctx, `DELETE FROM links WHERE src = $1 AND type = 'supplied by' AND dst = $2`, id, c.ID); err != nil {
				return err
			}
		}
		if len(add)+len(remove) > 0 {
			cj, _ := json.Marshal(map[string]any{"supplied": map[string]any{"added": add, "removed": remove}})
			if _, err := tx.Exec(ctx, `INSERT INTO company_history (company_id, user_id, changes) VALUES ($1, $2, $3)`, c.ID, u.ID, cj); err != nil {
				return err
			}
			a.auditQ(ctx, tx, u, ip, "company.supplied", c.ID, map[string]any{"added": add, "removed": remove})
		}
		return nil
	})
}

// apiItemSuppliers replaces the suppliers of one item.
func (a *App) apiItemSuppliers(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || e.Item.Type != model.TypePart || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	if !u.Can(PermCompanies) && !u.Can(PermLibEdit) {
		return errForbidden
	}
	var in struct {
		Companies []string `json:"companies"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	current, err := a.suppliersOf(ctx, id)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, s := range current {
		have[s.ID] = true
	}
	want := map[string]bool{}
	for _, cid := range in.Companies {
		want[cid] = true
		if !have[cid] {
			c, err := a.companyForUser(ctx, u, cid)
			if err != nil {
				return badRequest("Unknown company %s.", cid)
			}
			if err := a.setSupplied(ctx, u, clientIP(r), c, []string{id}, nil); err != nil {
				return err
			}
		}
	}
	for cid := range have {
		if !want[cid] {
			c, err := a.companyForUser(ctx, u, cid)
			if err != nil {
				continue
			}
			if err := a.setSupplied(ctx, u, clientIP(r), c, nil, []string{id}); err != nil {
				return err
			}
		}
	}
	a.Events.Publish(Event{Type: "record", ID: id})
	return ok(w, nil)
}

type supplierRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"supplierStatus"`
	Active bool   `json:"active"`
}

func (a *App) suppliersOf(ctx context.Context, itemID string) ([]supplierRef, error) {
	rows, err := a.DB.Query(ctx, `SELECT c.id, c.name, c.supplier_status, c.active FROM links l JOIN companies c ON c.id = l.dst
		WHERE l.src = $1 AND l.type IN ('supplied by', 'supplier') ORDER BY lower(c.name)`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []supplierRef{}
	for rows.Next() {
		var s supplierRef
		if err := rows.Scan(&s.ID, &s.Name, &s.Status, &s.Active); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Supplier documents

type supplierDoc struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	SHA256     string    `json:"sha256"`
	Company    string    `json:"company"`
	UploadedBy string    `json:"uploadedBy"`
	UploadedAt time.Time `json:"uploadedAt"`
}

func (a *App) supplierDocs(ctx context.Context, u *User, recordID string) ([]supplierDoc, error) {
	q := `SELECT d.id::text, d.name, d.sha256, c.name, us.display_name, d.uploaded_at FROM supplier_documents d
		JOIN companies c ON c.id = d.company_id JOIN users us ON us.id = d.uploaded_by WHERE d.record_id = $1`
	args := []any{recordID}
	if u.IsVendor() {
		q += ` AND d.company_id = $2`
		args = append(args, u.Company)
	}
	rows, err := a.DB.Query(ctx, q+` ORDER BY d.uploaded_at DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []supplierDoc{}
	for rows.Next() {
		var d supplierDoc
		if err := rows.Scan(&d.ID, &d.Name, &d.SHA256, &d.Company, &d.UploadedBy, &d.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (a *App) apiSupplierDocs(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	if !a.canSee(ctx, u, id) {
		return errNotFound
	}
	docs, err := a.supplierDocs(ctx, u, id)
	if err != nil {
		return err
	}
	return ok(w, docs)
}

// apiAddSupplierDoc lets a Vendor add a document to a record they can see.
// Internal users may add one on behalf of a supplier of the record.
func (a *App) apiAddSupplierDoc(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	if !a.canSee(ctx, u, id) {
		return errNotFound
	}
	var in struct {
		SHA256  string `json:"sha256"`
		Name    string `json:"name"`
		Company string `json:"company"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	company := in.Company
	if u.IsVendor() {
		company = u.Company
	} else if !u.Can(PermLibEdit) && !u.Can(PermWorkEdit) {
		return errForbidden
	}
	var linked int
	a.DB.QueryRow(ctx, `SELECT count(*) FROM links WHERE src = $1 AND dst = $2 AND type IN ('supplied by', 'supplier')`, id, company).Scan(&linked)
	if linked == 0 {
		return badRequest("The company is not a supplier of %s.", id)
	}
	var n int
	a.DB.QueryRow(ctx, `SELECT count(*) FROM files WHERE sha256 = $1`, in.SHA256).Scan(&n)
	if n == 0 {
		return badRequest("The file was not uploaded.")
	}
	name := sanitizeFileName(in.Name)
	if _, err := a.DB.Exec(ctx, `INSERT INTO supplier_documents (record_id, company_id, sha256, name, uploaded_by) VALUES ($1, $2, $3, $4, $5)`,
		id, company, in.SHA256, name, u.ID); err != nil {
		return err
	}
	a.Audit(ctx, u, clientIP(r), "supplier.document", id, map[string]string{"name": name, "company": company})
	a.notifyWatchers(ctx, u, id, "Watched record changed", fmt.Sprintf("%s added supplier document %s to %s", u.DisplayName, name, id))
	a.Events.Publish(Event{Type: "record", ID: id})
	return ok(w, nil)
}

// ---------------------------------------------------------------------------
// Supplier response on NCRs

var supplierResponseFields = []string{"supplier_root_cause", "supplier_corrective_action"}

// apiSupplierResponse saves a Vendor's root cause and corrective action, or
// lets an internal user request a response.
func (a *App) apiSupplierResponse(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	wi, err := a.workForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if wi.Type != "ncr" {
		return badRequest("Supplier responses apply to nonconformances.")
	}
	supplier, _ := wi.Fields["supplier"].(string)
	if supplier == "" {
		return badRequest("Choose a supplier first.")
	}
	var in struct {
		RootCause        *string `json:"rootCause"`
		CorrectiveAction *string `json:"correctiveAction"`
		Requested        *string `json:"requested"`
		Due              *string `json:"due"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	changes := map[string]any{}
	notifyVendors := false
	if in.RootCause != nil || in.CorrectiveAction != nil {
		if !u.Can(PermSupplier) || u.Company != supplier {
			return errForbidden
		}
		if in.RootCause != nil {
			changes["supplier_root_cause"] = strings.TrimSpace(*in.RootCause)
		}
		if in.CorrectiveAction != nil {
			changes["supplier_corrective_action"] = strings.TrimSpace(*in.CorrectiveAction)
		}
		if _, set := wi.Fields["supplier_responded_on"]; !set {
			changes["supplier_responded_on"] = time.Now().UTC().Format(time.RFC3339)
		}
	}
	if in.Requested != nil || in.Due != nil {
		if u.IsVendor() || !u.CanIn(wi.Space, PermWorkEdit) {
			return errForbidden
		}
		for k, p := range map[string]*string{"supplier_response_requested": in.Requested, "supplier_response_due": in.Due} {
			if p == nil {
				continue
			}
			if *p != "" {
				if _, err := time.Parse("2006-01-02", *p); err != nil {
					return badRequest("Dates must be dates.")
				}
			}
			changes[k] = *p
		}
		if in.Requested != nil && *in.Requested != "" {
			notifyVendors = true
		}
	}
	if len(changes) == 0 {
		return ok(w, nil)
	}
	hist := map[string]any{}
	for k, v := range changes {
		hist[k] = map[string]any{"from": wi.Fields[k], "to": v}
		wi.Fields[k] = v
	}
	fields, _ := json.Marshal(wi.Fields)
	err = a.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE work_items SET fields = $2, updated_at = now(), version = version + 1 WHERE id = $1`, wi.ID, fields); err != nil {
			return err
		}
		cj, _ := json.Marshal(hist)
		if _, err := tx.Exec(ctx, `INSERT INTO work_history (item_id, user_id, changes) VALUES ($1, $2, $3)`, wi.ID, u.ID, cj); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, clientIP(r), "ncr.supplier", wi.ID, hist)
		return nil
	})
	if err != nil {
		return err
	}
	if notifyVendors {
		rows, err := a.DB.Query(ctx, `SELECT us.id::text FROM users us JOIN roles ro ON ro.id = us.library_role_id
			WHERE us.company_id = $1 AND us.active AND ro.company_scope = 'vendor'`, supplier)
		if err == nil {
			var ids []string
			for rows.Next() {
				var id string
				if rows.Scan(&id) == nil {
					ids = append(ids, id)
				}
			}
			rows.Close()
			for _, id := range ids {
				a.notify(ctx, u, id, "Supplier response requested", wi.ID, fmt.Sprintf("%s requested a supplier response on %s %s", u.DisplayName, wi.ID, wi.Title))
			}
		}
	} else if u.IsVendor() {
		if wi.AssigneeID != nil {
			a.notify(ctx, u, *wi.AssigneeID, "Supplier responded", wi.ID, fmt.Sprintf("%s responded on %s %s", u.DisplayName, wi.ID, wi.Title))
		}
		a.notifyWatchers(ctx, u, wi.ID, "Supplier responded", fmt.Sprintf("%s responded on %s %s", u.DisplayName, wi.ID, wi.Title))
	}
	a.Events.Publish(Event{Type: "record", ID: wi.ID, Space: wi.Space})
	return ok(w, nil)
}

// ---------------------------------------------------------------------------
// Account expiry

// expiryNotices notifies user managers 14 days and 1 day before an account
// expires. Expired accounts are refused at sign-in and on every request.
func (a *App) expiryNotices(ctx context.Context) {
	rows, err := a.DB.Query(ctx, `SELECT id::text, display_name, expires_on::text, expiry_notices,
		(expires_on - current_date) FROM users WHERE active AND expires_on IS NOT NULL AND expires_on > current_date AND expires_on - current_date <= 14`)
	if err != nil {
		return
	}
	type due struct {
		id, name, on string
		notices      []string
		days         int
	}
	var list []due
	for rows.Next() {
		var d due
		if rows.Scan(&d.id, &d.name, &d.on, &d.notices, &d.days) == nil {
			list = append(list, d)
		}
	}
	rows.Close()
	if len(list) == 0 {
		return
	}
	managers := a.usersWith(ctx, PermUsers)
	for _, d := range list {
		key := d.on + "/14"
		if d.days <= 1 {
			key = d.on + "/1"
		}
		if contains(d.notices, key) {
			continue
		}
		for _, m := range managers {
			a.notify(ctx, nil, m, "Account expiring", "", fmt.Sprintf("The account of %s expires on %s.", d.name, d.on))
		}
		if d.days <= 1 {
			a.notify(ctx, nil, d.id, "Account expiring", "", fmt.Sprintf("Your DesignHub access ends on %s.", d.on))
		}
		a.DB.Exec(ctx, `UPDATE users SET expiry_notices = array_append(expiry_notices, $2) WHERE id = $1`, d.id, key)
	}
}

var errExpired = &Error{Status: http.StatusForbidden, Code: "account_expired", Message: "This account has expired. Contact your DesignHub administrator."}

// accountExpired reports an account past its expiry date or linked to an
// inactive company.
func (a *App) accountExpired(ctx context.Context, id string) bool {
	var expired bool
	a.DB.QueryRow(ctx, `SELECT (us.expires_on IS NOT NULL AND us.expires_on <= current_date) OR COALESCE(NOT c.active, false)
		FROM users us LEFT JOIN companies c ON c.id = us.company_id WHERE us.id = $1`, id).Scan(&expired)
	return expired
}

// usersWith returns the active users whose library role has a permission.
func (a *App) usersWith(ctx context.Context, perm string) []string {
	rows, err := a.DB.Query(ctx, `SELECT us.id::text FROM users us JOIN roles ro ON ro.id = us.library_role_id WHERE us.active AND $1 = ANY(ro.permissions)`, perm)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			out = append(out, id)
		}
	}
	return out
}
