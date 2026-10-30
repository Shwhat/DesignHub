package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"designhub/internal/model"
	"designhub/internal/repo"
)

// recordInfo resolves any record id to its kind, title, space and owner.
type recordInfo struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Space   string `json:"space"`
	OwnerID string `json:"ownerId"`
}

func (a *App) recordInfo(ctx context.Context, id string) (*recordInfo, error) {
	ri := &recordInfo{ID: id}
	var owner *string
	err := a.DB.QueryRow(ctx, `SELECT kind, title, status, space_id, owner_id::text FROM records WHERE id = $1`, id).Scan(&ri.Kind, &ri.Title, &ri.Status, &ri.Space, &owner)
	if err != nil {
		return nil, errNotFound
	}
	if owner != nil {
		ri.OwnerID = *owner
	}
	return ri, nil
}

// ---------------------------------------------------------------------------
// Watch and lock

func (a *App) apiWatch(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	if !a.canSee(ctx, u, id) {
		return errNotFound
	}
	var in struct {
		Watch bool `json:"watch"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	var err error
	if in.Watch {
		_, err = a.DB.Exec(ctx, `INSERT INTO watchers (record_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, id, u.ID)
	} else {
		_, err = a.DB.Exec(ctx, `DELETE FROM watchers WHERE record_id = $1 AND user_id = $2`, id, u.ID)
	}
	if err != nil {
		return err
	}
	return ok(w, map[string]bool{"watching": in.Watch})
}

// apiLock sets a record's access list. An empty list unlocks it. The user
// making the change is always kept on the list.
func (a *App) apiLock(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	if !a.canSee(ctx, u, id) {
		return errNotFound
	}
	ri, err := a.recordInfo(ctx, id)
	if err != nil {
		return err
	}
	isOwner := ri.OwnerID == u.ID
	canLock := isOwner || u.Can(PermUsers)
	if ri.Kind == "page" || ri.Kind == "task" || ri.Kind == "issue" || ri.Kind == "action" || ri.Kind == "epic" || ri.Kind == "ncr" {
		canLock = canLock || u.CanIn(ri.Space, PermSpaceAdmin)
	}
	if !canLock {
		return errForbidden
	}
	var in struct {
		Users []string `json:"users"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	users := []string{}
	for _, x := range in.Users {
		if x != "" && !contains(users, x) {
			users = append(users, x)
		}
	}
	if len(users) > 0 && !contains(users, u.ID) {
		users = append(users, u.ID)
	}
	err = a.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM record_access WHERE record_id = $1`, id); err != nil {
			return err
		}
		for _, x := range users {
			if _, err := tx.Exec(ctx, `INSERT INTO record_access (record_id, user_id) VALUES ($1, $2)`, id, x); err != nil {
				return badRequest("Unknown user in access list.")
			}
		}
		a.auditQ(ctx, tx, u, clientIP(r), "record.lock", id, map[string]any{"users": users})
		return nil
	})
	if err != nil {
		return err
	}
	a.Events.Publish(Event{Type: "record", ID: id})
	return ok(w, nil)
}

// ---------------------------------------------------------------------------
// Backlinks

type backlink struct {
	Src   string `json:"src"`
	Type  string `json:"type"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Rev   string `json:"rev,omitempty"`
}

var reverseNames = map[string]string{
	"contains": "used in", "satisfies": "satisfied by", "verifies": "verified by", "derived from": "derives", "affects": "affected by",
	"found on": "has finding", "fixed by": "fixes", "blocks": "blocked by", "duplicates": "duplicated by", "replaced by": "replaces",
	"relates to": "relates to", "applies to": "has procedure or document",
}

func (a *App) backlinks(ctx context.Context, u *User, id string) ([]backlink, error) {
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return nil, err
	}
	rows, err := a.DB.Query(ctx, `SELECT l.src, l.type, l.dst_rev, COALESCE(r.kind, ''), COALESCE(r.title, '') FROM links l LEFT JOIN records r ON r.id = l.src
		WHERE l.dst = $1 ORDER BY l.type, l.src`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []backlink{}
	for rows.Next() {
		var b backlink
		if err := rows.Scan(&b.Src, &b.Type, &b.Rev, &b.Kind, &b.Title); err != nil {
			return nil, err
		}
		if vis(b.Src) {
			if n, ok := reverseNames[b.Type]; ok {
				b.Type = n
			}
			out = append(out, b)
		}
	}
	return out, rows.Err()
}

func (a *App) apiBacklinks(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	if !a.canSee(ctx, u, id) {
		return errNotFound
	}
	b, err := a.backlinks(ctx, u, id)
	if err != nil {
		return err
	}
	return ok(w, b)
}

// apiResolve returns brief information about records by id, for links,
// item cards and pickers. Records the user cannot see are left out.
func (a *App) apiResolve(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	ids := strings.Split(r.URL.Query().Get("ids"), ",")
	if len(ids) > 200 {
		ids = ids[:200]
	}
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	out := map[string]any{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || !vis(id) {
			continue
		}
		var x struct {
			ID       string `json:"id"`
			Kind     string `json:"kind"`
			Title    string `json:"title"`
			Status   string `json:"status"`
			Revision string `json:"revision"`
			Space    string `json:"space"`
		}
		if err := a.DB.QueryRow(ctx, `SELECT id, kind, title, status, revision, space_id FROM records WHERE id = $1`, id).
			Scan(&x.ID, &x.Kind, &x.Title, &x.Status, &x.Revision, &x.Space); err == nil {
			out[id] = x
		}
	}
	return ok(w, out)
}

// ---------------------------------------------------------------------------
// Comments

type commentRow struct {
	ID        string     `json:"id"`
	Revision  string     `json:"revision"`
	AuthorID  string     `json:"authorId"`
	Author    string     `json:"author"`
	Body      string     `json:"body"`
	ReplyTo   *string    `json:"replyTo"`
	CreatedAt time.Time  `json:"createdAt"`
	EditedAt  *time.Time `json:"editedAt"`
	Resolved  bool       `json:"resolved"`
	Hidden    bool       `json:"hidden"`
}

func (a *App) apiComments(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	if !a.canSee(ctx, u, id) {
		return errNotFound
	}
	rows, err := a.DB.Query(ctx, `SELECT c.id::text, c.revision, c.author_id::text, us.display_name, c.body, c.reply_to::text, c.created_at, c.edited_at, c.resolved, c.hidden
		FROM comments c JOIN users us ON us.id = c.author_id WHERE c.record_id = $1 ORDER BY c.created_at`, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []commentRow{}
	for rows.Next() {
		var c commentRow
		if err := rows.Scan(&c.ID, &c.Revision, &c.AuthorID, &c.Author, &c.Body, &c.ReplyTo, &c.CreatedAt, &c.EditedAt, &c.Resolved, &c.Hidden); err != nil {
			return err
		}
		if c.Hidden {
			c.Body = ""
		}
		out = append(out, c)
	}
	return ok(w, out)
}

func (a *App) apiAddComment(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	if !a.canSee(ctx, u, id) {
		return errNotFound
	}
	var in struct {
		Body    string `json:"body"`
		ReplyTo string `json:"replyTo"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	body := strings.TrimSpace(in.Body)
	if body == "" || len(body) > 20000 {
		return badRequest("Comment text is required.")
	}
	var reply any
	if in.ReplyTo != "" {
		var parentReply *string
		if err := a.DB.QueryRow(ctx, `SELECT reply_to::text FROM comments WHERE id = $1 AND record_id = $2`, in.ReplyTo, id).Scan(&parentReply); err != nil {
			return badRequest("Unknown comment.")
		}
		if parentReply != nil {
			reply = *parentReply // one level of replies
		} else {
			reply = in.ReplyTo
		}
	}
	ri, _ := a.recordInfo(ctx, id)
	rev := ""
	title := ""
	if ri != nil {
		title = ri.Title
		if e := cache.get(id); e != nil {
			rev = e.Item.Revision
		}
	}
	var cid string
	if err := a.DB.QueryRow(ctx, `INSERT INTO comments (record_id, revision, author_id, body, reply_to) VALUES ($1, $2, $3, $4, $5) RETURNING id::text`,
		id, rev, u.ID, body, reply).Scan(&cid); err != nil {
		return err
	}
	a.notifyMentions(ctx, u, id, title, body)
	a.notifyWatchers(ctx, u, id, "Watched record changed", fmt.Sprintf("%s commented on %s %s", u.DisplayName, id, title))
	a.Events.Publish(Event{Type: "comments", ID: id})
	return ok(w, map[string]string{"id": cid})
}

func (a *App) apiUpdateComment(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id, cid := r.PathValue("id"), r.PathValue("cid")
	if !a.canSee(ctx, u, id) {
		return errNotFound
	}
	var in struct {
		Body     *string `json:"body"`
		Resolved *bool   `json:"resolved"`
		Hidden   *bool   `json:"hidden"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	var authorID, body string
	if err := a.DB.QueryRow(ctx, `SELECT author_id::text, body FROM comments WHERE id = $1 AND record_id = $2`, cid, id).Scan(&authorID, &body); err != nil {
		return err
	}
	isAuthor := authorID == u.ID
	if in.Body != nil {
		if !isAuthor {
			return errForbidden
		}
		nb := strings.TrimSpace(*in.Body)
		if nb == "" {
			return badRequest("Comment text is required.")
		}
		hist, _ := json.Marshal(map[string]any{"body": body, "at": time.Now()})
		if _, err := a.DB.Exec(ctx, `UPDATE comments SET body = $2, edited_at = now(), history = history || jsonb_build_array($3::jsonb) WHERE id = $1`, cid, nb, hist); err != nil {
			return err
		}
		ri, _ := a.recordInfo(ctx, id)
		if ri != nil {
			a.notifyMentions(ctx, u, id, ri.Title, nb)
		}
	}
	if in.Resolved != nil {
		if _, err := a.DB.Exec(ctx, `UPDATE comments SET resolved = $2 WHERE id = $1`, cid, *in.Resolved); err != nil {
			return err
		}
	}
	if in.Hidden != nil {
		if !isAuthor && !u.Can(PermUsers) {
			return errForbidden
		}
		if _, err := a.DB.Exec(ctx, `UPDATE comments SET hidden = $2 WHERE id = $1`, cid, *in.Hidden); err != nil {
			return err
		}
		a.Audit(ctx, u, clientIP(r), "comment.hide", id, map[string]any{"comment": cid, "hidden": *in.Hidden})
	}
	a.Events.Publish(Event{Type: "comments", ID: id})
	return ok(w, nil)
}

// ---------------------------------------------------------------------------
// Files

const maxUpload = 512 << 20

// apiUpload stores a file in the file store. The file is attached to a
// record when the record is saved with its checksum.
func (a *App) apiUpload(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermLibEdit) && len(u.Spaces) == 0 {
		return errForbidden
	}
	name := sanitizeFileName(r.URL.Query().Get("name"))
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+1)
	sum, size, err := a.Files.Put(r.Body, maxUpload)
	if err != nil {
		return badRequest("Upload failed: %v.", err)
	}
	ct := r.Header.Get("Content-Type")
	if ct == "" || ct == "application/octet-stream" {
		if t := mime.TypeByExtension(strings.ToLower(filepath.Ext(name))); t != "" {
			ct = t
		} else {
			ct = "application/octet-stream"
		}
	}
	if _, err := a.DB.Exec(r.Context(), `INSERT INTO files (sha256, size, mime, created_by) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, sum, size, ct, u.ID); err != nil {
		return err
	}
	return ok(w, model.File{SHA256: sum, Name: name, Size: size, Mime: ct})
}

// apiDownload serves a file attached to a record the user can see. The file
// must belong to some version of the record the user can open.
func (a *App) apiDownload(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id, sum := r.PathValue("id"), r.PathValue("sha")
	if !a.canSee(ctx, u, id) {
		return errNotFound
	}
	if !a.recordHasFile(ctx, id, sum) {
		return errNotFound
	}
	f, err := a.Files.Open(sum)
	if err != nil {
		return errNotFound
	}
	defer f.Close()
	var ct string
	var size int64
	a.DB.QueryRow(ctx, `SELECT mime, size FROM files WHERE sha256 = $1`, sum).Scan(&ct, &size)
	name := sanitizeFileName(r.URL.Query().Get("name"))
	inline := r.URL.Query().Get("inline") == "1" && (ct == "application/pdf" || strings.HasPrefix(ct, "image/")) && ct != "image/svg+xml"
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src 'self' data:; object-src 'self'; plugin-types application/pdf; sandbox allow-same-origin")
	disp := "attachment"
	if inline {
		disp = "inline"
	}
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disp, map[string]string{"filename": name}))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, name, time.Time{}, f)
	return nil
}

func (a *App) recordHasFile(ctx context.Context, id, sum string) bool {
	has := func(it *model.Item) bool {
		if it == nil {
			return false
		}
		for _, f := range it.Files {
			if f.SHA256 == sum {
				return true
			}
		}
		return strings.Contains(it.Body, sum)
	}
	if e := cache.get(id); e != nil {
		if has(e.Item) || has(e.Next) {
			return true
		}
		if co := a.openCOFor(ctx, a.DB, id); co != nil {
			if b, _, err := a.Repo.Read(co.Branch, a.draftPath(e)); err == nil {
				if it, err := model.Parse(b); err == nil && has(it) {
					return true
				}
			}
		}
		if vs, err := a.Repo.History(repo.Main, e.Path, 200); err == nil {
			for _, v := range vs {
				if b, err := a.Repo.Blob(v.Blob); err == nil && strings.Contains(string(b), sum) {
					return true
				}
			}
		}
		return false
	}
	var n int
	a.DB.QueryRow(ctx, `SELECT count(*) FROM work_items WHERE id = $1 AND (attachments::text LIKE '%' || $2 || '%' OR description LIKE '%' || $2 || '%')`, id, sum).Scan(&n)
	if n > 0 {
		return true
	}
	a.DB.QueryRow(ctx, `SELECT count(*) FROM comments WHERE record_id = $1 AND body LIKE '%' || $2 || '%'`, id, sum).Scan(&n)
	return n > 0
}

// ---------------------------------------------------------------------------
// Private autosaved drafts

func (a *App) apiGetDraft(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	var base string
	var content json.RawMessage
	var at time.Time
	err := a.DB.QueryRow(r.Context(), `SELECT base_hash, content, updated_at FROM edit_drafts WHERE user_id = $1 AND record_id = $2`, u.ID, r.PathValue("id")).Scan(&base, &content, &at)
	if err == pgx.ErrNoRows {
		return ok(w, map[string]any{"draft": nil})
	}
	if err != nil {
		return err
	}
	return ok(w, map[string]any{"draft": content, "base": base, "updatedAt": at})
}

func (a *App) apiPutDraft(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	if !a.canSee(ctx, u, id) {
		return errNotFound
	}
	var in struct {
		Base    string          `json:"base"`
		Content json.RawMessage `json:"content"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	_, err := a.DB.Exec(ctx, `INSERT INTO edit_drafts (user_id, record_id, base_hash, content) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, record_id) DO UPDATE SET base_hash = $3, content = $4, updated_at = now()`, u.ID, id, in.Base, []byte(in.Content))
	if err != nil {
		return err
	}
	return ok(w, nil)
}

func (a *App) apiDeleteDraft(w http.ResponseWriter, r *http.Request) error {
	_, err := a.DB.Exec(r.Context(), `DELETE FROM edit_drafts WHERE user_id = $1 AND record_id = $2`, userFrom(r).ID, r.PathValue("id"))
	if err != nil {
		return err
	}
	return ok(w, nil)
}

// copyLimited is used by tests and the backup code.
func copyLimited(dst io.Writer, src io.Reader, n int64) (int64, error) {
	return io.Copy(dst, io.LimitReader(src, n))
}
