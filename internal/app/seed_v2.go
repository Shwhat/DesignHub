package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// seedV2 loads the version 2 additions from TEST-PLAN-V2.md Appendix A.
func (s *seeder) seedV2(pw string) error {
	a, ctx := s.a, s.ctx
	super := s.user("superadmin")
	day := func(d int) string { return time.Now().AddDate(0, 0, d).Format("2006-01-02") }

	// A.1 Companies and users
	comp := func(name string, kinds []string, status string) string {
		id, err := a.CreateCompany(ctx, super, "seed", companyInput{Name: &name, Kinds: &kinds, SupplierStatus: &status})
		if err != nil {
			panic(fmt.Sprintf("company %s: %v", name, err))
		}
		return id
	}
	c1 := comp("Northfield Fabrication", []string{"Vendor"}, "Approved")
	c2 := comp("Ridgeway Electrical", []string{"Vendor", "Contractor"}, "Conditional")
	for cid, items := range map[string][]string{c1: {"100001-001", "100003-201"}, c2: {"100002-001"}} {
		c, err := a.companyForUser(ctx, super, cid)
		if err != nil {
			return err
		}
		if err := a.setSupplied(ctx, super, "seed", c, items, nil); err != nil {
			return err
		}
	}
	type vu struct {
		name, display, role, company string
		days                         int
	}
	for _, x := range []vu{
		{"vendor1", "Vendor One", RoleVendor, c1, 90},
		{"vendor2", "Vendor Two", RoleVendor, c2, 90},
		{"contractor1", "Contractor One", RoleContractor, c2, 30},
	} {
		exp := day(x.days)
		company := x.company
		if _, err := a.CreateUser(ctx, nil, "seed", userInput{Username: x.name, DisplayName: x.display, Email: x.name + "@designhub.test",
			Password: pw, LibraryRole: x.role, Company: &company, ExpiresOn: &exp}); err != nil {
			return fmt.Errorf("user %s: %w", x.name, err)
		}
		s.users[x.name] = nil
	}
	s.reloadUsers()
	r := a.roleByName(RoleContractor)
	if _, err := a.DB.Exec(ctx, `INSERT INTO space_members (space_id, user_id, role_id) VALUES ('SP-0001', $1, $2)`, s.user("contractor1").ID, r.ID); err != nil {
		return err
	}
	s.reloadUsers()

	// A.2 Planning and time. Numbers start at 0101 so they read apart from
	// the version 1 seed items.
	if _, err := a.DB.Exec(ctx, `UPDATE counters SET next = GREATEST(next, 101) WHERE name IN ('TSK', 'NCR', 'EPC')`); err != nil {
		return err
	}
	ed1, ed2, c1u := s.user("editor1"), s.user("editor2"), s.user("contrib1")
	work := func(by *User, in workInput) string {
		id, err := a.CreateWork(ctx, by, "seed", in)
		if err != nil {
			panic(fmt.Sprintf("work %s: %v", deref(in.Title), err))
		}
		return id
	}
	sprint := func(name, start, end string) string {
		var id string
		s.check(a.tx(ctx, func(tx pgx.Tx) error {
			var err error
			if id, err = a.nextPrefixed(ctx, tx, "SPR", "sprint", ed1.ID); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO sprints (id, space_id, name, goal, start_date, end_date, created_by) VALUES ($1, 'SP-0001', $2, $3, $4, $5, $6)`,
				id, name, "Seed sprint", start, end, ed1.ID); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO records (id, kind, title, status, space_id, origin) VALUES ($1, 'sprint', $2, 'Planned', 'SP-0001', 'db')`, id, name)
			return err
		}))
		return id
	}
	spr1 := sprint("Sprint 1", day(-7), day(7))
	spr2 := sprint("Sprint 2", day(8), day(21))
	milestone := func(name string, target string) string {
		var id string
		s.check(a.tx(ctx, func(tx pgx.Tx) error {
			var err error
			if id, err = a.nextPrefixed(ctx, tx, "MS", "milestone", ed1.ID); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO milestones (id, space_id, name, target_date, owner_id, created_by) VALUES ($1, 'SP-0001', $2, $3, $4, $4)`,
				id, name, target, ed1.ID); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO records (id, kind, title, status, space_id, owner_id, origin) VALUES ($1, 'milestone', $2, 'On track', 'SP-0001', $3, 'db')`, id, name, ed1.ID)
			return err
		}))
		return id
	}
	ms1 := milestone("Panel ready for wiring", day(30))
	ms2 := milestone("Enclosure delivered", day(5))
	epic := work(ed1, workInput{Type: "epic", Space: "SP-0001", Title: strPtr("Panel wiring"), AssigneeID: &ed1.ID})
	est := func(h float64) json.RawMessage { b, _ := json.Marshal(h); return b }
	t1 := work(ed1, workInput{Type: "task", Space: "SP-0001", Title: strPtr("Route wire duct"), AssigneeID: &c1u.ID, Epic: &epic,
		Estimate: est(4), Sprint: &spr1, Milestone: &ms1})
	t2 := work(ed1, workInput{Type: "task", Space: "SP-0001", Title: strPtr("Land field wiring"), AssigneeID: &ed1.ID, Epic: &epic,
		Estimate: est(4), Sprint: &spr1, Milestone: &ms1, DueDate: strPtr(day(10))})
	t3 := work(ed1, workInput{Type: "task", Space: "SP-0001", Title: strPtr("Label terminals"), AssigneeID: &ed2.ID, Epic: &epic,
		Estimate: est(2), Sprint: &spr1, Milestone: &ms2, DueDate: strPtr(day(5))})
	wi, err := a.loadWork(ctx, a.DB, t1)
	if err != nil {
		return err
	}
	s.check(a.setWorkStatus(ctx, c1u, "seed", wi, "Done"))
	if _, err := a.DB.Exec(ctx, `INSERT INTO links (src, type, dst, origin) VALUES ($1, 'blocks', $2, 'db')`, t1, t2); err != nil {
		return err
	}
	committed, _ := json.Marshal(map[string]any{"items": []string{t1, t2, t3}, "estimate": 10, "at": time.Now().AddDate(0, 0, -7).UTC()})
	if _, err := a.DB.Exec(ctx, `UPDATE sprints SET state = 'Active', committed = $2 WHERE id = $1`, spr1, committed); err != nil {
		return err
	}
	a.DB.Exec(ctx, `UPDATE records SET status = 'Active' WHERE id = $1`, spr1)
	_ = spr2
	for _, x := range []struct {
		user *User
		item string
		d    int
		h    float64
	}{{c1u, t1, -2, 3}, {ed1, t2, -1, 2}, {ed2, t3, -1, 1}} {
		if _, err := a.DB.Exec(ctx, `INSERT INTO time_entries (item_id, user_id, day, hours, note) VALUES ($1, $2, $3, $4, 'Seed entry')`, x.item, x.user.ID, day(x.d), x.h); err != nil {
			return err
		}
	}
	work(c1u, workInput{Type: "ncr", Space: "SP-0001", Title: strPtr("Bracket bend angle out of tolerance"), AssigneeID: &ed1.ID,
		Description: strPtr("Bend angle measured at 92 degrees; drawing calls for 90 plus or minus 1."),
		Fields: map[string]any{"found_on": "100001-001", "quantity": 12.0, "where_found": "Receiving inspection", "severity": "Major",
			"requires_design_change": false, "supplier": c1}})
	work(c1u, workInput{Type: "ncr", Space: "SP-0001", Title: strPtr("Back plate finish scratched"), AssigneeID: &ed1.ID,
		Description: strPtr("Powder coat scratched on 3 of 10 back plates."),
		Fields: map[string]any{"found_on": "100002-001", "quantity": 3.0, "where_found": "Receiving inspection", "severity": "Minor",
			"requires_design_change": false, "supplier": c2}})

	// A.3 Sample import files, offered in Admin, Imports.
	samples := []struct{ name, content string }{
		{"parts-good.csv", "ID,Kind,Name,Description,Unit,Tracking,Home space,Owner\n,Production,Cable gland plate,Gland plate for the bottom entry,Each,Lot,SP-0001,editor1\n" +
			",Production,Duct cover,,Each,Lot,SP-0001,editor1\n100020-001,Production,Shelf bracket,,Each,Lot,SP-0001,editor1\n" +
			"100021-001,Production,Shelf,,Each,Lot,SP-0001,editor1\n100022-001,Production,Shelf stop,,Each,Lot,SP-0001,editor1\n"},
		{"parts-bad.csv", "ID,Kind,Name,Unit,Home space,Manufacturer,Order number\n100001-001,Production,Duplicate bracket,Each,SP-0001,,\n,Production,,Each,SP-0001,,\n" +
			",Production,Odd unit part,Bushel,SP-0001,,\n,COTS,Old relay,Each,SP-0001,ZZZ,123\n"},
		{"bom-lines.csv", "Find,Item,Quantity,Reference designators,Notes\n10,100001-001,2,,\n20,100003-201,1,,\n30,PXC.3031212,24,X1 X2,\n40,100005-001,1,,\n"},
		{"work-items.csv", "Type,Title,Description,Priority,Assignee,Due date,Estimate,Area,Severity\ntask,Install cable tray,,Normal,editor1,,3,,\n" +
			"task,Pull feeder cables,,High,editor2,,6,,\ntask,Terminate feeders,,Normal,editor1,,4,,\nissue,Tray clashes with sprinkler pipe,,High,editor1,,,Factory,Major\n"},
	}
	var list []map[string]string
	for _, f := range samples {
		h, _, err := a.Files.Put(bytes.NewReader([]byte(f.content)), 1<<20)
		if err != nil {
			return err
		}
		if _, err := a.DB.Exec(ctx, `INSERT INTO files (sha256, size, mime) VALUES ($1, $2, 'text/csv') ON CONFLICT DO NOTHING`, h, len(f.content)); err != nil {
			return err
		}
		list = append(list, map[string]string{"name": f.name, "sha256": h})
	}
	return a.SetSetting(ctx, super, "seed", "import_samples", list)
}
