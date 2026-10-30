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
	"designhub/internal/repo"
)

type spaceView struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	HomePage           string          `json:"homePage"`
	Archived           bool            `json:"archived"`
	ControlledDefaults map[string]bool `json:"controlledDefaults"`
	PinnedLinks        []pinnedLink    `json:"pinnedLinks"`
	CreatedAt          time.Time       `json:"createdAt"`
}

type pinnedLink struct {
	Title string `json:"title"`
	URL   string `json:"url"` // record id or path inside DesignHub
}

func (a *App) space(ctx context.Context, id string) (*spaceView, error) {
	s := &spaceView{}
	var cd, pl []byte
	err := a.DB.QueryRow(ctx, `SELECT id, name, description, home_page_id, archived, controlled_defaults, pinned_links, created_at FROM spaces WHERE id = $1`, id).
		Scan(&s.ID, &s.Name, &s.Description, &s.HomePage, &s.Archived, &cd, &pl, &s.CreatedAt)
	if err != nil {
		return nil, err
	}
	s.ControlledDefaults = map[string]bool{}
	json.Unmarshal(cd, &s.ControlledDefaults)
	json.Unmarshal(pl, &s.PinnedLinks)
	if s.PinnedLinks == nil {
		s.PinnedLinks = []pinnedLink{}
	}
	return s, nil
}

func (a *App) apiSpaces(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	rows, err := a.DB.Query(r.Context(), `SELECT id FROM spaces ORDER BY lower(name)`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	out := []*spaceView{}
	for _, id := range ids {
		if !u.InSpace(id) && !u.Can(PermSpaces) {
			continue
		}
		s, err := a.space(r.Context(), id)
		if err != nil {
			return err
		}
		out = append(out, s)
	}
	return ok(w, out)
}

// CreateSpace creates a space with its home page. The creator becomes the
// space Admin.
func (a *App) CreateSpace(ctx context.Context, u *User, ip, name, description string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 60 {
		return "", badRequest("Space name is required (at most 60 characters).")
	}
	var spaceID, pageID string
	err := a.tx(ctx, func(tx pgx.Tx) error {
		var err error
		if spaceID, err = a.nextPrefixed(ctx, tx, "SP", "space", u.ID); err != nil {
			return err
		}
		if pageID, err = a.nextPrefixed(ctx, tx, "PG", "page", u.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO spaces (id, name, description, home_page_id, created_by) VALUES ($1, $2, $3, $4, $5)`,
			spaceID, name, strings.TrimSpace(description), pageID, u.ID); err != nil {
			return err
		}
		admin := a.roleByName(RoleAdmin)
		if _, err := tx.Exec(ctx, `INSERT INTO space_members (space_id, user_id, role_id) VALUES ($1, $2, $3)`, spaceID, u.ID, admin.ID); err != nil {
			return err
		}
		home := &model.Item{ID: pageID, Type: model.TypePage, Title: name, Owner: u.Username, CreatedBy: u.Username, Space: spaceID,
			PageType: "General", Controlled: "no", Body: ""}
		b, err := home.Marshal()
		if err != nil {
			return err
		}
		if _, err := a.Repo.Commit(repo.Main, []repo.Change{{Path: model.Path(home), Content: b, Expect: "new"}}, author(u), fmt.Sprintf("Create space %s %s", spaceID, name)); err != nil {
			return err
		}
		a.auditQ(ctx, tx, u, ip, "space.create", spaceID, map[string]any{"name": name})
		return nil
	})
	if err != nil {
		return "", err
	}
	return spaceID, a.refresh(ctx, pageID, model.TypePage, spaceID)
}

func (a *App) apiCreateSpace(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermSpaces) {
		return errForbidden
	}
	var in struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	id, err := a.CreateSpace(r.Context(), u, clientIP(r), in.Name, in.Description)
	if err != nil {
		return err
	}
	return ok(w, map[string]string{"id": id})
}

type spaceHome struct {
	*spaceView
	Recent     []recentChange `json:"recent"`
	WorkCounts map[string]int `json:"workCounts"`
	OpenCOs    []coListRow    `json:"openChangeOrders"`
}

type recentChange struct {
	ID      string    `json:"id"`
	Kind    string    `json:"kind"`
	Title   string    `json:"title"`
	Status  string    `json:"status"`
	Updated time.Time `json:"updated"`
}

func (a *App) apiSpace(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	if !u.InSpace(id) {
		return errNotFound
	}
	s, err := a.space(ctx, id)
	if err != nil {
		return err
	}
	h := &spaceHome{spaceView: s, WorkCounts: map[string]int{}, Recent: []recentChange{}, OpenCOs: []coListRow{}}
	vis, err := a.visibility(ctx, u)
	if err != nil {
		return err
	}
	rows, err := a.DB.Query(ctx, `SELECT id, kind, title, status, updated_at FROM records WHERE space_id = $1 ORDER BY updated_at DESC LIMIT 60`, id)
	if err != nil {
		return err
	}
	for rows.Next() {
		var c recentChange
		if err := rows.Scan(&c.ID, &c.Kind, &c.Title, &c.Status, &c.Updated); err != nil {
			rows.Close()
			return err
		}
		if len(h.Recent) < 15 && vis(c.ID) {
			h.Recent = append(h.Recent, c)
		}
	}
	rows.Close()
	rows, err = a.DB.Query(ctx, `SELECT id, type FROM work_items WHERE space_id = $1 AND status NOT IN ('Done', 'Cancelled', 'Closed')`, id)
	if err != nil {
		return err
	}
	for rows.Next() {
		var wid, t string
		if err := rows.Scan(&wid, &t); err != nil {
			rows.Close()
			return err
		}
		if vis(wid) {
			h.WorkCounts[t]++
		}
	}
	rows.Close()
	cos, err := a.listCOs(ctx, u, "open", id)
	if err != nil {
		return err
	}
	h.OpenCOs = cos
	return ok(w, h)
}

func (a *App) apiUpdateSpace(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	if !u.InSpace(id) {
		return errNotFound
	}
	var in struct {
		Name               *string          `json:"name"`
		Description        *string          `json:"description"`
		Archived           *bool            `json:"archived"`
		PinnedLinks        *[]pinnedLink    `json:"pinnedLinks"`
		ControlledDefaults *map[string]bool `json:"controlledDefaults"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	admin := u.CanIn(id, PermSpaceAdmin) || u.Can(PermSpaces)
	changes := map[string]any{}
	if in.PinnedLinks != nil {
		if !u.CanIn(id, PermSpacePin) {
			return errForbidden
		}
		for _, l := range *in.PinnedLinks {
			if strings.TrimSpace(l.Title) == "" || strings.Contains(l.URL, "://") || strings.HasPrefix(strings.TrimSpace(l.URL), "javascript:") {
				return badRequest("Pinned links need a title and a DesignHub record or page.")
			}
		}
		b, _ := json.Marshal(*in.PinnedLinks)
		if _, err := a.DB.Exec(ctx, `UPDATE spaces SET pinned_links = $2 WHERE id = $1`, id, b); err != nil {
			return err
		}
		changes["pinnedLinks"] = *in.PinnedLinks
	}
	if in.Name != nil || in.Description != nil || in.Archived != nil || in.ControlledDefaults != nil {
		if !admin {
			return errForbidden
		}
	}
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" || len([]rune(n)) > 60 {
			return badRequest("Space name is required (at most 60 characters).")
		}
		if _, err := a.DB.Exec(ctx, `UPDATE spaces SET name = $2 WHERE id = $1`, id, n); err != nil {
			return err
		}
		changes["name"] = n
	}
	if in.Description != nil {
		if _, err := a.DB.Exec(ctx, `UPDATE spaces SET description = $2 WHERE id = $1`, id, strings.TrimSpace(*in.Description)); err != nil {
			return err
		}
		changes["description"] = *in.Description
	}
	if in.Archived != nil {
		if !u.Can(PermSpaces) {
			return errForbidden
		}
		if _, err := a.DB.Exec(ctx, `UPDATE spaces SET archived = $2 WHERE id = $1`, id, *in.Archived); err != nil {
			return err
		}
		changes["archived"] = *in.Archived
	}
	if in.ControlledDefaults != nil {
		b, _ := json.Marshal(*in.ControlledDefaults)
		if _, err := a.DB.Exec(ctx, `UPDATE spaces SET controlled_defaults = $2 WHERE id = $1`, id, b); err != nil {
			return err
		}
		changes["controlledDefaults"] = *in.ControlledDefaults
	}
	a.Audit(ctx, u, clientIP(r), "space.update", id, changes)
	a.Events.Publish(Event{Type: "space", ID: id, Space: id})
	return ok(w, nil)
}

type memberRow struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Role     string `json:"role"`
}

func (a *App) apiSpaceMembers(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	id := r.PathValue("id")
	if !u.InSpace(id) {
		return errNotFound
	}
	rows, err := a.DB.Query(r.Context(), `SELECT us.id::text, us.username, us.display_name, ro.name FROM space_members m
		JOIN users us ON us.id = m.user_id JOIN roles ro ON ro.id = m.role_id WHERE m.space_id = $1 AND us.active ORDER BY us.display_name`, id)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []memberRow{}
	for rows.Next() {
		var m memberRow
		if err := rows.Scan(&m.UserID, &m.Username, &m.Name, &m.Role); err != nil {
			return err
		}
		out = append(out, m)
	}
	return ok(w, out)
}

func (a *App) apiPutSpaceMember(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	ctx := r.Context()
	id := r.PathValue("id")
	if !(u.CanIn(id, PermSpaceAdmin) || u.Can(PermSpaces)) {
		return errForbidden
	}
	var in struct {
		UserID string `json:"userId"`
		Role   string `json:"role"` // empty removes the member
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	err := a.tx(ctx, func(tx pgx.Tx) error {
		if err := a.setSpaceRoles(ctx, tx, u, in.UserID, map[string]string{id: in.Role}); err != nil {
			return err
		}
		var admins int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM space_members m JOIN roles ro ON ro.id = m.role_id
			WHERE m.space_id = $1 AND $2 = ANY(ro.permissions)`, id, PermSpaceAdmin).Scan(&admins); err != nil {
			return err
		}
		if admins == 0 {
			return badRequest("A space must keep at least one Admin.")
		}
		a.auditQ(ctx, tx, u, clientIP(r), "space.member", id, map[string]any{"userId": in.UserID, "role": in.Role})
		return nil
	})
	if err != nil {
		return err
	}
	return ok(w, nil)
}

// controlledDefault returns whether pages of a type are controlled by
// default in a space.
func (a *App) controlledDefault(ctx context.Context, space, pageType string) bool {
	var b []byte
	if err := a.DB.QueryRow(ctx, `SELECT controlled_defaults FROM spaces WHERE id = $1`, space).Scan(&b); err != nil {
		return false
	}
	m := map[string]bool{}
	json.Unmarshal(b, &m)
	return m[pageType]
}
