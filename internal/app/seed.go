package app

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"designhub/internal/model"
	"designhub/internal/repo"
)

// LoadSeed loads the test data set from TEST-PLAN.md Appendix A. It refuses
// to run when engineering data already exists.
func (a *App) LoadSeed(ctx context.Context, password string) error {
	var n int
	if err := a.DB.QueryRow(ctx, `SELECT count(*) FROM identifiers WHERE kind NOT IN ('space', 'page')`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("this installation already has data; seed data can only be loaded into a new installation")
	}
	s := &seeder{a: a, ctx: ctx, users: map[string]*User{}}
	return s.run(password)
}

type seeder struct {
	a     *App
	ctx   context.Context
	users map[string]*User
	err   error
}

func (s *seeder) check(err error) {
	if err != nil && s.err == nil {
		s.err = err
	}
}

func (s *seeder) user(name string) *User {
	u := s.users[name]
	if u == nil {
		panic("unknown seed user " + name)
	}
	return u
}

func (s *seeder) reloadUsers() {
	for name := range s.users {
		var id string
		s.a.DB.QueryRow(s.ctx, `SELECT id::text FROM users WHERE lower(username) = $1`, name).Scan(&id)
		u, err := s.a.loadUser(s.ctx, id)
		s.check(err)
		if u != nil {
			s.users[name] = u
		}
	}
}

func (s *seeder) run(pw string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("seed: %v", r)
		}
	}()
	a, ctx := s.a, s.ctx
	// Users
	type su struct{ name, display, role string }
	for _, x := range []su{
		{"superadmin", "Super Admin", RoleSuperAdmin}, {"admin1", "Admin One", RoleAdmin}, {"ra1", "Release Authority One", RoleRelease},
		{"appr1", "Approver One", RoleApprover}, {"appr2", "Approver Two", RoleApprover}, {"editor1", "Editor One", RoleEditor},
		{"editor2", "Editor Two", RoleEditor}, {"contrib1", "Contributor One", RoleContrib}, {"viewer1", "Viewer One", RoleViewer},
	} {
		exists, _, err := a.UserRole(ctx, x.name)
		if err != nil {
			return err
		}
		if !exists {
			if _, err := a.CreateUser(ctx, nil, "seed", userInput{Username: x.name, DisplayName: x.display, Email: x.name + "@designhub.test", Password: pw, LibraryRole: x.role}); err != nil {
				return fmt.Errorf("user %s: %w", x.name, err)
			}
		}
		s.users[x.name] = nil
	}
	s.reloadUsers()
	if s.err != nil {
		return s.err
	}
	super := s.user("superadmin")
	// Spaces: SP-0001 exists when setup-admin ran first.
	var sp1 string
	a.DB.QueryRow(ctx, `SELECT id FROM spaces ORDER BY id LIMIT 1`).Scan(&sp1)
	if sp1 == "" {
		if sp1, err = a.CreateSpace(ctx, super, "seed", "Factory Buildout", "Factory build-out test space"); err != nil {
			return err
		}
	}
	if sp1 != "SP-0001" {
		return fmt.Errorf("expected the first space to be SP-0001, found %s", sp1)
	}
	a.DB.Exec(ctx, `UPDATE spaces SET name = 'Factory Buildout', description = 'Factory build-out test space' WHERE id = 'SP-0001'`)
	member := func(space, user, role string) {
		r := a.roleByName(role)
		_, err := a.DB.Exec(ctx, `INSERT INTO space_members (space_id, user_id, role_id) VALUES ($1, $2, $3) ON CONFLICT (space_id, user_id) DO UPDATE SET role_id = $3`,
			space, s.user(user).ID, r.ID)
		s.check(err)
	}
	for user, role := range map[string]string{"superadmin": RoleAdmin, "admin1": RoleAdmin, "ra1": RoleViewer, "appr1": RoleViewer, "appr2": RoleViewer,
		"editor1": RoleEditor, "editor2": RoleEditor, "contrib1": RoleContrib, "viewer1": RoleViewer} {
		member("SP-0001", user, role)
	}
	s.reloadUsers()
	ed1 := s.user("editor1")

	// Files
	drawing := s.file("drawing.pdf", seedPDF("Drawing"), "application/pdf")
	datasheet := s.file("datasheet.pdf", seedPDF("Datasheet"), "application/pdf")
	standard := s.file("electrical-design-standard.pdf", seedPDF("Electrical design standard"), "application/pdf")
	withFile := func(f model.File, role string) []model.File { f.Role = role; return []model.File{f} }

	part := func(number, title, kind string, files []model.File, mutate func(*model.Item)) *model.Item {
		it := model.Item{Type: model.TypePart, Kind: kind, Title: title, Owner: "editor1", Space: "SP-0001", Unit: "Each", Tracking: "Lot", Files: files}
		in := createInput{Item: it, Number: number}
		if kind == model.KindCOTS {
			mfr, order, _ := model.ParseCOTS(number)
			in.Item.Manufacturer, in.OrderNumber, in.Item.Lifecycle = mfr, order, "Active"
			in.Number = ""
		}
		if mutate != nil {
			mutate(&in.Item)
		}
		created, err := a.CreateItem(ctx, ed1, "seed", in)
		if err != nil {
			panic(fmt.Sprintf("create %s: %v", number, err))
		}
		if created.ID != number {
			panic(fmt.Sprintf("expected %s, got %s", number, created.ID))
		}
		return created
	}
	part("100001-001", "Bracket, mounting", model.KindProduction, withFile(drawing, "Drawing"), nil)
	part("100001-002", "Bracket, mounting, left", model.KindProduction, nil, nil)
	part("100002-001", "Back plate", model.KindProduction, withFile(drawing, "Drawing"), nil)
	part("PXC.3031212", "Terminal block", model.KindCOTS, withFile(datasheet, "Datasheet"), nil)
	part("SIE.3RT2015-1BB41", "Contactor", model.KindCOTS, nil, nil)
	part("RIT.8606000", "Enclosure", model.KindCOTS, nil, nil)
	part("100003-201", "DIN rail subassembly", model.KindProduction, withFile(drawing, "Drawing"), func(it *model.Item) {
		it.BOM = []model.BOMLine{{Find: 10, Item: "PXC.3031212", Qty: 10}, {Find: 20, Item: "SIE.3RT2015-1BB41", Qty: 2, RefDes: []string{"K1", "K2"}}, {Find: 30, Item: "100001-001", Qty: 2}}
	})
	part("100005-001", "Door cover", model.KindProduction, nil, nil)
	part("100006-901", "Mounting interface", model.KindProduction, nil, nil)
	part("100007-001", "Shim", model.KindProduction, withFile(drawing, "Drawing"), nil)
	part("100004-201", "Control panel", model.KindProduction, nil, func(it *model.Item) {
		it.BOM = []model.BOMLine{{Find: 10, Item: "RIT.8606000", Qty: 1}, {Find: 20, Item: "100002-001", Qty: 1}, {Find: 30, Item: "100003-201", Qty: 3},
			{Find: 40, Item: "100001-001", Qty: 4}, {Find: 50, Item: "100005-001", Qty: 1}, {Find: 60, Item: "100007-001", Qty: 2}}
	})

	// Requirements, procedure and documents
	create := func(it model.Item) string {
		it.Owner, it.Space = "editor1", "SP-0001"
		c, err := a.CreateItem(ctx, ed1, "seed", createInput{Item: it})
		if err != nil {
			panic(fmt.Sprintf("create %s: %v", it.Title, err))
		}
		return c.ID
	}
	create(model.Item{Type: model.TypeRequirement, Title: "Control voltage", Statement: "The control panel shall operate from a 24 VDC control supply.",
		Category: "Functional", Priority: "Must", Verification: []string{"Test"}})
	create(model.Item{Type: model.TypeRequirement, Title: "Enclosure protection", Statement: "The enclosure shall provide IP54 protection.",
		Category: "Environmental", Priority: "Must", Verification: []string{"Inspection"}})
	create(model.Item{Type: model.TypeRequirement, Title: "Contactor coil voltage", Statement: "Contactor coils shall operate at 24 VDC.",
		Category: "Functional", Priority: "Should", Verification: []string{"Test"}, Links: []model.Link{{Type: "derived from", To: "REQ-0001"}}})
	create(model.Item{Type: model.TypeProcedure, Title: "Panel functional test", ProcedureType: "Test", Purpose: "Verify the control panel operates from 24 VDC.",
		Steps: []model.Step{{Instruction: "Connect a 24 VDC supply to the control terminals.", Acceptance: "Supply reads 24 V plus or minus 5 percent"},
			{Instruction: "Energize K1 and K2 from the test switch.", Acceptance: "Both contactors pull in"}, {Instruction: "Remove the supply.", Acceptance: "Both contactors drop out"}},
		Links: []model.Link{{Type: "verifies", To: "REQ-0001"}, {Type: "applies to", To: "100004-201"}}})
	create(model.Item{Type: model.TypeDocument, Title: "Electrical design standard", DocumentType: "Standard", Files: withFile(standard, "Other")})
	create(model.Item{Type: model.TypeDocument, Title: "Pricing agreement", DocumentType: "Agreement"})
	s.setLinks("RIT.8606000", []model.Link{{Type: "satisfies", To: "REQ-0002"}})
	s.setLinks("100004-201", []model.Link{{Type: "satisfies", To: "REQ-0001"}})

	// Pages in SP-0001
	page := func(title, parent, typ, controlled, body string) string {
		id := s.createPage("SP-0001", parent, title, typ, controlled, body)
		return id
	}
	pg2 := page("Control panel overview", "", "General", "no",
		"Overview of the control panel build.\n\n```bom 100004-201\n```\n\nOpen nonconformances on the DIN rail subassembly:\n\n```work type=ncr item=100003-201\n```\n")
	pg3 := page("Kickoff meeting notes", pg2, "Meeting notes", "no", "Attendees: Editor One, Editor Two.\n\nAgreed the panel layout.\n")
	s.editPage(pg3, "Attendees: Editor One, Editor Two, Contributor One.\n\nAgreed the panel layout and the enclosure size.\n", "Add attendee and decision")
	pg4 := page("Panel build standard", "", "General", "yes", "All panels are built to the electrical design standard DOC-0001.\n")
	a.DB.Exec(ctx, `UPDATE spaces SET pinned_links = '[{"title": "Control panel overview", "url": "PG-0002"}]' WHERE id = 'SP-0001'`)
	if e := cache.get(pg4); e != nil {
		upd := e.Item.Clone()
		upd.ReleasedOnly = true
		s.check(s.save(pg4, upd, "Show released revision only"))
	}

	// Space SP-0002 and its page
	sp2, err := a.CreateSpace(ctx, super, "seed", "Product Design", "Product design test space")
	if err != nil {
		return err
	}
	for user, role := range map[string]string{"superadmin": RoleAdmin, "admin1": RoleAdmin, "editor1": RoleEditor} {
		member(sp2, user, role)
	}
	s.reloadUsers()
	s.createPage(sp2, "", "Concept notes", "General", "no", "Early concepts for the product.\n")
	ed1 = s.user("editor1")
	part("1000001", "Keep-out zone, cable entry", model.KindReference, nil, func(it *model.Item) { it.Space = sp2 })

	// Locks
	for _, rec := range []string{"100007-001", "DOC-0002"} {
		for _, user := range []string{"admin1", "editor1"} {
			_, err := a.DB.Exec(ctx, `INSERT INTO record_access (record_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, rec, s.user(user).ID)
			s.check(err)
		}
	}

	// CO-0001: initial release
	appr1, appr2, ra1 := s.user("appr1"), s.user("appr2"), s.user("ra1")
	title := "Initial release, panel components"
	ct := "Initial release"
	reason := "Release the components ordered for the first panel."
	desc := "Initial release of the bracket, back plate, DIN rail subassembly, shim, purchased parts, requirements, test procedure, standard and build standard."
	co1, err := a.CreateCO(ctx, ed1, "seed", coInput{Title: &title, ChangeType: &ct, Reason: &reason, Description: &desc, HomeSpace: strPtr("SP-0001")})
	if err != nil {
		return err
	}
	for _, id := range []string{"100001-001", "100002-001", "PXC.3031212", "SIE.3RT2015-1BB41", "RIT.8606000", "100003-201", "100007-001", "REQ-0001", "REQ-0002", "PRC-0001", "DOC-0001", pg4} {
		if err := a.AddCOItem(ctx, ed1, "seed", co1, id, "Release"); err != nil {
			return fmt.Errorf("CO-0001 %s: %w", id, err)
		}
	}
	s.check(a.setApprovers(ctx, ed1, "seed", co1, []string{appr1.ID, appr2.ID}))
	s.submit(co1, ed1)
	s.approve(co1, appr1)
	s.approve(co1, appr2)
	if s.err != nil {
		return s.err
	}
	if err := a.ReleaseCO(ctx, ra1, "seed", co1, ""); err != nil {
		return fmt.Errorf("release CO-0001: %w", err)
	}

	// CO-0002: back plate revision 02 in review
	e := cache.get("100002-001")
	if err := a.StartRevision(ctx, ed1, "seed", e); err != nil {
		return err
	}
	title2, ct2 := "Back plate hole pattern", "Design change"
	reason2 := "Mounting holes do not match the DIN rail spacing."
	desc2 := "Move the DIN rail mounting holes to 125 mm centers."
	co2, err := a.CreateCO(ctx, ed1, "seed", coInput{Title: &title2, ChangeType: &ct2, Reason: &reason2, Description: &desc2, HomeSpace: strPtr("SP-0001")})
	if err != nil {
		return err
	}
	if err := a.AddCOItem(ctx, ed1, "seed", co2, "100002-001", "Revise"); err != nil {
		return err
	}
	e = cache.get("100002-001")
	b, h, err := a.Repo.Read("co/"+co2, a.draftPath(e))
	if err != nil {
		return err
	}
	draft, err := model.Parse(b)
	if err != nil {
		return err
	}
	draft.Description = "Back plate with DIN rail mounting holes on 125 mm centers."
	if _, err := a.SaveItem(ctx, ed1, "seed", e, draft, h, "Move mounting holes"); err != nil {
		return fmt.Errorf("edit CO-0002: %w", err)
	}
	s.check(a.setApprovers(ctx, ed1, "seed", co2, []string{appr1.ID, appr2.ID}))
	s.submit(co2, ed1)
	s.approve(co2, appr1)

	// CO-0003: empty draft owned by editor2
	title3, ct3 := "Door cover release", "Initial release"
	if _, err := a.CreateCO(ctx, s.user("editor2"), "seed", coInput{Title: &title3, ChangeType: &ct3, HomeSpace: strPtr("SP-0001")}); err != nil {
		return err
	}

	// Work items
	day := func(d int) *string { v := time.Now().AddDate(0, 0, d).Format("2006-01-02"); return &v }
	work := func(by string, in workInput) string {
		id, err := a.CreateWork(ctx, s.user(by), "seed", in)
		if err != nil {
			panic(fmt.Sprintf("work %s: %v", deref(in.Title), err))
		}
		return id
	}
	ed1ID, ed2ID, c1ID := s.user("editor1").ID, s.user("editor2").ID, s.user("contrib1").ID
	epic := work("editor1", workInput{Type: "epic", Space: "SP-0001", Title: strPtr("Control panel build"), AssigneeID: &ed1ID})
	work("editor1", workInput{Type: "task", Space: "SP-0001", Title: strPtr("Order enclosure"), AssigneeID: &ed1ID, DueDate: day(-3), Epic: &epic,
		Links: []model.Link{{Type: "affects", To: "RIT.8606000"}}})
	t2 := work("editor1", workInput{Type: "task", Space: "SP-0001", Title: strPtr("Mount DIN rail subassemblies"), AssigneeID: &ed1ID, DueDate: day(7), Epic: &epic})
	t3 := work("editor2", workInput{Type: "task", Space: "SP-0001", Title: strPtr("Approve panel layout"), AssigneeID: &ed2ID, DueDate: day(-10), Epic: &epic})
	work("editor1", workInput{Type: "task", Space: sp2, Title: strPtr("Concept review"), AssigneeID: &ed1ID, DueDate: day(14)})
	_, err = a.DB.Exec(ctx, `INSERT INTO links (src, type, dst, origin) VALUES ('TSK-0001', 'blocks', $1, 'db')`, t2)
	s.check(err)
	wi, err := a.loadWork(ctx, a.DB, t3)
	if err != nil {
		return err
	}
	s.check(a.setWorkStatus(ctx, s.user("editor2"), "seed", wi, "Done"))
	work("editor2", workInput{Type: "issue", Space: "SP-0001", Title: strPtr("Terminal blocks too close to wire duct"), AssigneeID: &ed2ID,
		Fields: map[string]any{"area": "Design", "severity": "Major"}, Links: []model.Link{{Type: "affects", To: "100003-201"}}})
	work("contrib1", workInput{Type: "action", Space: "SP-0001", Title: strPtr("Send meeting minutes"), AssigneeID: &c1ID, DueDate: day(1),
		Fields: map[string]any{"source": pg3}})
	work("contrib1", workInput{Type: "ncr", Space: "SP-0001", Title: strPtr("Contactor coil damaged"), AssigneeID: &c1ID,
		Description: strPtr("Two contactors arrived with cracked coil housings."),
		Fields: map[string]any{"found_on": "100003-201", "quantity": 2.0, "where_found": "Receiving inspection", "manufacturer": "SIE", "severity": "Major", "requires_design_change": false}})
	ncr2 := work("editor1", workInput{Type: "ncr", Space: "SP-0001", Title: strPtr("Back plate holes misaligned"), AssigneeID: &ed1ID,
		Description: strPtr("Mounting holes are 3 mm off the DIN rail spacing."),
		Fields: map[string]any{"found_on": "100002-001", "quantity": 1.0, "where_found": "In process", "severity": "Major", "requires_design_change": true,
			"disposition": "Rework", "disposition_approvers": []any{appr1.ID}}})
	_, err = a.DB.Exec(ctx, `UPDATE ncr_approvals SET decision = 'Approved', decided_at = now(), comment = 'Rework holes' WHERE ncr_id = $1`, ncr2)
	s.check(err)
	wi, err = a.loadWork(ctx, a.DB, ncr2)
	if err != nil {
		return err
	}
	s.check(a.setWorkStatus(ctx, appr1, "seed", wi, "Dispositioned"))

	// Comment with a mention
	var cid string
	s.check(a.DB.QueryRow(ctx, `INSERT INTO comments (record_id, author_id, body) VALUES ($1, $2, $3) RETURNING id::text`,
		pg2, s.user("editor2").ID, "@editor1 please check the enclosure size before ordering.").Scan(&cid))

	// Notifications at load (Appendix A.10)
	a.DB.Exec(ctx, `DELETE FROM notifications`)
	s.check(a.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO notifications (recipient_id, event, record_id, triggered_by, summary) VALUES ($1, 'Mentioned', $2, $3, $4)`,
			ed1ID, pg2, ed2ID, "Editor Two mentioned you on "+pg2+" Control panel overview"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO notifications (recipient_id, event, record_id, triggered_by, summary) VALUES ($1, 'Approval requested', $2, $3, $4)`,
			appr2.ID, co2, ed1ID, "Editor One asks for your approval of "+co2+" "+title2)
		return err
	}))
	if s.err != nil {
		return s.err
	}
	a.Audit(ctx, nil, "seed", "seed.load", "", nil)
	return a.RebuildIndex(ctx)
}

func strPtr(s string) *string { return &s }

func (s *seeder) file(name string, b []byte, mime string) model.File {
	sum, size, err := s.a.Files.Put(bytes.NewReader(b), maxUpload)
	s.check(err)
	_, err = s.a.DB.Exec(s.ctx, `INSERT INTO files (sha256, size, mime) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, sum, size, mime)
	s.check(err)
	return model.File{SHA256: sum, Name: name, Size: size, Mime: mime}
}

func (s *seeder) save(id string, it *model.Item, comment string) error {
	e := cache.get(id)
	if e == nil {
		return fmt.Errorf("%s missing", id)
	}
	u := s.user("editor1")
	co := s.a.openCOFor(s.ctx, s.a.DB, id)
	var perms itemPerms
	if e.Item.Type == model.TypePage {
		perms = s.a.pagePerms(s.ctx, u, e, co).itemPerms
	} else {
		perms = s.a.itemPerms(u, e, co)
	}
	_, err := s.a.writeRecord(s.ctx, u, "seed", e, co, perms, it, e.Hash, comment, "seed.save")
	return err
}

func (s *seeder) setLinks(id string, links []model.Link) {
	e := cache.get(id)
	upd := e.Item.Clone()
	upd.Links = append(upd.Links, links...)
	s.check(s.save(id, upd, "Add links"))
}

func (s *seeder) createPage(space, parent, title, typ, controlled, body string) string {
	a, ctx := s.a, s.ctx
	u := s.user("editor1")
	maxSort := 0
	for _, p := range a.pagesIn(space) {
		if p.Parent == parent && p.Sort > maxSort {
			maxSort = p.Sort
		}
	}
	p := &model.Item{Type: model.TypePage, Title: title, Owner: u.Username, CreatedBy: u.Username, Space: space, Parent: parent,
		Sort: maxSort + 10, PageType: typ, Controlled: controlled, Body: body}
	if a.isControlled(ctx, p) {
		p.Status = model.StatusDraft
	}
	var id string
	err := a.tx(ctx, func(tx pgx.Tx) error {
		var err error
		if id, err = a.nextPrefixed(ctx, tx, "PG", "page", u.ID); err != nil {
			return err
		}
		p.ID = id
		b, err := p.Marshal()
		if err != nil {
			return err
		}
		_, err = a.Repo.Commit("main", []repo.Change{{Path: model.Path(p), Content: b, Expect: "new"}}, author(u), "Create page "+id+" "+title)
		return err
	})
	if err != nil {
		panic(fmt.Sprintf("page %s: %v", title, err))
	}
	s.check(a.refresh(ctx, id, model.TypePage, space))
	return id
}

func (s *seeder) editPage(id, body, comment string) {
	e := cache.get(id)
	upd := e.Item.Clone()
	upd.Body = body
	s.check(s.save(id, upd, comment))
}

func (s *seeder) submit(co string, u *User) {
	a, ctx := s.a, s.ctx
	s.check(a.tx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE change_orders SET state = 'In Review' WHERE id = $1`, co)
		return err
	}))
	items, _ := a.coItems(ctx, co)
	a.afterCOStateChange(ctx, co, items)
}

func (s *seeder) approve(co string, u *User) {
	a, ctx := s.a, s.ctx
	s.check(a.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE co_approvers SET decision = 'Approved', decided_at = now(), comment = 'Approved' WHERE co_id = $1 AND user_id = $2`, co, u.ID); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, "seed", "co.decision", co, map[string]any{"decision": "Approved", "signature": "seed"})
		return a.settleCO(ctx, tx, u, "seed", co)
	}))
}

// seedPDF builds a one-page PDF with a title, for test attachments.
func seedPDF(title string) []byte {
	title = strings.NewReplacer("(", "", ")", "", "\\", "").Replace(title)
	content := fmt.Sprintf("BT /F1 24 Tf 72 700 Td (%s) Tj ET\nBT /F1 12 Tf 72 670 Td (DesignHub test file) Tj ET\n", title)
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objs))
	for i, o := range objs {
		offsets[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objs)+1, xref)
	return b.Bytes()
}
