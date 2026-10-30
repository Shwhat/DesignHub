package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"

	"designhub/internal/model"
	"designhub/internal/repo"
)

// entry is a git record on main: its current file and, when a new revision
// is being drafted, the draft file.
type entry struct {
	Item     *model.Item
	Path     string
	Hash     string
	Next     *model.Item
	NextHash string
}

// gitCache holds every git record on main in memory, for fast lookups,
// Bill of Design rollups and validation.
type gitCache struct {
	mu    sync.RWMutex
	items map[string]*entry
	bad   map[string]string // path to parse error
}

var cache = &gitCache{items: map[string]*entry{}, bad: map[string]string{}}

func (c *gitCache) get(id string) *entry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.items[id]
}

// lookup returns the current file of a record (released revision, or the
// draft of a never-released record).
func (c *gitCache) lookup(id string) *model.Item {
	if e := c.get(id); e != nil {
		return e.Item
	}
	return nil
}

func (c *gitCache) all(typ string) []*model.Item {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var out []*model.Item
	for _, e := range c.items {
		if typ == "" || e.Item.Type == typ {
			out = append(out, e.Item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// loadMain reads every record file on main.
func (a *App) loadMain() (map[string]*entry, map[string]string, error) {
	files, err := a.Repo.List(repo.Main, "")
	if err != nil {
		return nil, nil, err
	}
	items := map[string]*entry{}
	bad := map[string]string{}
	nexts := map[string]repo.Entry{}
	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".md") || !(strings.HasPrefix(f.Path, "items/") || strings.HasPrefix(f.Path, "pages/")) {
			continue
		}
		if strings.HasSuffix(f.Path, ".next.md") {
			nexts[f.Path] = f
			continue
		}
		b, err := a.Repo.Blob(f.Hash)
		if err != nil {
			return nil, nil, err
		}
		it, err := model.Parse(b)
		if err != nil {
			bad[f.Path] = err.Error()
			continue
		}
		if model.Path(it) != f.Path {
			bad[f.Path] = fmt.Sprintf("file for %s is at the wrong path (expected %s)", it.ID, model.Path(it))
			continue
		}
		items[it.ID] = &entry{Item: it, Path: f.Path, Hash: f.Hash}
	}
	for _, e := range items {
		if n, ok := nexts[model.NextPath(e.Path)]; ok {
			b, err := a.Repo.Blob(n.Hash)
			if err != nil {
				return nil, nil, err
			}
			it, err := model.Parse(b)
			if err != nil {
				bad[n.Path] = err.Error()
				continue
			}
			e.Next, e.NextHash = it, n.Hash
		}
	}
	return items, bad, nil
}

// RebuildIndex reloads the cache from git and rebuilds the search index and
// link graph rows that come from git.
func (a *App) RebuildIndex(ctx context.Context) error {
	a.indexMu.Lock()
	defer a.indexMu.Unlock()
	items, bad, err := a.loadMain()
	if err != nil {
		return err
	}
	for p, e := range bad {
		log.Printf("invalid record file %s: %s", p, e)
	}
	cache.mu.Lock()
	cache.items, cache.bad = items, bad
	cache.mu.Unlock()
	return a.tx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM records WHERE origin = 'git'`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM links WHERE origin = 'git'`); err != nil {
			return err
		}
		for _, e := range items {
			if err := a.indexItem(ctx, tx, e); err != nil {
				return err
			}
		}
		return nil
	})
}

// indexItem writes the search row and outgoing links of a git record.
func (a *App) indexItem(ctx context.Context, q querier, e *entry) error {
	it := e.Item
	status := it.Status
	if co := a.openCOFor(ctx, q, it.ID); co != nil && (co.State == "In Review" || co.State == "Approved") && status == model.StatusDraft {
		status = model.StatusInReview
	}
	body := it.SearchText()
	if e.Next != nil {
		body += "\n" + e.Next.SearchText()
	}
	if _, err := q.Exec(ctx, `INSERT INTO records (id, kind, title, status, revision, space_id, owner_id, labels, body, origin, path, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, (SELECT id FROM users WHERE lower(username) = lower($7)), $8, $9, 'git', $10, now())
		ON CONFLICT (id) DO UPDATE SET kind = $2, title = $3, status = $4, revision = $5, space_id = $6,
		owner_id = (SELECT id FROM users WHERE lower(username) = lower($7)), labels = $8, body = $9, origin = 'git', path = $10, updated_at = now()`,
		it.ID, kindOf(it), it.Title, status, it.Revision, it.Space, it.Owner, nz(it.Labels), body, e.Path); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `DELETE FROM links WHERE src = $1 AND origin = 'git'`, it.ID); err != nil {
		return err
	}
	for _, l := range it.Links {
		if _, err := q.Exec(ctx, `INSERT INTO links (src, type, dst, dst_rev, origin) VALUES ($1, $2, $3, $4, 'git') ON CONFLICT DO NOTHING`,
			it.ID, l.Type, l.To, l.Rev); err != nil {
			return err
		}
	}
	for _, l := range it.BOM {
		if _, err := q.Exec(ctx, `INSERT INTO links (src, type, dst, dst_rev, origin) VALUES ($1, 'contains', $2, '', 'git') ON CONFLICT DO NOTHING`,
			it.ID, l.Item); err != nil {
			return err
		}
	}
	return nil
}

func nz(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// kindOf returns the record kind used in the index and the interface.
func kindOf(it *model.Item) string {
	if it.Type == model.TypePart {
		if it.HasBOM() {
			return "assembly"
		}
		return "part"
	}
	return it.Type
}

// refresh re-reads one record from main after a commit and updates the
// cache and index.
func (a *App) refresh(ctx context.Context, id, typ, space string) error {
	p := model.PathFor(typ, id, space)
	b, h, err := a.Repo.Read(repo.Main, p)
	if errors.Is(err, repo.ErrNotFound) {
		cache.mu.Lock()
		delete(cache.items, id)
		cache.mu.Unlock()
		_, err = a.DB.Exec(ctx, `DELETE FROM records WHERE id = $1 AND origin = 'git'`, id)
		if err == nil {
			_, err = a.DB.Exec(ctx, `DELETE FROM links WHERE src = $1 AND origin = 'git'`, id)
		}
		return err
	}
	if err != nil {
		return err
	}
	it, err := model.Parse(b)
	if err != nil {
		return err
	}
	e := &entry{Item: it, Path: p, Hash: h}
	if nb, nh, err := a.Repo.Read(repo.Main, model.NextPath(p)); err == nil {
		if n, err := model.Parse(nb); err == nil {
			e.Next, e.NextHash = n, nh
		}
	}
	cache.mu.Lock()
	cache.items[id] = e
	cache.mu.Unlock()
	return a.indexItem(ctx, a.DB, e)
}

// author returns the git author for a user.
func author(u *User) repo.Author {
	n, e := u.Author()
	return repo.Author{Name: n, Email: e}
}

// ---------------------------------------------------------------------------
// Access

// locks returns the access lists of all locked records.
func (a *App) locks(ctx context.Context) (map[string][]string, error) {
	rows, err := a.DB.Query(ctx, `SELECT record_id, user_id::text FROM record_access`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string][]string{}
	for rows.Next() {
		var r, u string
		if err := rows.Scan(&r, &u); err != nil {
			return nil, err
		}
		m[r] = append(m[r], u)
	}
	return m, rows.Err()
}

func (a *App) lockOf(ctx context.Context, q querier, id string) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT user_id::text FROM record_access WHERE record_id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// visible reports whether u may see a record of kind in space with the
// given access list.
func visible(u *User, kind, space string, lock []string) bool {
	if len(lock) > 0 && !contains(lock, u.ID) && !u.IsSuperAdmin() {
		return false
	}
	switch kind {
	case "page", "task", "issue", "action", "epic", "ncr":
		return u.InSpace(space)
	default:
		return u.Can(PermLibView)
	}
}

// visibility returns a function that reports whether u can see a record id,
// using the index for kind and space.
func (a *App) visibility(ctx context.Context, u *User) (func(id string) bool, error) {
	locks, err := a.locks(ctx)
	if err != nil {
		return nil, err
	}
	return func(id string) bool {
		if e := cache.get(id); e != nil {
			return visible(u, kindOf(e.Item), e.Item.Space, locks[id])
		}
		var kind, space string
		if err := a.DB.QueryRow(ctx, `SELECT kind, space_id FROM records WHERE id = $1`, id).Scan(&kind, &space); err != nil {
			return false
		}
		return visible(u, kind, space, locks[id])
	}, nil
}

// canSee checks one record.
func (a *App) canSee(ctx context.Context, u *User, id string) bool {
	vis, err := a.visibility(ctx, u)
	return err == nil && vis(id)
}

// ---------------------------------------------------------------------------
// Identifiers

// nextPrefixed reserves the next PREFIX-NUMBER identifier.
func (a *App) nextPrefixed(ctx context.Context, tx pgx.Tx, prefix, kind, userID string) (string, error) {
	for {
		var n int64
		if err := tx.QueryRow(ctx, `UPDATE counters SET next = next + 1 WHERE name = $1 RETURNING next - 1`, prefix).Scan(&n); err != nil {
			return "", fmt.Errorf("counter %s: %w", prefix, err)
		}
		id := model.FormatPrefixed(prefix, n)
		tag, err := tx.Exec(ctx, `INSERT INTO identifiers (id, kind, created_by) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, id, kind, nullUUID(userID))
		if err != nil {
			return "", err
		}
		if tag.RowsAffected() == 1 {
			return id, nil
		}
	}
}

func nullUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// offerProduction returns the next unused production base number.
func (a *App) offerProduction(ctx context.Context, q querier) (string, error) {
	var next int64
	if err := q.QueryRow(ctx, `SELECT next FROM counters WHERE name = 'production'`).Scan(&next); err != nil {
		return "", err
	}
	for {
		base := fmt.Sprintf("%06d", next)
		var used bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM identifiers WHERE substring(id from 1 for 6) = $1 AND id ~ '^[0-9]{6}-')`, base).Scan(&used); err != nil {
			return "", err
		}
		if !used {
			return base, nil
		}
		next++
	}
}

func (a *App) offerReference(ctx context.Context, q querier) (string, error) {
	var next int64
	if err := q.QueryRow(ctx, `SELECT next FROM counters WHERE name = 'reference'`).Scan(&next); err != nil {
		return "", err
	}
	for {
		id := fmt.Sprintf("%07d", next)
		var used bool
		if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM identifiers WHERE id = $1)`, id).Scan(&used); err != nil {
			return "", err
		}
		if !used {
			return id, nil
		}
		next++
	}
}

// reserveID inserts an identifier, refusing numbers already used or retired.
func (a *App) reserveID(ctx context.Context, tx pgx.Tx, id, kind, userID string) error {
	tag, err := tx.Exec(ctx, `INSERT INTO identifiers (id, kind, created_by) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, id, kind, nullUUID(userID))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var state string
		tx.QueryRow(ctx, `SELECT state FROM identifiers WHERE id = $1`, id).Scan(&state)
		if state == "retired" {
			return badRequest("%s was retired and cannot be used again.", id)
		}
		return badRequest("%s is already used.", id)
	}
	return nil
}
