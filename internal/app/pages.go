package app

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"

	"designhub/internal/model"
	"designhub/internal/repo"
)

// isControlled resolves a page's Controlled setting.
func (a *App) isControlled(ctx context.Context, p *model.Item) bool {
	switch p.Controlled {
	case "yes":
		return true
	case "no":
		return false
	}
	return a.controlledDefault(ctx, p.Space, p.PageType)
}

type pagePerms struct {
	itemPerms
	EditUnderCO bool `json:"editUnderCO"`
	Create      bool `json:"create"`
	Move        bool `json:"move"`
	Comment     bool `json:"comment"`
}

func (a *App) pagePerms(ctx context.Context, u *User, e *entry, co *coRef) pagePerms {
	p := pagePerms{}
	space := e.Item.Space
	p.Comment = u.InSpace(space)
	if !u.CanIn(space, PermSpaceEdit) {
		p.EditBlocked = "Your role in this space is read-only."
		return p
	}
	var archived bool
	a.DB.QueryRow(ctx, `SELECT archived FROM spaces WHERE id = $1`, space).Scan(&archived)
	if archived {
		p.EditBlocked = "This space is archived."
		return p
	}
	p.Create, p.Move = true, true
	isOwner := strings.EqualFold(e.Item.Owner, u.Username)
	p.Lock = isOwner || u.CanIn(space, PermSpaceAdmin)
	controlled := a.isControlled(ctx, e.Item)
	switch {
	case co != nil && co.State == "Draft":
		p.Edit, p.EditTarget = true, "co"
	case co != nil:
		p.EditBlocked = fmt.Sprintf("%s is %s. It can be edited when the change order is back in Draft.", co.ID, strings.ToLower(co.State))
	case !controlled || e.Item.Status == model.StatusDraft || e.Item.Status == "":
		p.Edit, p.EditTarget = true, "main"
		home := false
		var hp string
		a.DB.QueryRow(ctx, `SELECT home_page_id FROM spaces WHERE id = $1`, space).Scan(&hp)
		home = hp == e.Item.ID
		p.Delete = (isOwner || u.CanIn(space, PermSpaceAdmin)) && !home && e.Item.Status != model.StatusReleased
		p.AddToCO = controlled && e.Item.Status == model.StatusDraft
	case e.Item.Status == model.StatusObsolete:
		p.EditBlocked = "This page is obsolete."
	default:
		p.EditUnderCO = true
		p.AddToCO = true
		p.EditBlocked = "This page is controlled. Edit it under a change order."
	}
	return p
}

// ---------------------------------------------------------------------------
// Tree

type treeNode struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Parent     string `json:"parent"`
	Sort       int    `json:"sort"`
	PageType   string `json:"pageType"`
	Status     string `json:"status"`
	Controlled bool   `json:"controlled"`
	Home       bool   `json:"home,omitempty"`
}

func (a *App) pagesIn(space string) []*model.Item {
	var out []*model.Item
	for _, p := range cache.all(model.TypePage) {
		if p.Space == space {
			out = append(out, p)
		}
	}
	return out
}

func (a *App) apiPageTree(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	space := r.PathValue("id")
	if !u.InSpace(space) {
		return errNotFound
	}
	locks, err := a.locks(ctx)
	if err != nil {
		return err
	}
	var home string
	a.DB.QueryRow(ctx, `SELECT home_page_id FROM spaces WHERE id = $1`, space).Scan(&home)
	out := []treeNode{}
	for _, p := range a.pagesIn(space) {
		if !visible(u, "page", space, locks[p.ID]) {
			continue
		}
		out = append(out, treeNode{ID: p.ID, Title: p.Title, Parent: p.Parent, Sort: p.Sort, PageType: p.PageType, Status: p.Status,
			Controlled: a.isControlled(ctx, p), Home: p.ID == home})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Sort != out[j].Sort {
			return out[i].Sort < out[j].Sort
		}
		return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title)
	})
	return ok(w, map[string]any{"home": home, "pages": out})
}

// ---------------------------------------------------------------------------
// View

type pageView struct {
	itemView
	PagePerms   pagePerms   `json:"pagePerms"`
	Controlled  bool        `json:"controlled"`
	Breadcrumbs []treeNode  `json:"breadcrumbs"`
	Children    []treeNode  `json:"children"`
	SpaceName   string      `json:"spaceName"`
	IsHome      bool        `json:"isHome"`
}

func (a *App) apiPage(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || e.Item.Type != model.TypePage || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	iv, err := a.itemView(ctx, u, e, r.URL.Query().Get("rev"), r.URL.Query().Get("show"))
	if err != nil {
		return err
	}
	v := &pageView{itemView: *iv, Controlled: a.isControlled(ctx, e.Item), Breadcrumbs: []treeNode{}, Children: []treeNode{}}
	v.PagePerms = a.pagePerms(ctx, u, e, iv.CO)
	v.Perms = v.PagePerms.itemPerms
	if !v.Controlled {
		v.Status = ""
	}
	// Readers see the latest draft of a controlled page unless the page is
	// pinned to its released revision.
	if v.Showing == "current" && v.Draft != nil && !e.Item.ReleasedOnly && r.URL.Query().Get("show") == "" {
		v.Item, v.Hash, v.Showing = v.Draft, v.DraftHash, "draft"
	}
	var home string
	a.DB.QueryRow(ctx, `SELECT home_page_id, name FROM spaces WHERE id = $1`, e.Item.Space).Scan(&home, &v.SpaceName)
	v.IsHome = id == home
	// Breadcrumbs: space home, then ancestors.
	seen := map[string]bool{id: true}
	for p := e.Item.Parent; p != "" && !seen[p]; {
		seen[p] = true
		pe := cache.get(p)
		if pe == nil {
			break
		}
		v.Breadcrumbs = append([]treeNode{{ID: pe.Item.ID, Title: pe.Item.Title}}, v.Breadcrumbs...)
		p = pe.Item.Parent
	}
	if !v.IsHome && home != "" && (len(v.Breadcrumbs) == 0 || v.Breadcrumbs[0].ID != home) {
		if he := cache.get(home); he != nil {
			v.Breadcrumbs = append([]treeNode{{ID: home, Title: he.Item.Title}}, v.Breadcrumbs...)
		}
	}
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	for _, p := range a.pagesIn(e.Item.Space) {
		if p.Parent == id && vis(p.ID) {
			v.Children = append(v.Children, treeNode{ID: p.ID, Title: p.Title, Sort: p.Sort, PageType: p.PageType})
		}
	}
	sort.Slice(v.Children, func(i, j int) bool { return v.Children[i].Sort < v.Children[j].Sort })
	a.DB.Exec(ctx, `INSERT INTO recent_views (user_id, record_id) VALUES ($1, $2) ON CONFLICT (user_id, record_id) DO UPDATE SET viewed_at = now()`, u.ID, id)
	return ok(w, v)
}

// ---------------------------------------------------------------------------
// Create, save, move, restore

func (a *App) apiCreatePage(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	var in struct {
		Space      string `json:"space"`
		Parent     string `json:"parent"`
		Title      string `json:"title"`
		PageType   string `json:"pageType"`
		Controlled string `json:"controlled"`
		Body       string `json:"body"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	if !u.CanIn(in.Space, PermSpaceEdit) {
		return errForbidden
	}
	var archived bool
	if err := a.DB.QueryRow(ctx, `SELECT archived FROM spaces WHERE id = $1`, in.Space).Scan(&archived); err != nil {
		return errNotFound
	}
	if archived {
		return badRequest("This space is archived.")
	}
	if in.Parent != "" {
		pe := cache.get(in.Parent)
		if pe == nil || pe.Item.Type != model.TypePage || pe.Item.Space != in.Space {
			return badRequest("Unknown parent page.")
		}
	}
	if in.PageType == "" {
		in.PageType = "General"
	}
	if in.Controlled == "" {
		in.Controlled = "default"
	}
	if !contains([]string{"default", "yes", "no"}, in.Controlled) {
		return badRequest("Controlled must be default, yes or no.")
	}
	maxSort := 0
	for _, p := range a.pagesIn(in.Space) {
		if p.Parent == in.Parent && p.Sort > maxSort {
			maxSort = p.Sort
		}
	}
	p := &model.Item{Type: model.TypePage, Title: strings.TrimSpace(in.Title), Owner: u.Username, CreatedBy: u.Username, Space: in.Space,
		Parent: in.Parent, Sort: maxSort + 10, PageType: in.PageType, Controlled: in.Controlled, Body: in.Body}
	if a.isControlled(ctx, p) {
		p.Status = model.StatusDraft
	}
	err := a.tx(ctx, func(tx pgx.Tx) error {
		id, err := a.nextPrefixed(ctx, tx, "PG", "page", u.ID)
		if err != nil {
			return err
		}
		p.ID = id
		if issues := errorsOnly(model.Required(p)); len(issues) > 0 {
			return &Error{Status: http.StatusBadRequest, Code: "invalid", Message: issues[0].Message + ".", Data: issues}
		}
		b, err := p.Marshal()
		if err != nil {
			return err
		}
		if _, err := a.Repo.Commit(repo.Main, []repo.Change{{Path: model.Path(p), Content: b, Expect: "new"}}, author(u), fmt.Sprintf("Create page %s %s", p.ID, p.Title)); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, clientIP(r), "page.create", p.ID, map[string]any{"title": p.Title, "space": p.Space})
		_, err = tx.Exec(ctx, `INSERT INTO watchers (record_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, p.ID, u.ID)
		return err
	})
	if err != nil {
		return err
	}
	if err := a.refresh(ctx, p.ID, p.Type, p.Space); err != nil {
		return err
	}
	a.Events.Publish(Event{Type: "tree", ID: p.ID, Space: p.Space})
	return ok(w, map[string]string{"id": p.ID})
}

func (a *App) apiSavePage(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || e.Item.Type != model.TypePage || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	var in saveInput
	if err := decode(r, &in); err != nil {
		return err
	}
	co := a.openCOFor(ctx, a.DB, id)
	pp := a.pagePerms(ctx, u, e, co)
	// Switching a released controlled page to uncontrolled needs a change
	// order; on the change order branch that is allowed.
	if in.Item.Controlled == "" {
		in.Item.Controlled = e.Item.Controlled
	}
	if pp.EditTarget == "main" && e.Item.Status == model.StatusDraft {
		// A never-released controlled draft may still be switched.
	}
	h, err := a.writeRecord(ctx, u, clientIP(r), e, co, pp.itemPerms, &in.Item, in.Base, in.Comment, "page.save")
	if err != nil {
		return err
	}
	if pp.EditTarget == "main" {
		// The controlled setting may have changed the page's status.
		if ne := cache.get(id); ne != nil {
			want := ""
			if a.isControlled(ctx, ne.Item) {
				want = model.StatusDraft
			}
			if ne.Item.Status == "" && want != "" || ne.Item.Status == model.StatusDraft && want == "" {
				upd := ne.Item.Clone()
				upd.Status = want
				b, _ := upd.Marshal()
				if _, err := a.Repo.Commit(repo.Main, []repo.Change{{Path: ne.Path, Content: b, Expect: ne.Hash}}, author(u), "Update controlled setting of "+id); err == nil {
					a.refresh(ctx, id, upd.Type, upd.Space)
					_, h, _ = a.Repo.Read(repo.Main, ne.Path)
				}
			}
		}
	}
	a.Events.Publish(Event{Type: "tree", ID: id, Space: e.Item.Space})
	return ok(w, map[string]string{"hash": h})
}

func (a *App) apiMovePage(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || e.Item.Type != model.TypePage || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	if !a.pagePerms(ctx, u, e, nil).Move {
		return errForbidden
	}
	var in struct {
		Parent string `json:"parent"`
		Index  int    `json:"index"` // position among the new siblings
		Title  string `json:"title"` // rename (uncontrolled pages only)
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	space := e.Item.Space
	var home string
	a.DB.QueryRow(ctx, `SELECT home_page_id FROM spaces WHERE id = $1`, space).Scan(&home)
	if id == home && in.Parent != "" {
		return badRequest("The space home page cannot be moved.")
	}
	if in.Parent != "" {
		for p := in.Parent; p != ""; {
			if p == id {
				return badRequest("A page cannot be moved under itself.")
			}
			pe := cache.get(p)
			if pe == nil || pe.Item.Space != space {
				return badRequest("Unknown parent page.")
			}
			p = pe.Item.Parent
		}
	}
	// New order of siblings.
	var sibs []*model.Item
	for _, p := range a.pagesIn(space) {
		if p.Parent == in.Parent && p.ID != id {
			sibs = append(sibs, p)
		}
	}
	sort.Slice(sibs, func(i, j int) bool { return sibs[i].Sort < sibs[j].Sort })
	if in.Index < 0 || in.Index > len(sibs) {
		in.Index = len(sibs)
	}
	order := append(append(append([]*model.Item{}, sibs[:in.Index]...), e.Item), sibs[in.Index:]...)
	var changes []repo.Change
	for i, p := range order {
		pe := cache.get(p.ID)
		upd := pe.Item.Clone()
		changed := false
		if upd.Sort != (i+1)*10 {
			upd.Sort, changed = (i+1)*10, true
		}
		if p.ID == id {
			if upd.Parent != in.Parent {
				upd.Parent, changed = in.Parent, true
			}
			if t := strings.TrimSpace(in.Title); t != "" && t != upd.Title {
				if a.isControlled(ctx, upd) && upd.Status != model.StatusDraft {
					return badRequest("Controlled pages are renamed under a change order.")
				}
				upd.Title, changed = t, true
			}
		}
		if changed {
			b, err := upd.Marshal()
			if err != nil {
				return err
			}
			changes = append(changes, repo.Change{Path: pe.Path, Content: b, Expect: pe.Hash})
		}
	}
	if len(changes) > 0 {
		if _, err := a.Repo.Commit(repo.Main, changes, author(u), fmt.Sprintf("Move page %s", id)); err != nil {
			return err
		}
		for _, p := range order {
			if err := a.refresh(ctx, p.ID, model.TypePage, space); err != nil {
				return err
			}
		}
		a.Audit(ctx, u, clientIP(r), "page.move", id, map[string]any{"parent": in.Parent, "index": in.Index, "title": in.Title})
		a.Events.Publish(Event{Type: "tree", ID: id, Space: space})
	}
	return ok(w, nil)
}

func (a *App) apiRestorePage(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	var in struct {
		Blob string `json:"blob"`
		Base string `json:"base"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	co := a.openCOFor(ctx, a.DB, id)
	h, err := a.recordHistory(ctx, e, co)
	if err != nil {
		return err
	}
	var ver *historyRow
	for i := range h {
		if h[i].Blob == in.Blob {
			ver = &h[i]
		}
	}
	if ver == nil {
		return errNotFound
	}
	b, err := a.Repo.Blob(in.Blob)
	if err != nil {
		return err
	}
	old, err := model.Parse(b)
	if err != nil {
		return err
	}
	var perms itemPerms
	if e.Item.Type == model.TypePage {
		perms = a.pagePerms(ctx, u, e, co).itemPerms
	} else {
		perms = a.itemPerms(u, e, co)
	}
	comment := fmt.Sprintf("Restore version of %s by %s", ver.When.Format("2006-01-02 15:04"), ver.Author)
	nh, err := a.writeRecord(ctx, u, clientIP(r), e, co, perms, old, in.Base, comment, "record.restore")
	if err != nil {
		return err
	}
	return ok(w, map[string]string{"hash": nh})
}

func (a *App) apiDeletePage(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || e.Item.Type != model.TypePage || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	if !a.pagePerms(ctx, u, e, a.openCOFor(ctx, a.DB, id)).Delete {
		return errForbidden
	}
	for _, p := range a.pagesIn(e.Item.Space) {
		if p.Parent == id {
			return badRequest("Move or delete the child pages first.")
		}
	}
	if err := a.deleteDraftRecord(ctx, u, clientIP(r), e); err != nil {
		return err
	}
	a.Events.Publish(Event{Type: "tree", ID: id, Space: e.Item.Space})
	return ok(w, nil)
}
