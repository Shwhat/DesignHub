package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"designhub/internal/model"
	"designhub/internal/repo"
)

// ---------------------------------------------------------------------------
// Library listing

type libraryRow struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	PartKind  string    `json:"partKind,omitempty"`
	Title     string    `json:"title"`
	Revision  string    `json:"revision"`
	Status    string    `json:"status"`
	Owner     string    `json:"owner"`
	Space     string    `json:"space"`
	Labels    []string  `json:"labels"`
	Extra     string    `json:"extra,omitempty"` // category, procedure type or document type
	Updated   time.Time `json:"updated"`
	Draft     string    `json:"draft,omitempty"` // revision being drafted
	Locked    bool      `json:"locked,omitempty"`
}

func (a *App) apiLibrary(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermLibView) {
		return errForbidden
	}
	ctx := r.Context()
	kind := r.URL.Query().Get("kind") // part, assembly, requirement, procedure, document
	includeOld := r.URL.Query().Get("all") == "1"
	locks, err := a.locks(ctx)
	if err != nil {
		return err
	}
	status := map[string]string{}
	updated := map[string]time.Time{}
	rows, err := a.DB.Query(ctx, `SELECT id, status, updated_at FROM records WHERE origin = 'git'`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, s string
		var t time.Time
		if err := rows.Scan(&id, &s, &t); err != nil {
			rows.Close()
			return err
		}
		status[id], updated[id] = s, t
	}
	rows.Close()
	out := []libraryRow{}
	add := func(e *entry, it *model.Item, st string) {
		row := libraryRow{ID: it.ID, Kind: kindOf(it), Title: it.Title, Revision: it.Revision, Status: st, Owner: it.Owner,
			Space: it.Space, Labels: nz(it.Labels), Updated: updated[it.ID], Locked: len(locks[it.ID]) > 0}
		if it.Type == model.TypePart {
			row.PartKind = it.Kind
		}
		switch it.Type {
		case model.TypeRequirement:
			row.Extra = it.Category
		case model.TypeProcedure:
			row.Extra = it.ProcedureType
		case model.TypeDocument:
			row.Extra = it.DocumentType
		}
		if e != nil && e.Next != nil {
			row.Draft = e.Next.Revision
		}
		out = append(out, row)
	}
	cache.mu.RLock()
	entries := make([]*entry, 0, len(cache.items))
	for _, e := range cache.items {
		entries = append(entries, e)
	}
	cache.mu.RUnlock()
	for _, e := range entries {
		it := e.Item
		if !it.IsEngineering() || (kind != "" && kindOf(it) != kind) {
			continue
		}
		if !visible(u, kindOf(it), it.Space, locks[it.ID]) {
			continue
		}
		st := status[it.ID]
		if st == "" {
			st = it.Status
		}
		if !includeOld && st == model.StatusObsolete {
			continue
		}
		add(e, it, st)
	}
	if includeOld {
		// Superseded revisions come from the release history.
		rows, err := a.DB.Query(ctx, `SELECT item_id, revision FROM item_revisions WHERE status = 'Superseded'`)
		if err != nil {
			return err
		}
		type sup struct{ id, rev string }
		var sups []sup
		for rows.Next() {
			var s sup
			if err := rows.Scan(&s.id, &s.rev); err != nil {
				rows.Close()
				return err
			}
			sups = append(sups, s)
		}
		rows.Close()
		for _, s := range sups {
			e := cache.get(s.id)
			if e == nil || (kind != "" && kindOf(e.Item) != kind) || !visible(u, kindOf(e.Item), e.Item.Space, locks[s.id]) {
				continue
			}
			old, err := a.itemAtRevision(ctx, s.id, s.rev)
			if err != nil {
				continue
			}
			add(nil, old, model.StatusSuperseded)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Revision > out[j].Revision
	})
	return ok(w, out)
}

// itemAtRevision reads an item as it was released at a revision.
func (a *App) itemAtRevision(ctx context.Context, id, rev string) (*model.Item, error) {
	var commit string
	if err := a.DB.QueryRow(ctx, `SELECT commit_hash FROM item_revisions WHERE item_id = $1 AND revision = $2`, id, rev).Scan(&commit); err != nil {
		return nil, err
	}
	e := cache.get(id)
	if e == nil {
		return nil, errNotFound
	}
	b, _, err := a.Repo.Read(commit, e.Path)
	if err != nil {
		return nil, err
	}
	it, err := model.Parse(b)
	if err != nil {
		return nil, err
	}
	var st string
	a.DB.QueryRow(ctx, `SELECT status FROM item_revisions WHERE item_id = $1 AND revision = $2`, id, rev).Scan(&st)
	if st != "" {
		it.Status = st
	}
	return it, nil
}

// ---------------------------------------------------------------------------
// Create

type createInput struct {
	Item        model.Item `json:"item"`
	Number      string     `json:"number"`      // production or reference number chosen by the user
	OrderNumber string     `json:"orderNumber"` // COTS
	Comment     string     `json:"comment"`
}

func (a *App) apiNewItemOffer(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermLibEdit) {
		return errForbidden
	}
	ctx := r.Context()
	base, err := a.offerProduction(ctx, a.DB)
	if err != nil {
		return err
	}
	ref, err := a.offerReference(ctx, a.DB)
	if err != nil {
		return err
	}
	var peek = func(prefix string) string {
		var n int64
		a.DB.QueryRow(ctx, `SELECT next FROM counters WHERE name = $1`, prefix).Scan(&n)
		return model.FormatPrefixed(prefix, n)
	}
	return ok(w, map[string]string{
		"production": base + "-001", "reference": ref,
		"requirement": peek("REQ"), "procedure": peek("PRC"), "document": peek("DOC"),
	})
}

func (a *App) apiCreateItem(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermLibEdit) {
		return errForbidden
	}
	var in createInput
	if err := decode(r, &in); err != nil {
		return err
	}
	it, err := a.CreateItem(r.Context(), u, clientIP(r), in)
	if err != nil {
		return err
	}
	return ok(w, map[string]string{"id": it.ID})
}

// CreateItem assigns the number, writes the first version to main and
// indexes it.
func (a *App) CreateItem(ctx context.Context, u *User, ip string, in createInput) (*model.Item, error) {
	it := in.Item
	it.Status = model.StatusDraft
	it.Revision = ""
	it.CreatedBy = u.Username
	it.Links = append([]model.Link(nil), it.Links...)
	if it.Owner == "" {
		it.Owner = u.Username
	}
	if it.Space == "" {
		return nil, badRequest("Home space is required.")
	}
	var spaceOK bool
	a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM spaces WHERE id = $1 AND NOT archived)`, it.Space).Scan(&spaceOK)
	if !spaceOK {
		return nil, badRequest("Unknown home space.")
	}
	if it.Type == model.TypePart {
		if it.Unit == "" {
			it.Unit = "Each"
		}
		if it.Tracking == "" {
			it.Tracking = "Lot"
		}
	}
	err := a.tx(ctx, func(tx pgx.Tx) error {
		switch it.Type {
		case model.TypePart:
			switch it.Kind {
			case model.KindProduction:
				offered, err := a.offerProduction(ctx, tx)
				if err != nil {
					return err
				}
				num := strings.TrimSpace(in.Number)
				if num == "" {
					num = offered + "-001"
				}
				base, _, err := model.ParseProduction(num)
				if err != nil {
					return badRequest("%s.", capitalize(err.Error()))
				}
				if err := a.reserveID(ctx, tx, num, "part", u.ID); err != nil {
					return err
				}
				if base == offered {
					n, _ := strconv.ParseInt(base, 10, 64)
					if _, err := tx.Exec(ctx, `UPDATE counters SET next = $1 WHERE name = 'production' AND next <= $1`, n+1); err != nil {
						return err
					}
				}
				it.ID = num
			case model.KindReference:
				offered, err := a.offerReference(ctx, tx)
				if err != nil {
					return err
				}
				num := strings.TrimSpace(in.Number)
				if num == "" {
					num = offered
				}
				if model.PartKindOf(num) != model.KindReference {
					return badRequest("Reference numbers have 7 digits.")
				}
				if err := a.reserveID(ctx, tx, num, "part", u.ID); err != nil {
					return err
				}
				if num == offered {
					n, _ := strconv.ParseInt(num, 10, 64)
					if _, err := tx.Exec(ctx, `UPDATE counters SET next = $1 WHERE name = 'reference' AND next <= $1`, n+1); err != nil {
						return err
					}
				}
				it.ID = num
			case model.KindCOTS:
				var active bool
				if err := tx.QueryRow(ctx, `SELECT active FROM manufacturers WHERE code = $1`, it.Manufacturer).Scan(&active); err != nil || !active {
					return badRequest("Choose an active manufacturer.")
				}
				id, err := model.COTSNumber(it.Manufacturer, in.OrderNumber)
				if err != nil {
					return badRequest("%s.", capitalize(err.Error()))
				}
				if err := a.reserveID(ctx, tx, id, "part", u.ID); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `UPDATE manufacturers SET used = true WHERE code = $1`, it.Manufacturer); err != nil {
					return err
				}
				it.ID, it.OrderNumber = id, strings.TrimSpace(in.OrderNumber)
				if it.Lifecycle == "" {
					it.Lifecycle = "Active"
				}
			default:
				return badRequest("Choose a part kind: production, reference or COTS.")
			}
		case model.TypeRequirement, model.TypeProcedure, model.TypeDocument:
			prefix := map[string]string{model.TypeRequirement: "REQ", model.TypeProcedure: "PRC", model.TypeDocument: "DOC"}[it.Type]
			id, err := a.nextPrefixed(ctx, tx, prefix, it.Type, u.ID)
			if err != nil {
				return err
			}
			it.ID = id
		default:
			return badRequest("Choose a type: part, assembly, requirement, procedure or document.")
		}
		if issues := errorsOnly(model.Required(&it)); len(issues) > 0 {
			return &Error{Status: http.StatusBadRequest, Code: "invalid", Message: issues[0].Message + ".", Data: issues}
		}
		b, err := it.Marshal()
		if err != nil {
			return err
		}
		msg := fmt.Sprintf("Create %s %s", it.ID, it.Title)
		if c := strings.TrimSpace(in.Comment); c != "" {
			msg += "\n\n" + c
		}
		if _, err := a.Repo.Commit(repo.Main, []repo.Change{{Path: model.Path(&it), Content: b, Expect: "new"}}, author(u), msg); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, ip, "item.create", it.ID, map[string]any{"title": it.Title})
		_, err = tx.Exec(ctx, `INSERT INTO watchers (record_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, it.ID, u.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := a.refresh(ctx, it.ID, it.Type, it.Space); err != nil {
		return nil, err
	}
	a.Events.Publish(Event{Type: "record", ID: it.ID})
	return &it, nil
}

func errorsOnly(is []model.Issue) []model.Issue {
	var out []model.Issue
	for _, i := range is {
		if i.Level == "error" {
			out = append(out, i)
		}
	}
	return out
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// ---------------------------------------------------------------------------
// View

type revRow struct {
	Revision   string     `json:"revision"`
	Status     string     `json:"status"`
	CO         string     `json:"co,omitempty"`
	ReleasedAt *time.Time `json:"releasedAt,omitempty"`
}

type itemPerms struct {
	Edit          bool   `json:"edit"`
	EditTarget    string `json:"editTarget"` // main, next, co or empty
	EditBlocked   string `json:"editBlocked,omitempty"`
	StartRevision bool   `json:"startRevision"`
	Delete        bool   `json:"delete"`
	Lock          bool   `json:"lock"`
	AddToCO       bool   `json:"addToCO"`
	Convert       bool   `json:"convert"`
}

type itemView struct {
	Item       *model.Item   `json:"item"`
	Hash       string        `json:"hash"`
	Showing    string        `json:"showing"` // current, draft, co, revision
	Status     string        `json:"status"`
	Current    *model.Item   `json:"current,omitempty"`
	Draft      *model.Item   `json:"draft,omitempty"`
	DraftHash  string        `json:"draftHash,omitempty"`
	CO         *coRef        `json:"co,omitempty"`
	Revisions  []revRow      `json:"revisions"`
	Perms      itemPerms     `json:"perms"`
	Lock       []userRef     `json:"lock"`
	Watching   bool          `json:"watching"`
	WorkCount  int           `json:"workCount"`
	Issues     []model.Issue `json:"issues"`
	Expected   []string      `json:"expectedFiles"`
	Kind       string        `json:"kind"`
	ReplacedBy []string      `json:"replacedBy,omitempty"`
}

type userRef struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

type coRef struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Branch string `json:"-"`
	Action string `json:"action"`
	Owner  string `json:"owner"`
}

// openCOFor returns the open change order that includes an item, if any.
func (a *App) openCOFor(ctx context.Context, q querier, id string) *coRef {
	var c coRef
	err := q.QueryRow(ctx, `SELECT c.id, c.title, c.state, c.branch, i.action, c.owner_id::text FROM change_orders c JOIN co_items i ON i.co_id = c.id
		WHERE i.item_id = $1 AND c.state IN ('Draft', 'In Review', 'Approved') LIMIT 1`, id).Scan(&c.ID, &c.Title, &c.State, &c.Branch, &c.Action, &c.Owner)
	if err != nil {
		return nil
	}
	return &c
}

func (a *App) apiItem(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || !e.Item.IsEngineering() || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	v, err := a.itemView(ctx, u, e, r.URL.Query().Get("rev"), r.URL.Query().Get("show"))
	if err != nil {
		return err
	}
	a.DB.Exec(ctx, `INSERT INTO recent_views (user_id, record_id) VALUES ($1, $2) ON CONFLICT (user_id, record_id) DO UPDATE SET viewed_at = now()`, u.ID, id)
	return ok(w, v)
}

func (a *App) itemView(ctx context.Context, u *User, e *entry, rev, show string) (*itemView, error) {
	id := e.Item.ID
	v := &itemView{Item: e.Item, Hash: e.Hash, Showing: "current", Current: e.Item, Kind: kindOf(e.Item), Revisions: []revRow{}, Issues: []model.Issue{}}
	v.CO = a.openCOFor(ctx, a.DB, id)
	// The working draft: on the change order branch, or the next revision on main.
	if v.CO != nil {
		if b, h, err := a.Repo.Read(v.CO.Branch, a.draftPath(e)); err == nil {
			if it, err := model.Parse(b); err == nil {
				v.Draft, v.DraftHash = it, h
			}
		}
	} else if e.Next != nil {
		v.Draft, v.DraftHash = e.Next, e.NextHash
	}
	v.Status = e.Item.Status
	if v.CO != nil && (v.CO.State == "In Review" || v.CO.State == "Approved") && e.Item.Status == model.StatusDraft {
		v.Status = model.StatusInReview
	}
	switch {
	case rev != "" && rev != e.Item.Revision:
		old, err := a.itemAtRevision(ctx, id, rev)
		if err != nil {
			return nil, errNotFound
		}
		v.Item, v.Hash, v.Showing, v.Status = old, "", "revision", old.Status
	case show == "draft" && v.Draft != nil:
		v.Item, v.Hash, v.Showing = v.Draft, v.DraftHash, "draft"
		v.Status = model.StatusDraft
		if v.CO != nil && (v.CO.State == "In Review" || v.CO.State == "Approved") {
			v.Status = model.StatusInReview
		}
	}
	rows, err := a.DB.Query(ctx, `SELECT revision, status, co_id, released_at FROM item_revisions WHERE item_id = $1 ORDER BY revision DESC`, id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var rr revRow
		var t time.Time
		if err := rows.Scan(&rr.Revision, &rr.Status, &rr.CO, &t); err != nil {
			rows.Close()
			return nil, err
		}
		rr.ReleasedAt = &t
		v.Revisions = append(v.Revisions, rr)
	}
	rows.Close()
	if v.Draft != nil && e.Item.Status != model.StatusDraft {
		st := model.StatusDraft
		if v.CO != nil && (v.CO.State == "In Review" || v.CO.State == "Approved") {
			st = model.StatusInReview
		}
		dr := revRow{Revision: v.Draft.Revision, Status: st}
		if v.CO != nil {
			dr.CO = v.CO.ID
		}
		v.Revisions = append([]revRow{dr}, v.Revisions...)
	}
	lock, err := a.lockOf(ctx, a.DB, id)
	if err != nil {
		return nil, err
	}
	v.Lock, err = a.userRefs(ctx, lock)
	if err != nil {
		return nil, err
	}
	a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM watchers WHERE record_id = $1 AND user_id = $2)`, id, u.ID).Scan(&v.Watching)
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return nil, err
	}
	rowsW, err := a.DB.Query(ctx, `SELECT DISTINCT src FROM links WHERE dst = $1 AND origin = 'db' AND type IN ('affects', 'found on')`, id)
	if err != nil {
		return nil, err
	}
	for rowsW.Next() {
		var s string
		if rowsW.Scan(&s) == nil && vis(s) && model.TypeOf(s) != "change_order" {
			v.WorkCount++
		}
	}
	rowsW.Close()
	target := v.Item
	if v.Showing == "current" && v.Draft != nil {
		target = v.Draft
	}
	for _, is := range model.Validate(target, cache.lookup) {
		v.Issues = append(v.Issues, is)
	}
	roles, err := a.fileRoleExpectations(ctx)
	if err != nil {
		return nil, err
	}
	v.Expected = model.ExpectedFileRoles(id, roles)
	for _, l := range e.Item.Links {
		if l.Type == "replaced by" {
			v.ReplacedBy = append(v.ReplacedBy, l.To)
		}
	}
	v.Perms = a.itemPerms(u, e, v.CO)
	return v, nil
}

// draftPath returns where the working draft of an item lives: the current
// file for a never-released item, otherwise the next-revision file.
func (a *App) draftPath(e *entry) string {
	if e.Item.Status == model.StatusDraft {
		return e.Path
	}
	return model.NextPath(e.Path)
}

func (a *App) itemPerms(u *User, e *entry, co *coRef) itemPerms {
	p := itemPerms{}
	isOwner := strings.EqualFold(e.Item.Owner, u.Username)
	if !u.Can(PermLibEdit) {
		p.EditBlocked = "Your role is read-only on engineering data."
		return p
	}
	p.Lock = isOwner || u.Can(PermUsers)
	switch {
	case co != nil && co.State == "Draft":
		p.Edit, p.EditTarget = true, "co"
	case co != nil:
		p.EditBlocked = fmt.Sprintf("%s is %s. It can be edited when the change order is back in Draft.", co.ID, strings.ToLower(co.State))
	case e.Item.Status == model.StatusDraft:
		p.Edit, p.EditTarget = true, "main"
		p.Delete = isOwner || u.Can(PermUsers)
		p.AddToCO = true
		p.Convert = e.Item.Kind == model.KindReference
	case e.Item.Status == model.StatusObsolete:
		p.EditBlocked = "This item is obsolete."
	case e.Next != nil:
		p.Edit, p.EditTarget = true, "next"
		p.AddToCO = true
	default:
		p.StartRevision = true
		p.AddToCO = true
		p.EditBlocked = "Released revisions are locked. Start a new revision to make changes."
	}
	if e.Item.Kind == model.KindReference && co == nil && e.Item.Status != model.StatusObsolete {
		p.Convert = true
	}
	return p
}

func (a *App) userRefs(ctx context.Context, ids []string) ([]userRef, error) {
	out := []userRef{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := a.DB.Query(ctx, `SELECT id::text, username, display_name FROM users WHERE id = ANY($1::uuid[]) ORDER BY display_name`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var x userRef
		if err := rows.Scan(&x.ID, &x.Username, &x.Name); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (a *App) fileRoleExpectations(ctx context.Context) (map[string][]string, error) {
	vs, err := a.ListValues(ctx, "file_roles", true)
	if err != nil {
		return nil, err
	}
	m := map[string][]string{}
	for _, v := range vs {
		m[v.Value] = v.ExpectedOn
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Save

type saveInput struct {
	Item    model.Item `json:"item"`
	Base    string     `json:"base"` // blob hash the editor opened
	Comment string     `json:"comment"`
}

func (a *App) apiSaveItem(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || !e.Item.IsEngineering() || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	var in saveInput
	if err := decode(r, &in); err != nil {
		return err
	}
	if in.Item.ID != "" && in.Item.ID != id {
		return badRequest("The item number cannot be changed.")
	}
	h, err := a.SaveItem(ctx, u, clientIP(r), e, &in.Item, in.Base, in.Comment)
	if err != nil {
		return err
	}
	return ok(w, map[string]string{"hash": h})
}

// SaveItem writes a new version of an item's working draft.
func (a *App) SaveItem(ctx context.Context, u *User, ip string, e *entry, it *model.Item, base, comment string) (string, error) {
	co := a.openCOFor(ctx, a.DB, e.Item.ID)
	perms := a.itemPerms(u, e, co)
	for i := range it.BOM {
		if !model.CanBeBOMLine(it.BOM[i].Item) {
			return "", badRequest("%s cannot be added as a BOM line.", it.BOM[i].Item)
		}
		if cache.lookup(it.BOM[i].Item) == nil {
			return "", badRequest("BOM line %s: item does not exist.", it.BOM[i].Item)
		}
	}
	return a.writeRecord(ctx, u, ip, e, co, perms, it, base, comment, "item.save")
}

// writeRecord saves a git record to its edit target (main, the next
// revision file, or the change order branch) with conflict detection.
func (a *App) writeRecord(ctx context.Context, u *User, ip string, e *entry, co *coRef, perms itemPerms, it *model.Item, base, comment, action string) (string, error) {
	if !perms.Edit {
		if perms.EditBlocked != "" {
			return "", &Error{Status: http.StatusForbidden, Code: "locked", Message: perms.EditBlocked}
		}
		return "", errForbidden
	}
	branch, path := repo.Main, e.Path
	var prev *model.Item
	switch perms.EditTarget {
	case "main":
		prev = e.Item
	case "next":
		path, prev = model.NextPath(e.Path), e.Next
	case "co":
		branch, path = co.Branch, a.draftPath(e)
		b, _, err := a.Repo.Read(branch, path)
		if err != nil {
			return "", err
		}
		if prev, err = model.Parse(b); err != nil {
			return "", err
		}
	default:
		return "", errForbidden
	}
	// Fields the editor cannot change.
	it.ID, it.Type, it.Kind, it.Revision, it.Status, it.CreatedBy = prev.ID, prev.Type, prev.Kind, prev.Revision, prev.Status, prev.CreatedBy
	if it.Type == model.TypePage {
		it.Space, it.Parent, it.Sort = prev.Space, prev.Parent, prev.Sort
		if it.Controlled == "" {
			it.Controlled = prev.Controlled
		}
	}
	if it.Kind == model.KindCOTS {
		it.Manufacturer, it.OrderNumber = prev.Manufacturer, prev.OrderNumber
	}
	if !strings.EqualFold(it.Owner, prev.Owner) {
		var exists bool
		a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE lower(username) = lower($1) AND active)`, it.Owner).Scan(&exists)
		if !exists {
			return "", badRequest("Unknown owner.")
		}
	}
	if it.Space != prev.Space {
		var exists bool
		a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM spaces WHERE id = $1)`, it.Space).Scan(&exists)
		if !exists {
			return "", badRequest("Unknown home space.")
		}
	}
	if err := a.checkFiles(ctx, prev, it); err != nil {
		return "", err
	}
	if issues := errorsOnly(model.Required(it)); len(issues) > 0 {
		return "", &Error{Status: http.StatusBadRequest, Code: "invalid", Message: issues[0].Message + ".", Data: issues}
	}
	b, err := it.Marshal()
	if err != nil {
		return "", err
	}
	msg := fmt.Sprintf("Edit %s", it.ID)
	if c := strings.TrimSpace(comment); c != "" {
		msg += ": " + firstLine(c)
		if strings.Contains(c, "\n") {
			msg += "\n\n" + c
		}
	}
	if branch != repo.Main {
		msg += " (" + co.ID + ")"
	}
	if base == "" {
		return "", badRequest("Missing base version.")
	}
	commit, err := a.Repo.Commit(branch, []repo.Change{{Path: path, Content: b, Expect: base}}, author(u), msg)
	if errors.Is(err, repo.ErrConflict) {
		cur, curHash, _ := a.Repo.Read(branch, path)
		var curItem *model.Item
		if cur != nil {
			curItem, _ = model.Parse(cur)
		}
		return "", conflict("Someone saved this record after you opened it. Review their changes before saving again.",
			map[string]any{"current": curItem, "hash": curHash, "diff": lineDiff(string(cur), string(b))})
	}
	if err != nil {
		return "", err
	}
	_, newHash, _ := a.Repo.Read(branch, path)
	a.DB.Exec(ctx, `DELETE FROM edit_drafts WHERE user_id = $1 AND record_id = $2`, u.ID, it.ID)
	if branch == repo.Main {
		if err := a.refresh(ctx, it.ID, it.Type, it.Space); err != nil {
			return "", err
		}
	}
	a.Audit(ctx, u, ip, action, it.ID, map[string]any{"commit": commit, "target": perms.EditTarget})
	a.notifyWatchers(ctx, u, it.ID, "Watched record changed", fmt.Sprintf("%s changed %s %s", u.DisplayName, it.ID, it.Title))
	a.Events.Publish(Event{Type: "record", ID: it.ID, Space: spaceIfPage(it)})
	return newHash, nil
}

func spaceIfPage(it *model.Item) string {
	if it.Type == model.TypePage {
		return it.Space
	}
	return ""
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return truncate(l, 120)
}

// checkFiles refuses attachments whose content is not in the file store and
// keeps file metadata from the store.
func (a *App) checkFiles(ctx context.Context, prev, it *model.Item) error {
	for i := range it.Files {
		f := &it.Files[i]
		if !shaRe.MatchString(f.SHA256) {
			return badRequest("Invalid attachment.")
		}
		var size int64
		var mime string
		if err := a.DB.QueryRow(ctx, `SELECT size, mime FROM files WHERE sha256 = $1`, f.SHA256).Scan(&size, &mime); err != nil {
			return badRequest("Attachment %s was not uploaded.", f.Name)
		}
		f.Size, f.Mime = size, mime
		f.Name = sanitizeFileName(f.Name)
		if f.Role == "" {
			f.Role = "Other"
		}
	}
	return nil
}

func sanitizeFileName(n string) string {
	n = strings.Map(func(r rune) rune {
		if r < 32 || r == '/' || r == '\\' || r == '"' {
			return '_'
		}
		return r
	}, strings.TrimSpace(n))
	if n == "" {
		n = "file"
	}
	return truncate(n, 200)
}

// ---------------------------------------------------------------------------
// Revisions, deletion and conversion

func (a *App) apiStartRevision(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || !e.Item.IsEngineering() || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	if err := a.StartRevision(ctx, u, clientIP(r), e); err != nil {
		return err
	}
	return ok(w, nil)
}

// StartRevision copies the released revision into a draft of the next one.
func (a *App) StartRevision(ctx context.Context, u *User, ip string, e *entry) error {
	if e.Item.Type == model.TypePage {
		if !u.CanIn(e.Item.Space, PermSpaceEdit) || e.Item.Status != model.StatusReleased || e.Next != nil {
			return errForbidden
		}
	} else if !a.itemPerms(u, e, a.openCOFor(ctx, a.DB, e.Item.ID)).StartRevision {
		return errForbidden
	}
	next := e.Item.Clone()
	next.Revision = model.NextRevision(e.Item.Revision)
	next.Status = model.StatusDraft
	b, err := next.Marshal()
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("Start revision %s of %s", next.Revision, e.Item.ID)
	if _, err := a.Repo.Commit(repo.Main, []repo.Change{{Path: model.NextPath(e.Path), Content: b, Expect: "new"}}, author(u), msg); err != nil {
		return err
	}
	if err := a.refresh(ctx, e.Item.ID, e.Item.Type, e.Item.Space); err != nil {
		return err
	}
	a.Audit(ctx, u, ip, "item.revision.start", e.Item.ID, map[string]any{"revision": next.Revision})
	a.Events.Publish(Event{Type: "record", ID: e.Item.ID})
	return nil
}

func (a *App) apiDeleteItem(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	co := a.openCOFor(ctx, a.DB, id)
	if e.Item.IsEngineering() {
		if !a.itemPerms(u, e, co).Delete {
			if e.Item.Status != model.StatusDraft {
				return badRequest("Released items cannot be deleted. Make them obsolete through a change order.")
			}
			return errForbidden
		}
	}
	return ok(w, a.deleteDraftRecord(ctx, u, clientIP(r), e))
}

// deleteDraftRecord removes a never-released record and retires its number.
func (a *App) deleteDraftRecord(ctx context.Context, u *User, ip string, e *entry) error {
	var users []string
	rows, err := a.DB.Query(ctx, `SELECT DISTINCT src FROM links WHERE dst = $1`, e.Item.ID)
	if err == nil {
		for rows.Next() {
			var s string
			if rows.Scan(&s) == nil {
				users = append(users, s)
			}
		}
		rows.Close()
	}
	if len(users) > 0 {
		return badRequest("%s is used by %s. Remove those links or BOM lines first.", e.Item.ID, strings.Join(users, ", "))
	}
	changes := []repo.Change{{Path: e.Path, Delete: true}}
	if e.Next != nil {
		changes = append(changes, repo.Change{Path: model.NextPath(e.Path), Delete: true})
	}
	if _, err := a.Repo.Commit(repo.Main, changes, author(u), fmt.Sprintf("Delete draft %s %s", e.Item.ID, e.Item.Title)); err != nil {
		return err
	}
	if _, err := a.DB.Exec(ctx, `UPDATE identifiers SET state = 'retired' WHERE id = $1`, e.Item.ID); err != nil {
		return err
	}
	if err := a.refresh(ctx, e.Item.ID, e.Item.Type, e.Item.Space); err != nil {
		return err
	}
	a.DB.Exec(ctx, `DELETE FROM watchers WHERE record_id = $1`, e.Item.ID)
	a.DB.Exec(ctx, `DELETE FROM recent_views WHERE record_id = $1`, e.Item.ID)
	a.Audit(ctx, u, ip, "record.delete", e.Item.ID, map[string]any{"title": e.Item.Title})
	a.Events.Publish(Event{Type: "record", ID: e.Item.ID})
	return nil
}

// apiConvertReference issues a production number for a reference part and
// links the reference part to it.
func (a *App) apiConvertReference(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	if !a.itemPerms(u, e, a.openCOFor(ctx, a.DB, id)).Convert {
		return errForbidden
	}
	var in struct {
		Number string `json:"number"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	src := e.Item
	if e.Next != nil {
		src = e.Next
	}
	n := src.Clone()
	n.Kind = model.KindProduction
	n.Links = nil
	n.Files = src.Files
	n.Owner = u.Username
	created, err := a.CreateItem(ctx, u, clientIP(r), createInput{Item: *n, Number: in.Number, Comment: "Converted from reference part " + id})
	if err != nil {
		return err
	}
	// Record the replacement on the reference part.
	target := e.Path
	cur := e.Item
	hash := e.Hash
	if e.Item.Status != model.StatusDraft {
		if e.Next == nil {
			if err := a.StartRevision(ctx, u, clientIP(r), e); err != nil {
				return err
			}
			e = cache.get(id)
		}
		target, cur, hash = model.NextPath(e.Path), e.Next, e.NextHash
	}
	upd := cur.Clone()
	upd.Links = append(upd.Links, model.Link{Type: "replaced by", To: created.ID})
	b, err := upd.Marshal()
	if err != nil {
		return err
	}
	if _, err := a.Repo.Commit(repo.Main, []repo.Change{{Path: target, Content: b, Expect: hash}}, author(u), fmt.Sprintf("%s replaced by %s", id, created.ID)); err != nil {
		return err
	}
	if err := a.refresh(ctx, id, upd.Type, upd.Space); err != nil {
		return err
	}
	a.Audit(ctx, u, clientIP(r), "item.convert", id, map[string]any{"to": created.ID})
	return ok(w, map[string]string{"id": created.ID})
}

// ---------------------------------------------------------------------------
// History

type historyRow struct {
	Commit  string    `json:"commit"`
	Blob    string    `json:"blob"`
	Author  string    `json:"author"`
	When    time.Time `json:"when"`
	Message string    `json:"message"`
	File    string    `json:"file"` // current, next or co
	Deleted bool      `json:"deleted,omitempty"`
}

func (a *App) recordHistory(ctx context.Context, e *entry, co *coRef) ([]historyRow, error) {
	var out []historyRow
	add := func(ref, path, file string) error {
		vs, err := a.Repo.History(ref, path, 500)
		if err != nil {
			return err
		}
		for _, v := range vs {
			out = append(out, historyRow{Commit: v.Commit, Blob: v.Blob, Author: v.Author, When: v.When, Message: v.Message, File: file, Deleted: v.Deleted})
		}
		return nil
	}
	if err := add(repo.Main, e.Path, "current"); err != nil {
		return nil, err
	}
	if e.Item.Type != model.TypePage || e.Next != nil {
		if err := add(repo.Main, model.NextPath(e.Path), "next"); err != nil {
			return nil, err
		}
	}
	if co != nil {
		vs, err := a.Repo.History(co.Branch, a.draftPath(e), 200)
		if err == nil {
			mainHead, _ := a.Repo.Head(repo.Main)
			seen := map[string]bool{}
			for _, h := range out {
				seen[h.Commit] = true
			}
			for _, v := range vs {
				if seen[v.Commit] || v.Commit == mainHead {
					continue
				}
				if strings.Contains(v.Message, "("+co.ID+")") {
					out = append(out, historyRow{Commit: v.Commit, Blob: v.Blob, Author: v.Author, When: v.When, Message: v.Message, File: "co"})
				}
			}
		}
	}
	// Moving a draft to released deletes the next file; skip those deletions.
	var keep []historyRow
	for _, h := range out {
		if !(h.Deleted && h.File == "next") {
			keep = append(keep, h)
		}
	}
	sort.SliceStable(keep, func(i, j int) bool { return keep[i].When.After(keep[j].When) })
	return keep, nil
}

func (a *App) apiHistory(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	h, err := a.recordHistory(ctx, e, a.openCOFor(ctx, a.DB, id))
	if err != nil {
		return err
	}
	if h == nil {
		h = []historyRow{}
	}
	return ok(w, h)
}

// apiDiff compares two blob versions of a record (from its history).
func (a *App) apiDiff(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	h, err := a.recordHistory(ctx, e, a.openCOFor(ctx, a.DB, id))
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	for _, x := range h {
		allowed[x.Blob] = true
	}
	from, to := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	read := func(blob string) (string, error) {
		if blob == "" || blob == "0000000000000000000000000000000000000000" {
			return "", nil
		}
		if !allowed[blob] {
			return "", errNotFound
		}
		b, err := a.Repo.Blob(blob)
		return string(b), err
	}
	fa, err := read(from)
	if err != nil {
		return err
	}
	fb, err := read(to)
	if err != nil {
		return err
	}
	return ok(w, map[string]any{"lines": lineDiff(fa, fb), "from": fa, "to": fb})
}

// apiVersion returns one historical version of a record.
func (a *App) apiVersion(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	h, err := a.recordHistory(ctx, e, a.openCOFor(ctx, a.DB, id))
	if err != nil {
		return err
	}
	blob := r.PathValue("blob")
	for _, x := range h {
		if x.Blob == blob {
			b, err := a.Repo.Blob(blob)
			if err != nil {
				return err
			}
			it, err := model.Parse(b)
			if err != nil {
				return err
			}
			return ok(w, map[string]any{"item": it, "version": x})
		}
	}
	return errNotFound
}
