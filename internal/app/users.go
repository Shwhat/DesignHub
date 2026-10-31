package app

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"designhub/internal/auth"
)

// Permissions. Library-wide permissions come from the user's library role;
// space permissions come from the user's role in that space.
const (
	PermSystem     = "system.manage" // system settings, backups, database status
	PermUsers      = "users.manage"
	PermSpaces     = "spaces.manage"
	PermLists      = "lists.manage"
	PermAudit      = "audit.view"
	PermLibView    = "library.view"
	PermLibEdit    = "library.edit"
	PermCOCreate   = "co.create"
	PermCOApprove  = "co.approve"
	PermCORelease  = "co.release"
	PermSpaceView  = "space.view"
	PermSpaceEdit  = "space.edit"
	PermWorkEdit   = "work.edit"
	PermSpacePin   = "space.pin"
	PermSpaceAdmin = "space.admin"
	PermRemove     = "record.remove" // permanently remove released records
)

// AllPermissions lists every permission in display order with a label.
var AllPermissions = []struct{ Key, Label string }{
	{PermSystem, "System settings, backups and database"},
	{PermUsers, "Manage users"},
	{PermSpaces, "Manage spaces"},
	{PermLists, "Manage lists and manufacturers"},
	{PermAudit, "View audit log"},
	{PermRemove, "Permanently remove records"},
	{PermLibView, "View library"},
	{PermLibEdit, "Create and edit engineering items"},
	{PermCOCreate, "Open change orders"},
	{PermCOApprove, "Approve change orders and NCR dispositions"},
	{PermCORelease, "Release change orders"},
	{PermSpaceView, "View space"},
	{PermSpaceEdit, "Create and edit pages"},
	{PermWorkEdit, "Create and update work items and NCRs"},
	{PermSpacePin, "Pin links on space home"},
	{PermSpaceAdmin, "Manage space members"},
}

// Role is a set of permissions.
type Role struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	BuiltIn            bool     `json:"builtIn"`
	AppliesTo          string   `json:"appliesTo"`
	Permissions        []string `json:"permissions"`
	CanManageApprovers bool     `json:"canManageApprovers"`
	SortOrder          int      `json:"sortOrder"`
}

func (r *Role) has(p string) bool { return r != nil && contains(r.Permissions, p) }

// Built-in role names.
const (
	RoleSuperAdmin = "Super Admin"
	RoleAdmin      = "Admin"
	RoleRelease    = "Release Authority"
	RoleApprover   = "Approver"
	RoleEditor     = "Editor"
	RoleContrib    = "Contributor"
	RoleViewer     = "Viewer"
)

func builtInRoles() []Role {
	all := make([]string, len(AllPermissions))
	for i, p := range AllPermissions {
		all[i] = p.Key
	}
	return []Role{
		{Name: RoleSuperAdmin, Permissions: all, CanManageApprovers: true},
		{Name: RoleAdmin, Permissions: []string{PermUsers, PermSpaces, PermLists, PermAudit, PermLibView, PermLibEdit, PermCOCreate,
			PermSpaceView, PermSpaceEdit, PermWorkEdit, PermSpacePin, PermSpaceAdmin}, CanManageApprovers: true},
		{Name: RoleRelease, Permissions: []string{PermLibView, PermCOApprove, PermCORelease, PermSpaceView}},
		{Name: RoleApprover, Permissions: []string{PermLibView, PermCOApprove, PermSpaceView}},
		{Name: RoleEditor, Permissions: []string{PermLibView, PermLibEdit, PermCOCreate, PermSpaceView, PermSpaceEdit, PermWorkEdit, PermSpacePin}, CanManageApprovers: true},
		{Name: RoleContrib, Permissions: []string{PermLibView, PermSpaceView, PermWorkEdit}},
		{Name: RoleViewer, Permissions: []string{PermLibView, PermSpaceView}},
	}
}

// syncBuiltInRoles creates the built-in roles and keeps their permissions
// in step with this version. The approver setting stays as an Admin set it.
func (a *App) syncBuiltInRoles(ctx context.Context) error {
	for i, r := range builtInRoles() {
		if _, err := a.DB.Exec(ctx, `INSERT INTO roles (name, built_in, applies_to, permissions, can_manage_approvers, sort_order)
			VALUES ($1, true, 'both', $2, $3, $4)
			ON CONFLICT (name) DO UPDATE SET built_in = true, permissions = $2, sort_order = $4`,
			r.Name, r.Permissions, r.CanManageApprovers, i); err != nil {
			return err
		}
	}
	return a.loadRoles(ctx)
}

func (a *App) loadRoles(ctx context.Context) error {
	rows, err := a.DB.Query(ctx, `SELECT id::text, name, built_in, applies_to, permissions, can_manage_approvers, sort_order FROM roles ORDER BY sort_order, name`)
	if err != nil {
		return err
	}
	defer rows.Close()
	m := map[string]*Role{}
	for rows.Next() {
		var r Role
		if err := rows.Scan(&r.ID, &r.Name, &r.BuiltIn, &r.AppliesTo, &r.Permissions, &r.CanManageApprovers, &r.SortOrder); err != nil {
			return err
		}
		m[r.ID] = &r
	}
	a.rolesMu.Lock()
	a.roles = m
	a.rolesMu.Unlock()
	return rows.Err()
}

func (a *App) role(id string) *Role {
	a.rolesMu.RLock()
	defer a.rolesMu.RUnlock()
	return a.roles[id]
}

func (a *App) roleByName(name string) *Role {
	a.rolesMu.RLock()
	defer a.rolesMu.RUnlock()
	for _, r := range a.roles {
		if r.Name == name {
			return r
		}
	}
	return nil
}

// User is the signed-in user with resolved roles.
type User struct {
	ID          string
	Username    string
	DisplayName string
	Email       string
	Library     *Role
	Spaces      map[string]*Role // space id to role
	EditMode    string
	TimeZone    string
	Theme       string
	SignIn      string // local or google
	CSRF        string
	SessionHash []byte
}

// Can reports a library-wide permission.
func (u *User) Can(p string) bool { return u != nil && u.Library.has(p) }

// IsSuperAdmin reports whether the user is a Super Admin.
func (u *User) IsSuperAdmin() bool { return u != nil && u.Library != nil && u.Library.Name == RoleSuperAdmin }

// CanIn reports a permission in a space. Users who manage spaces act with
// their library role in spaces they are not a member of.
func (u *User) CanIn(space, p string) bool {
	if u == nil {
		return false
	}
	if r, ok := u.Spaces[space]; ok {
		return r.has(p) || (u.Can(PermSpaces) && u.Library.has(p))
	}
	return u.Can(PermSpaces) && u.Library.has(p)
}

// InSpace reports whether the user can see a space at all.
func (u *User) InSpace(space string) bool { return u.CanIn(space, PermSpaceView) }

// Author returns the git author for commits by this user.
func (u *User) Author() (name, email string) {
	email = u.Email
	if email == "" {
		email = u.Username + "@designhub.local"
	}
	return u.DisplayName, email
}

type ctxKey int

const userKey ctxKey = 1

func userFrom(r *http.Request) *User {
	u, _ := r.Context().Value(userKey).(*User)
	return u
}

// loadUser reads a user and their space roles.
func (a *App) loadUser(ctx context.Context, id string) (*User, error) {
	u := &User{Spaces: map[string]*Role{}}
	var roleID string
	err := a.DB.QueryRow(ctx, `SELECT id::text, username, display_name, email, library_role_id::text, default_edit_mode, time_zone, theme, sign_in_method
		FROM users WHERE id = $1 AND active`, id).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Email, &roleID, &u.EditMode, &u.TimeZone, &u.Theme, &u.SignIn)
	if err != nil {
		return nil, err
	}
	u.Library = a.role(roleID)
	rows, err := a.DB.Query(ctx, `SELECT m.space_id, m.role_id::text FROM space_members m JOIN spaces s ON s.id = m.space_id WHERE m.user_id = $1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s, r string
		if err := rows.Scan(&s, &r); err != nil {
			return nil, err
		}
		u.Spaces[s] = a.role(r)
	}
	return u, rows.Err()
}

// ---------------------------------------------------------------------------
// User administration

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{1,39}$`)

type userRow struct {
	ID          string            `json:"id"`
	Username    string            `json:"username"`
	DisplayName string            `json:"displayName"`
	Email       string            `json:"email"`
	LibraryRole string            `json:"libraryRole"`
	Active      bool              `json:"active"`
	LastSignIn  *time.Time        `json:"lastSignIn"`
	SignIn      string            `json:"signInMethod"`
	PinSet      bool              `json:"signingPinSet"`
	SpaceRoles  map[string]string `json:"spaceRoles,omitempty"`
}

func (a *App) apiUsers(w http.ResponseWriter, r *http.Request) error {
	// Every signed-in user may list people (for assignee and approver
	// pickers); only user managers see the full administrative view.
	u := userFrom(r)
	rows, err := a.DB.Query(r.Context(), `SELECT u.id::text, u.username, u.display_name, u.email, ro.name, u.active, u.last_sign_in, u.sign_in_method
		FROM users u JOIN roles ro ON ro.id = u.library_role_id ORDER BY lower(u.display_name)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []userRow{}
	for rows.Next() {
		var x userRow
		if err := rows.Scan(&x.ID, &x.Username, &x.DisplayName, &x.Email, &x.LibraryRole, &x.Active, &x.LastSignIn, &x.SignIn); err != nil {
			return err
		}
		if !u.Can(PermUsers) {
			if !x.Active {
				continue
			}
			x.Email, x.LastSignIn = "", nil
		}
		out = append(out, x)
	}
	return ok(w, out)
}

func (a *App) apiUser(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermUsers) {
		return errForbidden
	}
	var x userRow
	err := a.DB.QueryRow(r.Context(), `SELECT u.id::text, u.username, u.display_name, u.email, ro.name, u.active, u.last_sign_in, u.sign_in_method, u.signing_pin_hash <> ''
		FROM users u JOIN roles ro ON ro.id = u.library_role_id WHERE u.id = $1`, r.PathValue("id")).
		Scan(&x.ID, &x.Username, &x.DisplayName, &x.Email, &x.LibraryRole, &x.Active, &x.LastSignIn, &x.SignIn, &x.PinSet)
	if err != nil {
		return err
	}
	x.SpaceRoles = map[string]string{}
	rows, err := a.DB.Query(r.Context(), `SELECT m.space_id, ro.name FROM space_members m JOIN roles ro ON ro.id = m.role_id WHERE m.user_id = $1`, x.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var s, n string
		if err := rows.Scan(&s, &n); err != nil {
			return err
		}
		x.SpaceRoles[s] = n
	}
	return ok(w, x)
}

type userInput struct {
	Username    string            `json:"username"`
	DisplayName string            `json:"displayName"`
	Email       string            `json:"email"`
	Password    string            `json:"password"`
	LibraryRole string            `json:"libraryRole"`
	Active      *bool             `json:"active"`
	SpaceRoles  map[string]string `json:"spaceRoles"`

	ClearSigningPin bool `json:"clearSigningPin"`
}

// CreateUser creates a local account. by may be nil for the setup command.
func (a *App) CreateUser(ctx context.Context, by *User, ip string, in userInput) (string, error) {
	in.Username = strings.TrimSpace(in.Username)
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if !usernameRe.MatchString(in.Username) {
		return "", badRequest("Username must be 2 to 40 letters, digits, periods, dashes or underscores.")
	}
	if in.DisplayName == "" {
		in.DisplayName = in.Username
	}
	if err := auth.ValidatePassword(in.Password, in.Username); err != nil {
		return "", badRequest("%s.", strings.ToUpper(err.Error()[:1])+err.Error()[1:])
	}
	role := a.roleByName(in.LibraryRole)
	if role == nil {
		return "", badRequest("Unknown library role.")
	}
	if role.Name == RoleSuperAdmin && by != nil && !by.IsSuperAdmin() {
		return "", errForbidden
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return "", err
	}
	var id string
	err = a.tx(ctx, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO users (username, display_name, email, password_hash, library_role_id)
			VALUES ($1, $2, $3, $4, $5) RETURNING id::text`, in.Username, in.DisplayName, strings.TrimSpace(in.Email), hash, role.ID).Scan(&id); err != nil {
			return err
		}
		if err := a.setSpaceRoles(ctx, tx, by, id, in.SpaceRoles); err != nil {
			return err
		}
		a.auditQ(ctx, tx, by, ip, "user.create", "", map[string]any{"username": in.Username, "libraryRole": role.Name, "spaceRoles": in.SpaceRoles})
		return nil
	})
	return id, err
}

func (a *App) setSpaceRoles(ctx context.Context, tx pgx.Tx, by *User, userID string, roles map[string]string) error {
	for space, name := range roles {
		if name == "" {
			if _, err := tx.Exec(ctx, `DELETE FROM space_members WHERE space_id = $1 AND user_id = $2`, space, userID); err != nil {
				return err
			}
			continue
		}
		role := a.roleByName(name)
		if role == nil || role.AppliesTo == "library" {
			return badRequest("Unknown space role %q.", name)
		}
		if role.Name == RoleSuperAdmin && by != nil && !by.IsSuperAdmin() {
			return errForbidden
		}
		if _, err := tx.Exec(ctx, `INSERT INTO space_members (space_id, user_id, role_id) VALUES ($1, $2, $3)
			ON CONFLICT (space_id, user_id) DO UPDATE SET role_id = $3`, space, userID, role.ID); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) apiCreateUser(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermUsers) {
		return errForbidden
	}
	var in userInput
	if err := decode(r, &in); err != nil {
		return err
	}
	id, err := a.CreateUser(r.Context(), u, clientIP(r), in)
	if err != nil {
		return err
	}
	return ok(w, map[string]string{"id": id})
}

func (a *App) apiUpdateUser(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermUsers) {
		return errForbidden
	}
	ctx, id := r.Context(), r.PathValue("id")
	var in userInput
	if err := decode(r, &in); err != nil {
		return err
	}
	var curRole string
	if err := a.DB.QueryRow(ctx, `SELECT ro.name FROM users us JOIN roles ro ON ro.id = us.library_role_id WHERE us.id = $1`, id).Scan(&curRole); err != nil {
		return err
	}
	// Admins cannot change Super Admins or make anyone a Super Admin.
	if (curRole == RoleSuperAdmin || in.LibraryRole == RoleSuperAdmin) && !u.IsSuperAdmin() {
		return errForbidden
	}
	err := a.tx(ctx, func(tx pgx.Tx) error {
		changes := map[string]any{}
		if in.DisplayName != "" {
			if _, err := tx.Exec(ctx, `UPDATE users SET display_name = $2 WHERE id = $1`, id, strings.TrimSpace(in.DisplayName)); err != nil {
				return err
			}
			changes["displayName"] = in.DisplayName
		}
		if in.Email != "" {
			if _, err := tx.Exec(ctx, `UPDATE users SET email = $2 WHERE id = $1`, id, strings.TrimSpace(in.Email)); err != nil {
				return err
			}
			changes["email"] = in.Email
		}
		if in.LibraryRole != "" && in.LibraryRole != curRole {
			role := a.roleByName(in.LibraryRole)
			if role == nil {
				return badRequest("Unknown library role.")
			}
			if curRole == RoleSuperAdmin {
				if err := a.ensureAnotherSuperAdmin(ctx, tx, id); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx, `UPDATE users SET library_role_id = $2 WHERE id = $1`, id, role.ID); err != nil {
				return err
			}
			changes["libraryRole"] = map[string]string{"from": curRole, "to": role.Name}
		}
		if in.Active != nil {
			if !*in.Active {
				if id == u.ID {
					return badRequest("You cannot disable your own account.")
				}
				if curRole == RoleSuperAdmin {
					if err := a.ensureAnotherSuperAdmin(ctx, tx, id); err != nil {
						return err
					}
				}
				if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, id); err != nil {
					return err
				}
			}
			if _, err := tx.Exec(ctx, `UPDATE users SET active = $2 WHERE id = $1`, id, *in.Active); err != nil {
				return err
			}
			changes["active"] = *in.Active
		}
		if in.SpaceRoles != nil {
			if err := a.setSpaceRoles(ctx, tx, u, id, in.SpaceRoles); err != nil {
				return err
			}
			changes["spaceRoles"] = in.SpaceRoles
		}
		if in.Password != "" {
			if err := a.setPassword(ctx, tx, id, in.Password); err != nil {
				return err
			}
			changes["password"] = "reset"
		}
		if in.ClearSigningPin {
			if _, err := tx.Exec(ctx, `UPDATE users SET signing_pin_hash = '' WHERE id = $1`, id); err != nil {
				return err
			}
			changes["signingPin"] = "cleared"
		}
		if len(changes) > 0 {
			a.auditQ(ctx, tx, u, clientIP(r), "user.update", "", map[string]any{"userId": id, "changes": changes})
		}
		return nil
	})
	if err != nil {
		return err
	}
	return ok(w, nil)
}

func (a *App) ensureAnotherSuperAdmin(ctx context.Context, tx pgx.Tx, exceptID string) error {
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM users us JOIN roles ro ON ro.id = us.library_role_id
		WHERE ro.name = $1 AND us.active AND us.id <> $2`, RoleSuperAdmin, exceptID).Scan(&n); err != nil {
		return err
	}
	if n == 0 {
		return badRequest("At least one active Super Admin must remain.")
	}
	return nil
}

func (a *App) setPassword(ctx context.Context, q querier, userID, pw string) error {
	var username string
	if err := q.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, userID).Scan(&username); err != nil {
		return err
	}
	if err := auth.ValidatePassword(pw, username); err != nil {
		return badRequest("%s.", strings.ToUpper(err.Error()[:1])+err.Error()[1:])
	}
	hash, err := auth.HashPassword(pw)
	if err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, hash); err != nil {
		return err
	}
	_, err = q.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

// ---------------------------------------------------------------------------
// Roles

func (a *App) apiRoles(w http.ResponseWriter, r *http.Request) error {
	a.rolesMu.RLock()
	out := []*Role{}
	for _, ro := range a.roles {
		out = append(out, ro)
	}
	a.rolesMu.RUnlock()
	sortRoles(out)
	return ok(w, map[string]any{"roles": out, "permissions": AllPermissions})
}

func sortRoles(rs []*Role) {
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && rs[j].SortOrder < rs[j-1].SortOrder; j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}

func (a *App) apiUpdateRole(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermUsers) {
		return errForbidden
	}
	var in struct {
		CanManageApprovers bool `json:"canManageApprovers"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	ro := a.role(r.PathValue("id"))
	if ro == nil {
		return errNotFound
	}
	if _, err := a.DB.Exec(r.Context(), `UPDATE roles SET can_manage_approvers = $2 WHERE id = $1`, ro.ID, in.CanManageApprovers); err != nil {
		return err
	}
	a.Audit(r.Context(), u, clientIP(r), "role.update", "", map[string]any{"role": ro.Name, "canManageApprovers": in.CanManageApprovers})
	if err := a.loadRoles(r.Context()); err != nil {
		return err
	}
	return ok(w, nil)
}

// ---------------------------------------------------------------------------
// Own profile and settings

type profile struct {
	ID          string          `json:"id"`
	Username    string          `json:"username"`
	DisplayName string          `json:"displayName"`
	Email       string          `json:"email"`
	LibraryRole string          `json:"libraryRole"`
	Permissions []string        `json:"permissions"`
	EditMode    string          `json:"editMode"`
	TimeZone    string          `json:"timeZone"`
	Theme       string          `json:"theme"`
	HomeLayout  json.RawMessage `json:"homeLayout"`
	Notify      json.RawMessage `json:"notifyEvents"`
	Spaces      []spaceRef      `json:"spaces"`
	CSRF        string          `json:"csrf"`
	ManageAppr  bool            `json:"canManageApprovers"`
	SignIn      string          `json:"signInMethod"`
	PinSet      bool            `json:"signingPinSet"`
}

type spaceRef struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
	Archived    bool     `json:"archived"`
	HomePage    string   `json:"homePage"`
}

func (a *App) profileOf(ctx context.Context, u *User) (*profile, error) {
	p := &profile{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName, Email: u.Email, LibraryRole: u.Library.Name,
		Permissions: u.Library.Permissions, EditMode: u.EditMode, TimeZone: u.TimeZone, Theme: u.Theme, CSRF: u.CSRF,
		ManageAppr: u.Library.CanManageApprovers, SignIn: u.SignIn, Spaces: []spaceRef{}}
	if err := a.DB.QueryRow(ctx, `SELECT home_layout, notify_events, signing_pin_hash <> '' FROM users WHERE id = $1`, u.ID).Scan(&p.HomeLayout, &p.Notify, &p.PinSet); err != nil {
		return nil, err
	}
	rows, err := a.DB.Query(ctx, `SELECT id, name, archived, home_page_id FROM spaces ORDER BY lower(name)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var s spaceRef
		if err := rows.Scan(&s.ID, &s.Name, &s.Archived, &s.HomePage); err != nil {
			return nil, err
		}
		if !u.InSpace(s.ID) {
			continue
		}
		role := u.Spaces[s.ID]
		if role == nil {
			role = u.Library
		}
		s.Role = role.Name
		for _, perm := range []string{PermSpaceView, PermSpaceEdit, PermWorkEdit, PermSpacePin, PermSpaceAdmin} {
			if u.CanIn(s.ID, perm) {
				s.Permissions = append(s.Permissions, perm)
			}
		}
		p.Spaces = append(p.Spaces, s)
	}
	return p, rows.Err()
}

func (a *App) apiMe(w http.ResponseWriter, r *http.Request) error {
	p, err := a.profileOf(r.Context(), userFrom(r))
	if err != nil {
		return err
	}
	return ok(w, p)
}

var timeZoneRe = regexp.MustCompile(`^([A-Za-z_]+(/[A-Za-z0-9_+-]+){0,2}|UTC)?$`)

func (a *App) apiUpdateMe(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	var in struct {
		DisplayName *string          `json:"displayName"`
		Email       *string          `json:"email"`
		EditMode    *string          `json:"editMode"`
		TimeZone    *string          `json:"timeZone"`
		Theme       *string          `json:"theme"`
		HomeLayout  *json.RawMessage `json:"homeLayout"`
		Notify      *json.RawMessage `json:"notifyEvents"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	set := func(col string, v any) error {
		_, err := a.DB.Exec(ctx, `UPDATE users SET `+col+` = $2 WHERE id = $1`, u.ID, v)
		return err
	}
	if in.DisplayName != nil {
		n := strings.TrimSpace(*in.DisplayName)
		if n == "" || len(n) > 80 {
			return badRequest("Display name is required (at most 80 characters).")
		}
		if err := set("display_name", n); err != nil {
			return err
		}
	}
	if in.Email != nil && !(u.SignIn == "google" && strings.EqualFold(strings.TrimSpace(*in.Email), u.Email)) {
		// A Google account's email comes from Google.
		if u.SignIn == "google" {
			return badRequest("Your email comes from your Google account and cannot be changed here.")
		}
		if err := set("email", strings.TrimSpace(*in.Email)); err != nil {
			return err
		}
	}
	if in.EditMode != nil {
		if !contains([]string{"visual", "split", "source", "preview"}, *in.EditMode) {
			return badRequest("Unknown editing mode.")
		}
		if err := set("default_edit_mode", *in.EditMode); err != nil {
			return err
		}
	}
	if in.TimeZone != nil {
		if !timeZoneRe.MatchString(*in.TimeZone) {
			return badRequest("Unknown time zone.")
		}
		if *in.TimeZone != "" {
			if _, err := time.LoadLocation(*in.TimeZone); err != nil {
				return badRequest("Unknown time zone.")
			}
		}
		if err := set("time_zone", *in.TimeZone); err != nil {
			return err
		}
	}
	if in.Theme != nil {
		if !contains([]string{"light", "dark", "device"}, *in.Theme) {
			return badRequest("Unknown theme.")
		}
		if err := set("theme", *in.Theme); err != nil {
			return err
		}
	}
	if in.HomeLayout != nil {
		if err := set("home_layout", []byte(*in.HomeLayout)); err != nil {
			return err
		}
	}
	if in.Notify != nil {
		if err := set("notify_events", []byte(*in.Notify)); err != nil {
			return err
		}
	}
	nu, err := a.loadUser(ctx, u.ID)
	if err != nil {
		return err
	}
	nu.CSRF = u.CSRF
	p, err := a.profileOf(ctx, nu)
	if err != nil {
		return err
	}
	return ok(w, p)
}

// User preferences stored per key (column choices, tree expansion, filters).
func (a *App) apiGetPref(w http.ResponseWriter, r *http.Request) error {
	var v json.RawMessage
	err := a.DB.QueryRow(r.Context(), `SELECT value FROM user_prefs WHERE user_id = $1 AND key = $2`, userFrom(r).ID, r.PathValue("key")).Scan(&v)
	if err == pgx.ErrNoRows {
		// No saved preference: answer JSON null so callers fall back to defaults.
		v = json.RawMessage("null")
		err = nil
	}
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, err = w.Write(v)
	return err
}

func (a *App) apiPutPref(w http.ResponseWriter, r *http.Request) error {
	var v json.RawMessage
	if err := decode(r, &v); err != nil {
		return err
	}
	key := r.PathValue("key")
	if len(key) > 100 {
		return badRequest("Key too long.")
	}
	_, err := a.DB.Exec(r.Context(), `INSERT INTO user_prefs (user_id, key, value) VALUES ($1, $2, $3)
		ON CONFLICT (user_id, key) DO UPDATE SET value = $3`, userFrom(r).ID, key, []byte(v))
	if err != nil {
		return err
	}
	return ok(w, nil)
}
