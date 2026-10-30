package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"designhub/internal/model"
)

type searchResult struct {
	ID       string    `json:"id"`
	Kind     string    `json:"kind"`
	Title    string    `json:"title"`
	Status   string    `json:"status"`
	Revision string    `json:"revision"`
	Space    string    `json:"space"`
	Snippet  string    `json:"snippet"`
	Updated  time.Time `json:"updated"`
}

// apiSearch runs a full-text search with filters. The snippet marks matches
// with \x01 and \x02 so the browser can highlight them without HTML.
func (a *App) apiSearch(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	q := r.URL.Query()
	text := strings.TrimSpace(q.Get("q"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	resp := map[string]any{}
	// An exact identifier opens that record.
	if text != "" {
		up := strings.ToUpper(text)
		for _, cand := range []string{text, up} {
			var kind string
			if err := a.DB.QueryRow(ctx, `SELECT kind FROM records WHERE id = $1`, cand).Scan(&kind); err == nil && vis(cand) {
				resp["exact"] = map[string]string{"id": cand, "kind": kind}
				break
			}
		}
	}
	where := []string{"true"}
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	rank := "0"
	snippet := "left(r.body, 200)"
	if text != "" {
		tq := arg(text)
		like := arg("%" + text + "%")
		where = append(where, fmt.Sprintf("(r.search @@ websearch_to_tsquery('english', %s) OR r.search @@ websearch_to_tsquery('simple', %s) OR r.id ILIKE %s OR r.title ILIKE %s)", tq, tq, like, like))
		rank = fmt.Sprintf("ts_rank(r.search, websearch_to_tsquery('english', %s)) + CASE WHEN r.id ILIKE %s THEN 10 WHEN r.title ILIKE %s THEN 1 ELSE 0 END", tq, like, like)
		snippet = fmt.Sprintf("ts_headline('english', r.body, websearch_to_tsquery('english', %s), 'StartSel=%s, StopSel=%s, MaxWords=30, MinWords=10, ShortWord=2, MaxFragments=1')", tq, "\x01", "\x02")
	}
	if s := q.Get("space"); s != "" {
		where = append(where, "r.space_id = "+arg(s))
	}
	if k := q.Get("kind"); k != "" {
		where = append(where, "r.kind = ANY("+arg(strings.Split(k, ","))+")")
	}
	if s := q.Get("status"); s != "" {
		where = append(where, "r.status = ANY("+arg(strings.Split(s, ","))+")")
	}
	if a := q.Get("author"); a != "" {
		where = append(where, "r.owner_id = "+arg(a)+"::uuid")
	}
	if l := q.Get("label"); l != "" {
		where = append(where, arg(l)+" = ANY(r.labels)")
	}
	if text == "" && len(where) == 1 {
		resp["results"] = []searchResult{}
		return ok(w, resp)
	}
	sql := fmt.Sprintf(`SELECT r.id, r.kind, r.title, r.status, r.revision, r.space_id, %s, r.updated_at FROM records r WHERE %s ORDER BY %s DESC, r.updated_at DESC LIMIT %d`,
		snippet, strings.Join(where, " AND "), rank, limit*4)
	rows, err := a.DB.Query(ctx, sql, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []searchResult{}
	for rows.Next() {
		var s searchResult
		if err := rows.Scan(&s.ID, &s.Kind, &s.Title, &s.Status, &s.Revision, &s.Space, &s.Snippet, &s.Updated); err != nil {
			return err
		}
		if !vis(s.ID) {
			continue
		}
		if !strings.Contains(s.Snippet, "\x01") {
			s.Snippet = truncate(firstLine(s.Snippet), 200)
		}
		out = append(out, s)
		if len(out) >= limit {
			break
		}
	}
	resp["results"] = out
	return ok(w, resp)
}

// ---------------------------------------------------------------------------
// Saved filters and searches

func (a *App) apiFilters(w http.ResponseWriter, r *http.Request) error {
	rows, err := a.DB.Query(r.Context(), `SELECT id::text, name, filter FROM saved_filters WHERE user_id = $1 AND view = $2 ORDER BY name`, userFrom(r).ID, r.URL.Query().Get("view"))
	if err != nil {
		return err
	}
	defer rows.Close()
	type f struct {
		ID     string          `json:"id"`
		Name   string          `json:"name"`
		Filter json.RawMessage `json:"filter"`
	}
	out := []f{}
	for rows.Next() {
		var x f
		if err := rows.Scan(&x.ID, &x.Name, &x.Filter); err != nil {
			return err
		}
		out = append(out, x)
	}
	return ok(w, out)
}

func (a *App) apiSaveFilter(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		View   string          `json:"view"`
		Name   string          `json:"name"`
		Filter json.RawMessage `json:"filter"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Name) == "" || in.View == "" {
		return badRequest("A name is required.")
	}
	var id string
	if err := a.DB.QueryRow(r.Context(), `INSERT INTO saved_filters (user_id, view, name, filter) VALUES ($1, $2, $3, $4) RETURNING id::text`,
		userFrom(r).ID, in.View, strings.TrimSpace(in.Name), []byte(in.Filter)).Scan(&id); err != nil {
		return err
	}
	return ok(w, map[string]string{"id": id})
}

func (a *App) apiDeleteFilter(w http.ResponseWriter, r *http.Request) error {
	if _, err := a.DB.Exec(r.Context(), `DELETE FROM saved_filters WHERE id = $1 AND user_id = $2`, r.PathValue("id"), userFrom(r).ID); err != nil {
		return err
	}
	return ok(w, nil)
}

// ---------------------------------------------------------------------------
// Home

func (a *App) apiHome(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	out := map[string]any{}
	myWork, err := a.queryWork(ctx, u, `w.assignee_id = $1 AND w.status NOT IN ('Done', 'Cancelled', 'Closed')`, u.ID)
	if err != nil {
		return err
	}
	sortMyWork(myWork)
	out["assigned"] = myWork
	awaiting, err := a.listCOs(ctx, u, "awaiting", "")
	if err != nil {
		return err
	}
	out["approvals"] = awaiting
	if u.Can(PermCORelease) {
		rel, err := a.listCOs(ctx, u, "release", "")
		if err != nil {
			return err
		}
		out["toRelease"] = rel
	}
	ncrSign := []map[string]string{}
	rows, err := a.DB.Query(ctx, `SELECT w.id, w.title FROM ncr_approvals ap JOIN work_items w ON w.id = ap.ncr_id WHERE ap.user_id = $1 AND ap.decision = '' AND w.status = 'Reported'`, u.ID)
	if err == nil {
		for rows.Next() {
			var id, t string
			if rows.Scan(&id, &t) == nil && vis(id) {
				ncrSign = append(ncrSign, map[string]string{"id": id, "title": t})
			}
		}
		rows.Close()
	}
	out["dispositions"] = ncrSign
	mine := []recentChange{}
	rows, err = a.DB.Query(ctx, `SELECT id, kind, title, status, updated_at FROM records WHERE owner_id = $1 AND origin = 'git' AND status IN ('Draft', 'In Review') ORDER BY updated_at DESC LIMIT 30`, u.ID)
	if err == nil {
		for rows.Next() {
			var c recentChange
			if rows.Scan(&c.ID, &c.Kind, &c.Title, &c.Status, &c.Updated) == nil && vis(c.ID) && c.Kind != "page" {
				mine = append(mine, c)
			}
		}
		rows.Close()
	}
	// Released items with a draft revision in progress also count as mine.
	for _, it := range cache.all("") {
		if e := cache.get(it.ID); e != nil && e.Next != nil && strings.EqualFold(it.Owner, u.Username) && it.Type != model.TypePage {
			mine = append(mine, recentChange{ID: it.ID, Kind: kindOf(it), Title: it.Title, Status: "Draft rev " + e.Next.Revision})
		}
	}
	out["myItems"] = mine
	recent := []recentChange{}
	rows, err = a.DB.Query(ctx, `SELECT r.id, r.kind, r.title, r.status, v.viewed_at FROM recent_views v JOIN records r ON r.id = v.record_id
		WHERE v.user_id = $1 ORDER BY v.viewed_at DESC LIMIT 15`, u.ID)
	if err == nil {
		for rows.Next() {
			var c recentChange
			if rows.Scan(&c.ID, &c.Kind, &c.Title, &c.Status, &c.Updated) == nil && vis(c.ID) {
				recent = append(recent, c)
			}
		}
		rows.Close()
	}
	out["recent"] = recent
	var spaces []string
	for s := range u.Spaces {
		spaces = append(spaces, s)
	}
	activity := []recentChange{}
	rows, err = a.DB.Query(ctx, `SELECT id, kind, title, status, updated_at FROM records WHERE space_id = ANY($1) ORDER BY updated_at DESC LIMIT 60`, spaces)
	if err == nil {
		for rows.Next() {
			var c recentChange
			if rows.Scan(&c.ID, &c.Kind, &c.Title, &c.Status, &c.Updated) == nil && vis(c.ID) && len(activity) < 20 {
				activity = append(activity, c)
			}
		}
		rows.Close()
	}
	out["activity"] = activity
	notes := []notificationRow{}
	rows, err = a.DB.Query(ctx, `SELECT n.id, n.event, n.record_id, COALESCE(rc.title, ''), COALESCE(rc.kind, ''), COALESCE(us.display_name, 'DesignHub'), n.summary, n.created_at, n.read
		FROM notifications n LEFT JOIN users us ON us.id = n.triggered_by LEFT JOIN records rc ON rc.id = n.record_id
		WHERE n.recipient_id = $1 AND NOT n.read AND NOT n.dismissed ORDER BY n.created_at DESC LIMIT 10`, u.ID)
	if err == nil {
		for rows.Next() {
			var n notificationRow
			if rows.Scan(&n.ID, &n.Event, &n.RecordID, &n.RecordTitle, &n.RecordKind, &n.By, &n.Summary, &n.CreatedAt, &n.Read) == nil && vis(n.RecordID) {
				notes = append(notes, n)
			}
		}
		rows.Close()
	}
	out["notifications"] = notes
	return ok(w, out)
}
