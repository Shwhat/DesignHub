package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"designhub/internal/model"
	"designhub/internal/repo"
)

// Change order states.
const (
	CODraft     = "Draft"
	COInReview  = "In Review"
	COApproved  = "Approved"
	COReleased  = "Released"
	CORejected  = "Rejected"
	COCancelled = "Cancelled"
)

type coListRow struct {
	ID         string     `json:"id"`
	Title      string     `json:"title"`
	ChangeType string     `json:"changeType"`
	State      string     `json:"state"`
	Priority   string     `json:"priority"`
	Owner      string     `json:"owner"`
	Space      string     `json:"space"`
	NeedDate   *time.Time `json:"needDate"`
	Updated    time.Time  `json:"updated"`
	Items      int        `json:"items"`
	Awaiting   []string   `json:"awaiting,omitempty"`
}

// listCOs lists change orders visible to u. filter is open, mine, awaiting or all.
func (a *App) listCOs(ctx context.Context, u *User, filter, space string) ([]coListRow, error) {
	if !u.Can(PermLibView) {
		return []coListRow{}, nil
	}
	q := `SELECT c.id, c.title, c.change_type, c.state, c.priority, us.display_name, c.home_space_id, c.need_date, c.updated_at,
		(SELECT count(*) FROM co_items i WHERE i.co_id = c.id),
		COALESCE((SELECT array_agg(au.display_name ORDER BY au.display_name) FROM co_approvers ap JOIN users au ON au.id = ap.user_id WHERE ap.co_id = c.id AND ap.decision = ''), '{}')
		FROM change_orders c JOIN users us ON us.id = c.owner_id WHERE true`
	args := []any{}
	switch filter {
	case "open":
		q += ` AND c.state IN ('Draft', 'In Review', 'Approved')`
	case "awaiting":
		args = append(args, u.ID)
		q += fmt.Sprintf(` AND c.state = 'In Review' AND EXISTS (SELECT 1 FROM co_approvers ap WHERE ap.co_id = c.id AND ap.user_id = $%d AND ap.decision = '')`, len(args))
	case "release":
		q += ` AND c.state = 'Approved'`
	}
	if space != "" {
		args = append(args, space)
		q += fmt.Sprintf(` AND c.home_space_id = $%d`, len(args))
	}
	q += ` ORDER BY c.updated_at DESC LIMIT 500`
	rows, err := a.DB.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	locks, err := a.locks(ctx)
	if err != nil {
		return nil, err
	}
	out := []coListRow{}
	for rows.Next() {
		var c coListRow
		if err := rows.Scan(&c.ID, &c.Title, &c.ChangeType, &c.State, &c.Priority, &c.Owner, &c.Space, &c.NeedDate, &c.Updated, &c.Items, &c.Awaiting); err != nil {
			return nil, err
		}
		if visible(u, "change_order", "", locks[c.ID]) {
			out = append(out, c)
		}
	}
	return out, rows.Err()
}

func (a *App) apiChangeOrders(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	f := r.URL.Query().Get("filter")
	if f == "" {
		f = "all"
	}
	cos, err := a.listCOs(r.Context(), u, f, r.URL.Query().Get("space"))
	if err != nil {
		return err
	}
	return ok(w, cos)
}

// ---------------------------------------------------------------------------
// Create and edit

type coInput struct {
	Title             *string    `json:"title"`
	ChangeType        *string    `json:"changeType"`
	Reason            *string    `json:"reason"`
	Description       *string    `json:"description"`
	StockDisposition  *string    `json:"stockDisposition"`
	NeedDate          *string    `json:"needDate"`
	Priority          *string    `json:"priority"`
	HomeSpace         *string    `json:"homeSpace"`
	Owner             *string    `json:"owner"`
	Labels            *[]string  `json:"labels"`
	Items             []coItemIn `json:"items"`
	Approvers         []string   `json:"approvers"`
	Links             []string   `json:"links"` // NCRs or issues this change order fixes
}

type coItemIn struct {
	Item   string `json:"item"`
	Action string `json:"action"`
}

var coTypes = []string{"Initial release", "Design change", "Documentation only", "Obsolescence"}
var priorities = []string{"Low", "Normal", "High", "Urgent"}
var stockDispositions = []string{"", "Use as is", "Rework", "Scrap", "Return to vendor"}

func (a *App) apiCreateCO(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermCOCreate) {
		return errForbidden
	}
	var in coInput
	if err := decode(r, &in); err != nil {
		return err
	}
	id, err := a.CreateCO(r.Context(), u, clientIP(r), in)
	if err != nil {
		return err
	}
	return ok(w, map[string]string{"id": id})
}

// CreateCO creates a change order with its branch and optional items.
func (a *App) CreateCO(ctx context.Context, u *User, ip string, in coInput) (string, error) {
	title := strings.TrimSpace(deref(in.Title))
	if title == "" || len([]rune(title)) > 100 {
		return "", badRequest("Title is required (at most 100 characters).")
	}
	ct := deref(in.ChangeType)
	if ct == "" {
		ct = "Design change"
	}
	if !contains(coTypes, ct) {
		return "", badRequest("Unknown change type.")
	}
	pr := deref(in.Priority)
	if pr == "" {
		pr = "Normal"
	}
	if !contains(priorities, pr) {
		return "", badRequest("Unknown priority.")
	}
	space := deref(in.HomeSpace)
	if space == "" {
		for _, it := range in.Items {
			if e := cache.get(it.Item); e != nil {
				space = e.Item.Space
				break
			}
		}
	}
	var need any
	if d := deref(in.NeedDate); d != "" {
		t, err := time.Parse("2006-01-02", d)
		if err != nil {
			return "", badRequest("Need date must be a date.")
		}
		need = t
	}
	var id string
	err := a.tx(ctx, func(tx pgx.Tx) error {
		var err error
		if id, err = a.nextPrefixed(ctx, tx, "CO", "change_order", u.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO change_orders (id, title, change_type, reason, description, stock_disposition, need_date, priority, owner_id, created_by, home_space_id, labels, branch)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9, $10, $11, $12)`,
			id, title, ct, strings.TrimSpace(deref(in.Reason)), strings.TrimSpace(deref(in.Description)), deref(in.StockDisposition), need, pr, u.ID, space,
			nz(derefList(in.Labels)), "co/"+id); err != nil {
			return err
		}
		if err := a.Repo.CreateBranch("co/"+id, repo.Main); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO watchers (record_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, u.ID); err != nil {
			return err
		}
		for _, l := range in.Links {
			if _, err := tx.Exec(ctx, `INSERT INTO links (src, type, dst, origin) VALUES ($1, 'fixed by', $2, 'db') ON CONFLICT DO NOTHING`, l, id); err != nil {
				return err
			}
		}
		a.auditQ(ctx, tx, u, ip, "co.create", id, map[string]any{"title": title})
		return nil
	})
	if err != nil {
		return "", err
	}
	a.indexCO(ctx, id)
	for _, it := range in.Items {
		if err := a.AddCOItem(ctx, u, ip, id, it.Item, it.Action); err != nil {
			return id, err
		}
	}
	if len(in.Approvers) > 0 {
		if err := a.setApprovers(ctx, u, ip, id, in.Approvers); err != nil {
			return id, err
		}
	}
	return id, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefList(s *[]string) []string {
	if s == nil {
		return nil
	}
	return *s
}

// indexCO updates the search row of a change order.
func (a *App) indexCO(ctx context.Context, id string) {
	_, err := a.DB.Exec(ctx, `INSERT INTO records (id, kind, title, status, space_id, owner_id, labels, body, origin, updated_at)
		SELECT id, 'change_order', title, state, home_space_id, owner_id, labels, reason || E'\n' || description, 'db', now() FROM change_orders WHERE id = $1
		ON CONFLICT (id) DO UPDATE SET title = EXCLUDED.title, status = EXCLUDED.status, space_id = EXCLUDED.space_id, owner_id = EXCLUDED.owner_id,
		labels = EXCLUDED.labels, body = EXCLUDED.body, updated_at = now()`, id)
	if err != nil {
		fmt.Println("index", id, err)
	}
	a.DB.Exec(ctx, `DELETE FROM links WHERE src = $1 AND type = 'affects' AND origin = 'db'`, id)
	a.DB.Exec(ctx, `INSERT INTO links (src, type, dst, origin) SELECT co_id, 'affects', item_id, 'db' FROM co_items WHERE co_id = $1 ON CONFLICT DO NOTHING`, id)
}

type coRecord struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	ChangeType       string     `json:"changeType"`
	Reason           string     `json:"reason"`
	Description      string     `json:"description"`
	StockDisposition string     `json:"stockDisposition"`
	NeedDate         *time.Time `json:"needDate"`
	Priority         string     `json:"priority"`
	State            string     `json:"state"`
	OwnerID          string     `json:"ownerId"`
	Owner            string     `json:"owner"`
	CreatedBy        string     `json:"createdBy"`
	Space            string     `json:"space"`
	Labels           []string   `json:"labels"`
	Branch           string     `json:"branch"`
	Baseline         string     `json:"baseline"`
	ReleasedBy       string     `json:"releasedBy"`
	ReleasedAt       *time.Time `json:"releasedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
	Updated          time.Time  `json:"updated"`
	Version          int        `json:"version"`
}

func (a *App) loadCO(ctx context.Context, q querier, id string) (*coRecord, error) {
	c := &coRecord{}
	err := q.QueryRow(ctx, `SELECT c.id, c.title, c.change_type, c.reason, c.description, c.stock_disposition, c.need_date, c.priority, c.state,
		c.owner_id::text, o.display_name, cb.display_name, c.home_space_id, c.labels, c.branch, c.baseline, COALESCE(rb.display_name, ''), c.released_at,
		c.created_at, c.updated_at, c.version
		FROM change_orders c JOIN users o ON o.id = c.owner_id JOIN users cb ON cb.id = c.created_by LEFT JOIN users rb ON rb.id = c.released_by
		WHERE c.id = $1`, id).Scan(&c.ID, &c.Title, &c.ChangeType, &c.Reason, &c.Description, &c.StockDisposition, &c.NeedDate, &c.Priority, &c.State,
		&c.OwnerID, &c.Owner, &c.CreatedBy, &c.Space, &c.Labels, &c.Branch, &c.Baseline, &c.ReleasedBy, &c.ReleasedAt, &c.CreatedAt, &c.Updated, &c.Version)
	return c, err
}

func (a *App) apiUpdateCO(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	c, err := a.coForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !a.coEditable(u, c) {
		return errForbidden
	}
	var in coInput
	if err := decode(r, &in); err != nil {
		return err
	}
	set := func(col string, v any) error {
		_, err := a.DB.Exec(ctx, `UPDATE change_orders SET `+col+` = $2, updated_at = now(), version = version + 1 WHERE id = $1`, c.ID, v)
		return err
	}
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if t == "" || len([]rune(t)) > 100 {
			return badRequest("Title is required (at most 100 characters).")
		}
		if err := set("title", t); err != nil {
			return err
		}
	}
	if in.ChangeType != nil {
		if !contains(coTypes, *in.ChangeType) {
			return badRequest("Unknown change type.")
		}
		if err := set("change_type", *in.ChangeType); err != nil {
			return err
		}
	}
	if in.Reason != nil {
		if err := set("reason", strings.TrimSpace(*in.Reason)); err != nil {
			return err
		}
	}
	if in.Description != nil {
		if err := set("description", strings.TrimSpace(*in.Description)); err != nil {
			return err
		}
	}
	if in.StockDisposition != nil {
		if !contains(stockDispositions, *in.StockDisposition) {
			return badRequest("Unknown stock disposition.")
		}
		if err := set("stock_disposition", *in.StockDisposition); err != nil {
			return err
		}
	}
	if in.Priority != nil {
		if !contains(priorities, *in.Priority) {
			return badRequest("Unknown priority.")
		}
		if err := set("priority", *in.Priority); err != nil {
			return err
		}
	}
	if in.NeedDate != nil {
		var v any
		if *in.NeedDate != "" {
			t, err := time.Parse("2006-01-02", *in.NeedDate)
			if err != nil {
				return badRequest("Need date must be a date.")
			}
			v = t
		}
		if err := set("need_date", v); err != nil {
			return err
		}
	}
	if in.HomeSpace != nil {
		if err := set("home_space_id", *in.HomeSpace); err != nil {
			return err
		}
	}
	if in.Labels != nil {
		if err := set("labels", nz(*in.Labels)); err != nil {
			return err
		}
	}
	if in.Owner != nil {
		if err := set("owner_id", *in.Owner); err != nil {
			return err
		}
	}
	a.indexCO(ctx, c.ID)
	a.Audit(ctx, u, clientIP(r), "co.update", c.ID, in)
	a.Events.Publish(Event{Type: "record", ID: c.ID})
	return ok(w, nil)
}

func (a *App) coForUser(ctx context.Context, u *User, id string) (*coRecord, error) {
	if !u.Can(PermLibView) || !a.canSeeLocked(ctx, u, id) {
		return nil, errNotFound
	}
	c, err := a.loadCO(ctx, a.DB, id)
	if err != nil {
		return nil, errNotFound
	}
	return c, nil
}

func (a *App) canSeeLocked(ctx context.Context, u *User, id string) bool {
	lock, err := a.lockOf(ctx, a.DB, id)
	return err == nil && (len(lock) == 0 || contains(lock, u.ID) || u.IsSuperAdmin())
}

// coEditable reports whether u may change the change order's content.
func (a *App) coEditable(u *User, c *coRecord) bool {
	return c.State == CODraft && u.Can(PermCOCreate) && (c.OwnerID == u.ID || u.Can(PermUsers) || u.Can(PermLibEdit))
}

// ---------------------------------------------------------------------------
// Affected items

func (a *App) apiAddCOItem(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	c, err := a.coForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !a.coEditable(u, c) {
		return &Error{Status: http.StatusForbidden, Code: "locked", Message: "Items can be added only while the change order is in Draft."}
	}
	var in coItemIn
	if err := decode(r, &in); err != nil {
		return err
	}
	if !a.canSee(ctx, u, in.Item) {
		return badRequest("Unknown item %s.", in.Item)
	}
	if err := a.AddCOItem(ctx, u, clientIP(r), c.ID, in.Item, in.Action); err != nil {
		return err
	}
	return ok(w, nil)
}

// AddCOItem adds an engineering item or controlled page to a change order
// and copies its working draft to the change order branch.
func (a *App) AddCOItem(ctx context.Context, u *User, ip, coID, itemID, action string) error {
	c, err := a.loadCO(ctx, a.DB, coID)
	if err != nil {
		return err
	}
	if c.State != CODraft {
		return badRequest("Items can be added only while the change order is in Draft.")
	}
	e := cache.get(itemID)
	if e == nil {
		return badRequest("Unknown item %s.", itemID)
	}
	if e.Item.Type == model.TypePage && !a.isControlled(ctx, e.Item) {
		return badRequest("%s is not a controlled page.", itemID)
	}
	if other := a.openCOFor(ctx, a.DB, itemID); other != nil {
		if other.ID == coID {
			return nil
		}
		return badRequest("%s is already on %s.", itemID, other.ID)
	}
	if action == "" {
		switch {
		case e.Item.Status == model.StatusDraft:
			action = "Release"
		case e.Item.Status == model.StatusReleased:
			action = "Revise"
		}
	}
	from, to := e.Item.Revision, ""
	switch action {
	case "Release":
		if e.Item.Status != model.StatusDraft {
			return badRequest("%s is already released. Use Revise.", itemID)
		}
		from, to = "", model.NextRevision("")
	case "Revise":
		if e.Item.Status != model.StatusReleased {
			return badRequest("Only released items can be revised.")
		}
		if e.Next == nil {
			if err := a.StartRevision(ctx, u, ip, e); err != nil {
				if errors.Is(err, errForbidden) {
					return badRequest("You cannot start a new revision of %s.", itemID)
				}
				return err
			}
			e = cache.get(itemID)
		}
		to = e.Next.Revision
	case "Obsolete":
		if e.Item.Status != model.StatusReleased {
			return badRequest("Only released items can be made obsolete.")
		}
		to = e.Item.Revision
	default:
		return badRequest("Action must be Release, Revise or Obsolete.")
	}
	// Bring the branch's copy of the draft up to date with main.
	if action != "Obsolete" {
		p := a.draftPath(e)
		b, _, err := a.Repo.Read(repo.Main, p)
		if err != nil {
			return err
		}
		_, curHash, rerr := a.Repo.Read(c.Branch, p)
		ch := repo.Change{Path: p, Content: b}
		if rerr == nil {
			ch.Expect = curHash
		}
		if _, err := a.Repo.Commit(c.Branch, []repo.Change{ch}, author(u), fmt.Sprintf("Add %s to %s", itemID, coID)); err != nil {
			return err
		}
	}
	if _, err := a.DB.Exec(ctx, `INSERT INTO co_items (co_id, item_id, action, from_rev, to_rev) VALUES ($1, $2, $3, $4, $5)`, coID, itemID, action, from, to); err != nil {
		return err
	}
	a.DB.Exec(ctx, `UPDATE change_orders SET updated_at = now(), version = version + 1 WHERE id = $1`, coID)
	a.indexCO(ctx, coID)
	a.indexItem(ctx, a.DB, e)
	a.Audit(ctx, u, ip, "co.item.add", coID, map[string]any{"item": itemID, "action": action})
	a.Events.Publish(Event{Type: "record", ID: coID})
	a.Events.Publish(Event{Type: "record", ID: itemID})
	return nil
}

func (a *App) apiRemoveCOItem(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	c, err := a.coForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !a.coEditable(u, c) {
		return errForbidden
	}
	itemID := r.PathValue("item")
	var action string
	if err := a.DB.QueryRow(ctx, `SELECT action FROM co_items WHERE co_id = $1 AND item_id = $2`, c.ID, itemID).Scan(&action); err != nil {
		return err
	}
	// Edits made on the branch return to main so no work is lost.
	if e := cache.get(itemID); e != nil && action != "Obsolete" {
		if err := a.returnDraftToMain(ctx, u, c, e); err != nil {
			return err
		}
	}
	if _, err := a.DB.Exec(ctx, `DELETE FROM co_items WHERE co_id = $1 AND item_id = $2`, c.ID, itemID); err != nil {
		return err
	}
	a.indexCO(ctx, c.ID)
	if e := cache.get(itemID); e != nil {
		a.indexItem(ctx, a.DB, e)
	}
	a.Audit(ctx, u, clientIP(r), "co.item.remove", c.ID, map[string]any{"item": itemID})
	a.Events.Publish(Event{Type: "record", ID: c.ID})
	return ok(w, nil)
}

// returnDraftToMain copies an item's working draft from the change order
// branch back to main.
func (a *App) returnDraftToMain(ctx context.Context, u *User, c *coRecord, e *entry) error {
	p := a.draftPath(e)
	b, _, err := a.Repo.Read(c.Branch, p)
	if err != nil {
		return nil
	}
	_, mainHash, merr := a.Repo.Read(repo.Main, p)
	if merr == nil {
		mb, _ := a.Repo.Blob(mainHash)
		if string(mb) == string(b) {
			return nil
		}
	}
	ch := repo.Change{Path: p, Content: b}
	if merr == nil {
		ch.Expect = mainHash
	}
	if _, err := a.Repo.Commit(repo.Main, []repo.Change{ch}, author(u), fmt.Sprintf("Return %s draft from %s", e.Item.ID, c.ID)); err != nil {
		return err
	}
	return a.refresh(ctx, e.Item.ID, e.Item.Type, e.Item.Space)
}

// ---------------------------------------------------------------------------
// View, changes and validation

type coItemRow struct {
	Item     string `json:"item"`
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	Action   string `json:"action"`
	FromRev  string `json:"fromRev"`
	ToRev    string `json:"toRev"`
	Status   string `json:"status"`
	Hidden   bool   `json:"hidden,omitempty"`
	Space    string `json:"space"`
}

type approverRow struct {
	UserID    string     `json:"userId"`
	Username  string     `json:"username"`
	Name      string     `json:"name"`
	Decision  string     `json:"decision"`
	DecidedAt *time.Time `json:"decidedAt"`
	Comment   string     `json:"comment"`
}

type coPerms struct {
	Edit            bool `json:"edit"`
	ManageApprovers bool `json:"manageApprovers"`
	Submit          bool `json:"submit"`
	Withdraw        bool `json:"withdraw"`
	Decide          bool `json:"decide"`
	Release         bool `json:"release"`
	Cancel          bool `json:"cancel"`
	Lock            bool `json:"lock"`
}

func (a *App) coItems(ctx context.Context, coID string) ([]coItemRow, error) {
	rows, err := a.DB.Query(ctx, `SELECT item_id, action, from_rev, to_rev FROM co_items WHERE co_id = $1 ORDER BY item_id`, coID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []coItemRow
	for rows.Next() {
		var x coItemRow
		if err := rows.Scan(&x.Item, &x.Action, &x.FromRev, &x.ToRev); err != nil {
			return nil, err
		}
		if e := cache.get(x.Item); e != nil {
			x.Title, x.Kind, x.Status, x.Space = e.Item.Title, kindOf(e.Item), e.Item.Status, e.Item.Space
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (a *App) coApprovers(ctx context.Context, q querier, coID string) ([]approverRow, error) {
	rows, err := q.Query(ctx, `SELECT ap.user_id::text, us.username, us.display_name, ap.decision, ap.decided_at, ap.comment
		FROM co_approvers ap JOIN users us ON us.id = ap.user_id WHERE ap.co_id = $1 ORDER BY us.display_name`, coID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []approverRow{}
	for rows.Next() {
		var x approverRow
		if err := rows.Scan(&x.UserID, &x.Username, &x.Name, &x.Decision, &x.DecidedAt, &x.Comment); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (a *App) apiCO(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	c, err := a.coForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	items, err := a.coItems(ctx, c.ID)
	if err != nil {
		return err
	}
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	shown := []coItemRow{}
	for _, it := range items {
		if !vis(it.Item) {
			it = coItemRow{Item: "Restricted item", Hidden: true, Action: it.Action}
		}
		shown = append(shown, it)
	}
	appr, err := a.coApprovers(ctx, a.DB, c.ID)
	if err != nil {
		return err
	}
	issues, err := a.validateCO(ctx, c)
	if err != nil {
		return err
	}
	var visIssues []model.Issue
	for _, is := range issues {
		if vis(is.Item) {
			visIssues = append(visIssues, is)
		}
	}
	if visIssues == nil {
		visIssues = []model.Issue{}
	}
	p := coPerms{Edit: a.coEditable(u, c), ManageApprovers: (c.State == CODraft || c.State == COInReview || c.State == COApproved) && u.Library.CanManageApprovers}
	p.Submit = p.Edit
	p.Withdraw = (c.State == COInReview || c.State == COApproved) && (c.OwnerID == u.ID || u.Can(PermUsers))
	p.Cancel = (c.State == CODraft || c.State == COInReview || c.State == COApproved) && (c.OwnerID == u.ID || u.Can(PermUsers))
	p.Release = c.State == COApproved && u.Can(PermCORelease)
	p.Lock = c.OwnerID == u.ID || u.Can(PermUsers)
	for _, ap := range appr {
		if ap.UserID == u.ID && c.State == COInReview && ap.Decision == "" && u.Can(PermCOApprove) {
			p.Decide = true
		}
	}
	lock, _ := a.lockOf(ctx, a.DB, c.ID)
	lockRefs, _ := a.userRefs(ctx, lock)
	var watching bool
	a.DB.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM watchers WHERE record_id = $1 AND user_id = $2)`, c.ID, u.ID).Scan(&watching)
	var fixes []string
	rows, err := a.DB.Query(ctx, `SELECT src FROM links WHERE dst = $1 AND type = 'fixed by'`, c.ID)
	if err == nil {
		for rows.Next() {
			var s string
			if rows.Scan(&s) == nil && vis(s) {
				fixes = append(fixes, s)
			}
		}
		rows.Close()
	}
	a.DB.Exec(ctx, `INSERT INTO recent_views (user_id, record_id) VALUES ($1, $2) ON CONFLICT (user_id, record_id) DO UPDATE SET viewed_at = now()`, u.ID, c.ID)
	return ok(w, map[string]any{"co": c, "items": shown, "approvers": appr, "issues": visIssues, "perms": p, "lock": lockRefs, "watching": watching, "fixes": fixes})
}

// proposed returns the version of each affected item as it would be after
// release.
func (a *App) proposed(ctx context.Context, c *coRecord, items []coItemRow) (map[string]*model.Item, error) {
	out := map[string]*model.Item{}
	for _, x := range items {
		e := cache.get(x.Item)
		if e == nil {
			continue
		}
		var it *model.Item
		switch x.Action {
		case "Obsolete":
			it = e.Item.Clone()
			it.Status = model.StatusObsolete
		default:
			ref := c.Branch
			if c.State == COReleased {
				ref = c.Baseline
			}
			p := a.draftPath(e)
			if c.State == COReleased {
				p = e.Path
			}
			b, _, err := a.Repo.Read(ref, p)
			if err != nil {
				continue
			}
			if it, err = model.Parse(b); err != nil {
				return nil, err
			}
			it.Revision, it.Status = x.ToRev, model.StatusReleased
		}
		out[x.Item] = it
	}
	return out, nil
}

// validateCO checks the affected items as they would be released.
func (a *App) validateCO(ctx context.Context, c *coRecord) ([]model.Issue, error) {
	items, err := a.coItems(ctx, c.ID)
	if err != nil {
		return nil, err
	}
	issues := []model.Issue{}
	if len(items) == 0 {
		issues = append(issues, model.Issue{Level: "error", Message: "The change order has no affected items"})
		return issues, nil
	}
	prop, err := a.proposed(ctx, c, items)
	if err != nil {
		return nil, err
	}
	get := func(id string) *model.Item {
		if p, ok := prop[id]; ok {
			return p
		}
		return cache.lookup(id)
	}
	roles, err := a.fileRoleExpectations(ctx)
	if err != nil {
		return nil, err
	}
	for _, x := range items {
		it := prop[x.Item]
		if it == nil {
			issues = append(issues, model.Issue{Level: "error", Item: x.Item, Message: x.Item + " no longer exists"})
			continue
		}
		issues = append(issues, model.Validate(it, get)...)
		if x.Action == "Obsolete" {
			for _, p := range model.WhereUsed(x.Item, cache.all(model.TypePart)) {
				pp := get(p.ID)
				if pp.Status == model.StatusReleased {
					issues = append(issues, model.Issue{Level: "warning", Item: x.Item, Message: fmt.Sprintf("%s is still used in released assembly %s", x.Item, p.ID)})
				}
			}
			continue
		}
		have := map[string]bool{}
		for _, f := range it.Files {
			have[f.Role] = true
		}
		for _, role := range model.ExpectedFileRoles(x.Item, roles) {
			if !have[role] {
				issues = append(issues, model.Issue{Level: "warning", Item: x.Item, Field: "files", Message: fmt.Sprintf("%s has no %s file", x.Item, role)})
			}
		}
		if it.Type == model.TypeDocument {
			if len(it.Files) == 0 {
				issues = append(issues, model.Issue{Level: "error", Item: x.Item, Field: "files", Message: "A document needs its primary file before release"})
			}
		}
	}
	return issues, nil
}

type fieldChange struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

type itemChange struct {
	Item    string        `json:"item"`
	Title   string        `json:"title"`
	Action  string        `json:"action"`
	FromRev string        `json:"fromRev"`
	ToRev   string        `json:"toRev"`
	Fields  []fieldChange `json:"fields"`
	Lines   []DiffLine    `json:"lines"`
	Before  *model.Item   `json:"before"`
	After   *model.Item   `json:"after"`
}

// apiCOChanges shows every change the change order makes, compared with the
// current released data.
func (a *App) apiCOChanges(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	c, err := a.coForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	items, err := a.coItems(ctx, c.ID)
	if err != nil {
		return err
	}
	prop, err := a.proposed(ctx, c, items)
	if err != nil {
		return err
	}
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	out := []itemChange{}
	for _, x := range items {
		if !vis(x.Item) {
			continue
		}
		after := prop[x.Item]
		var before *model.Item
		if x.FromRev != "" {
			if c.State == COReleased {
				before, _ = a.itemAtRevision(ctx, x.Item, x.FromRev)
			} else if e := cache.get(x.Item); e != nil {
				before = e.Item
			}
		}
		ch := itemChange{Item: x.Item, Title: x.Title, Action: x.Action, FromRev: x.FromRev, ToRev: x.ToRev, Before: before, After: after, Fields: []fieldChange{}}
		var bb, ab []byte
		if before != nil {
			bb, _ = before.Marshal()
		}
		if after != nil {
			ab, _ = after.Marshal()
		}
		ch.Lines = lineDiff(string(bb), string(ab))
		ch.Fields = fieldChanges(before, after)
		out = append(out, ch)
	}
	return ok(w, out)
}

// fieldChanges summarizes changed frontmatter fields.
func fieldChanges(before, after *model.Item) []fieldChange {
	out := []fieldChange{}
	if after == nil {
		return out
	}
	if before == nil {
		before = &model.Item{}
	}
	add := func(name, a, b string) {
		if a != b {
			out = append(out, fieldChange{name, a, b})
		}
	}
	add("Title", before.Title, after.Title)
	add("Revision", before.Revision, after.Revision)
	add("Status", before.Status, after.Status)
	add("Owner", before.Owner, after.Owner)
	add("Description", before.Description, after.Description)
	add("Statement", before.Statement, after.Statement)
	add("Unit", before.Unit, after.Unit)
	add("Lifecycle", before.Lifecycle, after.Lifecycle)
	bomStr := func(it *model.Item) string {
		var lines []string
		for _, l := range it.BOM {
			lines = append(lines, fmt.Sprintf("%d %s x%g %s", l.Find, l.Item, l.Qty, strings.Join(l.RefDes, " ")))
		}
		return strings.Join(lines, "\n")
	}
	add("BOM", bomStr(before), bomStr(after))
	fileStr := func(it *model.Item) string {
		var lines []string
		for _, f := range it.Files {
			lines = append(lines, f.Role+": "+f.Name)
		}
		sort.Strings(lines)
		return strings.Join(lines, "\n")
	}
	add("Files", fileStr(before), fileStr(after))
	linkStr := func(it *model.Item) string {
		var lines []string
		for _, l := range it.Links {
			lines = append(lines, l.Type+" "+l.To)
		}
		sort.Strings(lines)
		return strings.Join(lines, "\n")
	}
	add("Links", linkStr(before), linkStr(after))
	add("Body", before.Body, after.Body)
	return out
}

func (a *App) apiCOValidation(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	c, err := a.coForUser(r.Context(), u, r.PathValue("id"))
	if err != nil {
		return err
	}
	is, err := a.validateCO(r.Context(), c)
	if err != nil {
		return err
	}
	return ok(w, is)
}

// ---------------------------------------------------------------------------
// Workflow

func (a *App) apiSetApprovers(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	c, err := a.coForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !u.Library.CanManageApprovers || !(c.State == CODraft || c.State == COInReview || c.State == COApproved) {
		return errForbidden
	}
	var in struct {
		Approvers []string `json:"approvers"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	if err := a.setApprovers(ctx, u, clientIP(r), c.ID, in.Approvers); err != nil {
		return err
	}
	return ok(w, nil)
}

func (a *App) setApprovers(ctx context.Context, u *User, ip, coID string, ids []string) error {
	return a.tx(ctx, func(tx pgx.Tx) error {
		c, err := a.loadCO(ctx, tx, coID)
		if err != nil {
			return err
		}
		cur, err := a.coApprovers(ctx, tx, coID)
		if err != nil {
			return err
		}
		want := map[string]bool{}
		for _, id := range ids {
			want[id] = true
		}
		var added []string
		for id := range want {
			found := false
			for _, ap := range cur {
				found = found || ap.UserID == id
			}
			if found {
				continue
			}
			au, err := a.loadUser(ctx, id)
			if err != nil {
				return badRequest("Unknown approver.")
			}
			if !au.Can(PermCOApprove) {
				return badRequest("%s cannot approve change orders.", au.DisplayName)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO co_approvers (co_id, user_id) VALUES ($1, $2)`, coID, id); err != nil {
				return err
			}
			added = append(added, id)
		}
		for _, ap := range cur {
			if !want[ap.UserID] {
				if ap.Decision != "" && c.State != CODraft {
					return badRequest("%s has already decided and cannot be removed.", ap.Name)
				}
				if _, err := tx.Exec(ctx, `DELETE FROM co_approvers WHERE co_id = $1 AND user_id = $2`, coID, ap.UserID); err != nil {
					return err
				}
			}
		}
		if c.State != CODraft && len(want) > 0 {
			if err := a.checkApproverRule(ctx, tx, c); err != nil {
				return err
			}
		}
		// A new approver on an approved change order reopens the review.
		if c.State == COApproved && len(added) > 0 {
			if _, err := tx.Exec(ctx, `UPDATE change_orders SET state = 'In Review' WHERE id = $1`, coID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE change_orders SET updated_at = now(), version = version + 1 WHERE id = $1`, coID); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, ip, "co.approvers", coID, map[string]any{"approvers": ids})
		if c.State != CODraft {
			for _, id := range added {
				a.notifyQ(ctx, tx, u, id, "Approval requested", coID, fmt.Sprintf("%s asks for your approval of %s %s", u.DisplayName, coID, c.Title))
			}
		}
		if c.State == COInReview || c.State == COApproved {
			return a.settleCO(ctx, tx, u, ip, coID)
		}
		return nil
	})
}

// checkApproverRule enforces that the author is not the only approver.
func (a *App) checkApproverRule(ctx context.Context, q querier, c *coRecord) error {
	appr, err := a.coApprovers(ctx, q, c.ID)
	if err != nil {
		return err
	}
	if len(appr) == 0 {
		return badRequest("Add at least one approver.")
	}
	others := 0
	for _, ap := range appr {
		if ap.UserID != c.OwnerID {
			others++
		}
	}
	if others == 0 {
		return badRequest("The author cannot be the only approver.")
	}
	return nil
}

func (a *App) apiSubmitCO(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	c, err := a.coForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !a.coEditable(u, c) {
		return errForbidden
	}
	if strings.TrimSpace(c.Reason) == "" || strings.TrimSpace(c.Description) == "" {
		return badRequest("Reason for change and description of change are required before review.")
	}
	items, err := a.coItems(ctx, c.ID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return badRequest("Add at least one affected item.")
	}
	err = a.tx(ctx, func(tx pgx.Tx) error {
		if err := a.checkApproverRule(ctx, tx, c); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE change_orders SET state = 'In Review', updated_at = now(), version = version + 1 WHERE id = $1 AND state = 'Draft'`, c.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE co_approvers SET decision = '', decided_at = NULL, comment = '' WHERE co_id = $1`, c.ID); err != nil {
			return err
		}
		appr, err := a.coApprovers(ctx, tx, c.ID)
		if err != nil {
			return err
		}
		for _, ap := range appr {
			a.notifyQ(ctx, tx, u, ap.UserID, "Approval requested", c.ID, fmt.Sprintf("%s asks for your approval of %s %s", u.DisplayName, c.ID, c.Title))
		}
		a.auditQ(ctx, tx, u, clientIP(r), "co.submit", c.ID, nil)
		return nil
	})
	if err != nil {
		return err
	}
	a.afterCOStateChange(ctx, c.ID, items)
	return ok(w, nil)
}

func (a *App) afterCOStateChange(ctx context.Context, coID string, items []coItemRow) {
	a.indexCO(ctx, coID)
	for _, x := range items {
		if e := cache.get(x.Item); e != nil {
			a.indexItem(ctx, a.DB, e)
			a.Events.Publish(Event{Type: "record", ID: x.Item})
		}
	}
	a.Events.Publish(Event{Type: "record", ID: coID})
}

func (a *App) apiWithdrawCO(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	c, err := a.coForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !((c.State == COInReview || c.State == COApproved) && (c.OwnerID == u.ID || u.Can(PermUsers))) {
		return errForbidden
	}
	if _, err := a.DB.Exec(ctx, `UPDATE change_orders SET state = 'Draft', updated_at = now(), version = version + 1 WHERE id = $1`, c.ID); err != nil {
		return err
	}
	a.DB.Exec(ctx, `UPDATE co_approvers SET decision = '', decided_at = NULL, comment = '' WHERE co_id = $1`, c.ID)
	a.Audit(ctx, u, clientIP(r), "co.withdraw", c.ID, nil)
	items, _ := a.coItems(ctx, c.ID)
	a.afterCOStateChange(ctx, c.ID, items)
	return ok(w, nil)
}

// apiDecideCO records an approver's decision with an electronic signature.
func (a *App) apiDecideCO(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	c, err := a.coForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	var in struct {
		Decision string `json:"decision"`
		Password string `json:"password"`
		Comment  string `json:"comment"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	if in.Decision != "Approved" && in.Decision != "Rejected" {
		return badRequest("Decision must be Approved or Rejected.")
	}
	if c.State != COInReview {
		return badRequest("%s is not in review.", c.ID)
	}
	if !u.Can(PermCOApprove) {
		return errForbidden
	}
	if in.Decision == "Rejected" && strings.TrimSpace(in.Comment) == "" {
		return badRequest("A comment is required when rejecting.")
	}
	if in.Decision == "Approved" {
		issues, err := a.validateCO(ctx, c)
		if err != nil {
			return err
		}
		if errs := errorsOnly(issues); len(errs) > 0 {
			return &Error{Status: http.StatusBadRequest, Code: "validation", Message: "This change order cannot be approved while validation errors exist.", Data: errs}
		}
	}
	if !a.checkUserPassword(ctx, u.ID, in.Password) {
		a.Audit(ctx, u, clientIP(r), "signature.failed", c.ID, nil)
		return &Error{Status: http.StatusUnauthorized, Code: "signature", Message: "Password is incorrect. The decision was not recorded."}
	}
	err = a.tx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE co_approvers SET decision = $3, decided_at = now(), comment = $4 WHERE co_id = $1 AND user_id = $2 AND decision = ''`,
			c.ID, u.ID, in.Decision, strings.TrimSpace(in.Comment))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return badRequest("You are not an approver of %s, or you have already decided.", c.ID)
		}
		a.auditQ(ctx, tx, u, clientIP(r), "co.decision", c.ID, map[string]any{"decision": in.Decision, "comment": in.Comment, "signature": "password"})
		a.notifyQ(ctx, tx, u, c.OwnerID, "Approval decided", c.ID, fmt.Sprintf("%s %s %s", u.DisplayName, strings.ToLower(in.Decision), c.ID))
		return a.settleCO(ctx, tx, u, clientIP(r), c.ID)
	})
	if err != nil {
		return err
	}
	items, _ := a.coItems(ctx, c.ID)
	a.afterCOStateChange(ctx, c.ID, items)
	return ok(w, nil)
}

// settleCO moves a change order in review to Approved when every approver
// has approved, or Rejected when any approver rejected.
func (a *App) settleCO(ctx context.Context, tx pgx.Tx, u *User, ip, coID string) error {
	c, err := a.loadCO(ctx, tx, coID)
	if err != nil {
		return err
	}
	if c.State != COInReview && c.State != COApproved {
		return nil
	}
	appr, err := a.coApprovers(ctx, tx, coID)
	if err != nil {
		return err
	}
	if len(appr) == 0 {
		return nil
	}
	approved, rejected := 0, 0
	for _, ap := range appr {
		switch ap.Decision {
		case "Approved":
			approved++
		case "Rejected":
			rejected++
		}
	}
	next := COInReview
	switch {
	case rejected > 0:
		next = CORejected
	case approved == len(appr):
		next = COApproved
	}
	if next == c.State {
		return nil
	}
	if _, err := tx.Exec(ctx, `UPDATE change_orders SET state = $2, updated_at = now(), version = version + 1 WHERE id = $1`, coID, next); err != nil {
		return err
	}
	a.auditQ(ctx, tx, u, ip, "co.state", coID, map[string]any{"from": c.State, "to": next})
	a.notifyWatchersQ(ctx, tx, u, coID, "Status changed", fmt.Sprintf("%s is now %s", coID, next))
	if next == COApproved {
		rows, err := tx.Query(ctx, `SELECT us.id::text FROM users us JOIN roles ro ON ro.id = us.library_role_id WHERE us.active AND $1 = ANY(ro.permissions)`, PermCORelease)
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
				if id != u.ID {
					a.notifyQ(ctx, tx, u, id, "Approval requested", coID, fmt.Sprintf("%s is approved and ready to release", coID))
				}
			}
		}
	}
	if next == CORejected {
		return a.closeCOItems(ctx, tx, u, c)
	}
	return nil
}

// closeCOItems applies the rejected or cancelled change order setting.
func (a *App) closeCOItems(ctx context.Context, tx pgx.Tx, u *User, c *coRecord) error {
	var toDraft bool
	a.Setting("co_reject_returns_to_draft", &toDraft)
	if !toDraft {
		return nil
	}
	items, err := a.coItems(ctx, c.ID)
	if err != nil {
		return err
	}
	for _, x := range items {
		if e := cache.get(x.Item); e != nil && x.Action != "Obsolete" {
			if err := a.returnDraftToMain(ctx, u, c, e); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *App) apiCancelCO(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	c, err := a.coForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !((c.State == CODraft || c.State == COInReview || c.State == COApproved) && (c.OwnerID == u.ID || u.Can(PermUsers))) {
		return errForbidden
	}
	err = a.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `UPDATE change_orders SET state = 'Cancelled', updated_at = now(), version = version + 1 WHERE id = $1`, c.ID); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, clientIP(r), "co.cancel", c.ID, nil)
		a.notifyWatchersQ(ctx, tx, u, c.ID, "Status changed", fmt.Sprintf("%s was cancelled", c.ID))
		return a.closeCOItems(ctx, tx, u, c)
	})
	if err != nil {
		return err
	}
	items, _ := a.coItems(ctx, c.ID)
	a.afterCOStateChange(ctx, c.ID, items)
	return ok(w, nil)
}

// apiReleaseCO is the Release Authority's final sign-off.
func (a *App) apiReleaseCO(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	c, err := a.coForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if !u.Can(PermCORelease) {
		return errForbidden
	}
	var in struct {
		Password string `json:"password"`
		Comment  string `json:"comment"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	if !a.checkUserPassword(ctx, u.ID, in.Password) {
		a.Audit(ctx, u, clientIP(r), "signature.failed", c.ID, nil)
		return &Error{Status: http.StatusUnauthorized, Code: "signature", Message: "Password is incorrect. The change order was not released."}
	}
	if err := a.ReleaseCO(ctx, u, clientIP(r), c.ID, in.Comment); err != nil {
		return err
	}
	return ok(w, nil)
}

// ReleaseCO merges the change order into main, updates revisions and
// creates the baseline tag.
func (a *App) ReleaseCO(ctx context.Context, u *User, ip, coID, comment string) error {
	c, err := a.loadCO(ctx, a.DB, coID)
	if err != nil {
		return err
	}
	if c.State != COApproved {
		return badRequest("%s must be approved before release.", coID)
	}
	issues, err := a.validateCO(ctx, c)
	if err != nil {
		return err
	}
	if errs := errorsOnly(issues); len(errs) > 0 {
		return &Error{Status: http.StatusBadRequest, Code: "validation", Message: "Validation errors must be fixed before release.", Data: errs}
	}
	items, err := a.coItems(ctx, coID)
	if err != nil {
		return err
	}
	prop, err := a.proposed(ctx, c, items)
	if err != nil {
		return err
	}
	// Write the released files on the branch, then merge into main.
	var changes []repo.Change
	var paths []string
	for _, x := range items {
		e := cache.get(x.Item)
		it := prop[x.Item]
		if e == nil || it == nil {
			return badRequest("%s no longer exists.", x.Item)
		}
		b, err := it.Marshal()
		if err != nil {
			return err
		}
		changes = append(changes, repo.Change{Path: e.Path, Content: b})
		paths = append(paths, e.Path)
		if e.Item.Status != model.StatusDraft {
			next := model.NextPath(e.Path)
			if _, _, err := a.Repo.Read(c.Branch, next); err == nil {
				changes = append(changes, repo.Change{Path: next, Delete: true})
			}
			paths = append(paths, next)
		}
	}
	if _, err := a.Repo.Commit(c.Branch, changes, author(u), fmt.Sprintf("Prepare release of %s", coID)); err != nil {
		return err
	}
	msg := fmt.Sprintf("Release %s %s", coID, c.Title)
	if strings.TrimSpace(comment) != "" {
		msg += "\n\n" + strings.TrimSpace(comment)
	}
	merge, err := a.Repo.Merge(repo.Main, c.Branch, paths, author(u), msg)
	if err != nil {
		return err
	}
	baseline := "baseline/" + coID
	if err := a.Repo.Tag(baseline, merge, author(u), fmt.Sprintf("Baseline %s %s", coID, c.Title), a.signingKey()); err != nil {
		return err
	}
	now := time.Now()
	err = a.tx(ctx, func(tx pgx.Tx) error {
		for _, x := range items {
			switch x.Action {
			case "Obsolete":
				if _, err := tx.Exec(ctx, `INSERT INTO item_revisions (item_id, revision, status, co_id, commit_hash, released_at) VALUES ($1, $2, 'Obsolete', $3, $4, $5)
					ON CONFLICT (item_id, revision) DO UPDATE SET status = 'Obsolete'`, x.Item, x.ToRev, coID, merge, now); err != nil {
					return err
				}
			default:
				if _, err := tx.Exec(ctx, `UPDATE item_revisions SET status = 'Superseded' WHERE item_id = $1 AND status = 'Released'`, x.Item); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `INSERT INTO item_revisions (item_id, revision, status, co_id, commit_hash, released_at) VALUES ($1, $2, 'Released', $3, $4, $5)`,
					x.Item, x.ToRev, coID, merge, now); err != nil {
					return err
				}
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE change_orders SET state = 'Released', released_by = $2, released_at = $3, baseline = $4, updated_at = now(), version = version + 1 WHERE id = $1`,
			coID, u.ID, now, baseline); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, ip, "co.release", coID, map[string]any{"commit": merge, "baseline": baseline, "signature": "password"})
		a.notifyWatchersQ(ctx, tx, u, coID, "Status changed", fmt.Sprintf("%s was released", coID))
		return nil
	})
	if err != nil {
		return err
	}
	a.Repo.DeleteBranch(c.Branch)
	for _, x := range items {
		e := cache.get(x.Item)
		if e != nil {
			if err := a.refresh(ctx, x.Item, e.Item.Type, e.Item.Space); err != nil {
				return err
			}
			a.notifyWatchers(ctx, u, x.Item, "Status changed", fmt.Sprintf("%s rev %s released by %s", x.Item, x.ToRev, coID))
		}
	}
	a.afterCOStateChange(ctx, coID, items)
	return nil
}

// apiCOHistory lists the audit entries of a change order.
func (a *App) apiCOHistory(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	c, err := a.coForUser(ctx, u, r.PathValue("id"))
	if err != nil {
		return err
	}
	rows, err := a.DB.Query(ctx, `SELECT id, at, username, action, record_id, detail, ip FROM audit_log WHERE record_id = $1 ORDER BY id DESC LIMIT 500`, c.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []auditEntry{}
	for rows.Next() {
		var e auditEntry
		if err := rows.Scan(&e.ID, &e.At, &e.Username, &e.Action, &e.RecordID, &e.Detail, &e.IP); err != nil {
			return err
		}
		e.IP = ""
		out = append(out, e)
	}
	return ok(w, out)
}

// ---------------------------------------------------------------------------
// Baselines

func (a *App) apiBaselines(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermLibView) {
		return errForbidden
	}
	rows, err := a.DB.Query(r.Context(), `SELECT c.id, c.title, c.baseline, c.released_at, us.display_name FROM change_orders c JOIN users us ON us.id = c.released_by
		WHERE c.state = 'Released' ORDER BY c.released_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type bl struct {
		CO         string    `json:"co"`
		Title      string    `json:"title"`
		Baseline   string    `json:"baseline"`
		ReleasedAt time.Time `json:"releasedAt"`
		ReleasedBy string    `json:"releasedBy"`
	}
	out := []bl{}
	for rows.Next() {
		var b bl
		if err := rows.Scan(&b.CO, &b.Title, &b.Baseline, &b.ReleasedAt, &b.ReleasedBy); err != nil {
			return err
		}
		out = append(out, b)
	}
	return ok(w, out)
}

// apiBaseline lists every released engineering item in a baseline, exactly
// as it was released, and optionally compares it with another baseline.
func (a *App) apiBaseline(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	if !u.Can(PermLibView) {
		return errForbidden
	}
	name := "baseline/" + r.PathValue("co")
	read := func(ref string) (map[string]*model.Item, error) {
		files, err := a.Repo.List(ref, "items/")
		if err != nil {
			return nil, err
		}
		out := map[string]*model.Item{}
		for _, f := range files {
			if strings.HasSuffix(f.Path, ".next.md") {
				continue
			}
			b, err := a.Repo.Blob(f.Hash)
			if err != nil {
				return nil, err
			}
			it, err := model.Parse(b)
			if err != nil || (it.Status != model.StatusReleased && it.Status != model.StatusObsolete) {
				continue
			}
			out[it.ID] = it
		}
		return out, nil
	}
	cur, err := read(name)
	if err != nil {
		return errNotFound
	}
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	type row struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Revision string `json:"revision"`
		Status   string `json:"status"`
		Change   string `json:"change,omitempty"` // added, removed, changed (comparison)
		Other    string `json:"otherRevision,omitempty"`
	}
	var out []row
	if other := r.URL.Query().Get("compare"); other != "" {
		prev, err := read("baseline/" + other)
		if err != nil {
			return errNotFound
		}
		ids := map[string]bool{}
		for id := range cur {
			ids[id] = true
		}
		for id := range prev {
			ids[id] = true
		}
		for id := range ids {
			if !vis(id) {
				continue
			}
			c, p := cur[id], prev[id]
			switch {
			case p == nil:
				out = append(out, row{ID: id, Title: c.Title, Revision: c.Revision, Status: c.Status, Change: "added"})
			case c == nil:
				out = append(out, row{ID: id, Title: p.Title, Other: p.Revision, Status: p.Status, Change: "removed"})
			case c.Revision != p.Revision || c.Status != p.Status:
				out = append(out, row{ID: id, Title: c.Title, Revision: c.Revision, Status: c.Status, Change: "changed", Other: p.Revision})
			}
		}
	} else {
		for id, it := range cur {
			if vis(id) {
				out = append(out, row{ID: id, Title: it.Title, Revision: it.Revision, Status: it.Status})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if out == nil {
		out = []row{}
	}
	return ok(w, out)
}
