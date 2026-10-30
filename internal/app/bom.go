package app

import (
	"net/http"
	"sort"

	"designhub/internal/model"
	"designhub/internal/repo"
)

// apiBOM returns the multi-level tree and flattened list of an assembly.
// show=draft uses the working draft at the top level.
func (a *App) apiBOM(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	root := e.Item
	if r.URL.Query().Get("show") == "draft" {
		if co := a.openCOFor(ctx, a.DB, id); co != nil {
			if b, _, err := a.Repo.Read(co.Branch, a.draftPath(e)); err == nil {
				if it, err := model.Parse(b); err == nil {
					root = it
				}
			}
		} else if e.Next != nil {
			root = e.Next
		}
	}
	if !root.HasBOM() {
		return badRequest("%s is not an assembly.", id)
	}
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	tree := model.Tree(root, cache.lookup)
	flat := model.Flatten(tree)
	shownFlat := []model.FlatLine{}
	for _, f := range flat {
		if vis(f.ID) {
			shownFlat = append(shownFlat, f)
		}
	}
	model.Hide(tree, vis)
	issues := []model.Issue{}
	for _, is := range model.Validate(root, cache.lookup) {
		if is.Field == "bom" {
			issues = append(issues, is)
		}
	}
	return ok(w, map[string]any{"tree": tree, "flat": shownFlat, "issues": issues})
}

type usedIn struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Revision string  `json:"revision"`
	Status   string  `json:"status"`
	Qty      float64 `json:"qty"`
	Find     int     `json:"find"`
	Level    string  `json:"level"` // direct or indirect
}

func (a *App) apiWhereUsed(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	if cache.get(id) == nil || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	all := cache.all(model.TypePart)
	out := []usedIn{}
	direct := map[string]bool{}
	for _, p := range model.WhereUsed(id, all) {
		direct[p.ID] = true
		if !vis(p.ID) {
			continue
		}
		for _, l := range p.BOM {
			if l.Item == id {
				out = append(out, usedIn{p.ID, p.Title, p.Revision, p.Status, l.Qty, l.Find, "direct"})
			}
		}
	}
	for _, anc := range model.Ancestors(id, all) {
		if direct[anc] || !vis(anc) {
			continue
		}
		p := cache.lookup(anc)
		out = append(out, usedIn{ID: p.ID, Title: p.Title, Revision: p.Revision, Status: p.Status, Level: "indirect"})
	}
	// Assemblies whose working drafts use the item.
	drafts := []string{}
	cache.mu.RLock()
	for _, e := range cache.items {
		if e.Next != nil {
			for _, l := range e.Next.BOM {
				if l.Item == id && !direct[e.Item.ID] {
					drafts = append(drafts, e.Item.ID)
				}
			}
		}
	}
	cache.mu.RUnlock()
	sort.Strings(drafts)
	return ok(w, map[string]any{"usedIn": out, "drafts": drafts})
}

func (a *App) apiItemValidation(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	e := cache.get(id)
	if e == nil || !a.canSee(ctx, u, id) {
		return errNotFound
	}
	it := e.Item
	if r.URL.Query().Get("show") == "draft" && e.Next != nil {
		it = e.Next
	}
	issues := model.Validate(it, cache.lookup)
	if issues == nil {
		issues = []model.Issue{}
	}
	return ok(w, issues)
}

// apiLibraryValidation validates every engineering item and reports files
// that cannot be read.
func (a *App) apiLibraryValidation(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	if !u.Can(PermLibView) {
		return errForbidden
	}
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	out := []model.Issue{}
	for _, it := range cache.all("") {
		if !vis(it.ID) {
			continue
		}
		out = append(out, model.Validate(it, cache.lookup)...)
	}
	cache.mu.RLock()
	for p, msg := range cache.bad {
		out = append(out, model.Issue{Level: "error", Item: p, Message: "File cannot be read: " + msg})
	}
	cache.mu.RUnlock()
	return ok(w, out)
}

// ---------------------------------------------------------------------------
// Traceability

type traceRow struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	Status       string     `json:"status"`
	Priority     string     `json:"priority"`
	SatisfiedBy  []traceRef `json:"satisfiedBy"`
	VerifiedBy   []traceRef `json:"verifiedBy"`
	DerivedFrom  []string   `json:"derivedFrom"`
	Coverage     string     `json:"coverage"` // Complete, No test, No design, No design and no test
}

type traceRef struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

func (a *App) traceability(vis func(string) bool, ids map[string]bool) []traceRow {
	sat := map[string][]traceRef{}
	ver := map[string][]traceRef{}
	for _, it := range cache.all("") {
		for _, l := range it.Links {
			ref := traceRef{it.ID, it.Title, it.Status}
			switch l.Type {
			case "satisfies":
				sat[l.To] = append(sat[l.To], ref)
			case "verifies":
				ver[l.To] = append(ver[l.To], ref)
			}
		}
	}
	out := []traceRow{}
	for _, req := range cache.all(model.TypeRequirement) {
		if !vis(req.ID) || (ids != nil && !ids[req.ID]) {
			continue
		}
		row := traceRow{ID: req.ID, Title: req.Title, Status: req.Status, Priority: req.Priority, SatisfiedBy: []traceRef{}, VerifiedBy: []traceRef{}, DerivedFrom: []string{}}
		for _, s := range sat[req.ID] {
			if vis(s.ID) {
				row.SatisfiedBy = append(row.SatisfiedBy, s)
			}
		}
		for _, v := range ver[req.ID] {
			if vis(v.ID) {
				row.VerifiedBy = append(row.VerifiedBy, v)
			}
		}
		for _, l := range req.Links {
			if l.Type == "derived from" {
				row.DerivedFrom = append(row.DerivedFrom, l.To)
			}
		}
		switch {
		case len(sat[req.ID]) > 0 && len(ver[req.ID]) > 0:
			row.Coverage = "Complete"
		case len(sat[req.ID]) > 0:
			row.Coverage = "No test"
		case len(ver[req.ID]) > 0:
			row.Coverage = "No design"
		default:
			row.Coverage = "No design and no test"
		}
		out = append(out, row)
	}
	return out
}

func (a *App) apiTraceability(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	if !u.Can(PermLibView) {
		return errForbidden
	}
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	var ids map[string]bool
	if root := r.URL.Query().Get("item"); root != "" {
		// Requirements satisfied by an assembly or any item in its BOM.
		e := cache.get(root)
		if e == nil {
			return errNotFound
		}
		ids = map[string]bool{}
		items := map[string]bool{root: true}
		if e.Item.HasBOM() {
			for _, f := range model.Flatten(model.Tree(e.Item, cache.lookup)) {
				items[f.ID] = true
			}
		}
		for id := range items {
			if it := cache.lookup(id); it != nil {
				for _, l := range it.Links {
					if l.Type == "satisfies" {
						ids[l.To] = true
					}
				}
			}
		}
		if e.Item.Type == model.TypeRequirement {
			ids[root] = true
		}
	}
	return ok(w, a.traceability(vis, ids))
}

// itemAtRef reads an item at a branch, tag or commit.
func (a *App) itemAtRef(ref, path string) (*model.Item, error) {
	b, _, err := a.Repo.Read(ref, path)
	if err != nil {
		return nil, err
	}
	return model.Parse(b)
}

var _ = repo.Main
