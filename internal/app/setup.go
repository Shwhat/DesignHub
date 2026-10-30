package app

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

// UserRole reports whether a user exists and their library role.
func (a *App) UserRole(ctx context.Context, username string) (bool, string, error) {
	var role string
	err := a.DB.QueryRow(ctx, `SELECT ro.name FROM users us JOIN roles ro ON ro.id = us.library_role_id WHERE lower(us.username) = lower($1)`, strings.TrimSpace(username)).Scan(&role)
	if err == pgx.ErrNoRows {
		return false, "", nil
	}
	return err == nil, role, err
}

// SetupSuperAdmin creates a Super Admin from the setup command and makes
// them Admin of every space.
func (a *App) SetupSuperAdmin(ctx context.Context, username, display, email, pw string) error {
	id, err := a.CreateUser(ctx, nil, "setup", userInput{Username: username, DisplayName: display, Email: email, Password: pw, LibraryRole: RoleSuperAdmin})
	if err != nil {
		return err
	}
	admin := a.roleByName(RoleAdmin)
	_, err = a.DB.Exec(ctx, `INSERT INTO space_members (space_id, user_id, role_id) SELECT id, $1, $2 FROM spaces ON CONFLICT DO NOTHING`, id, admin.ID)
	if err != nil {
		return err
	}
	a.Audit(ctx, nil, "setup", "setup.superadmin", "", map[string]any{"username": username})
	return a.ensureFirstSpace(ctx, id)
}

// ResetPassword sets a Super Admin's password from the setup command and
// re-enables the account.
func (a *App) ResetPassword(ctx context.Context, username, pw string) error {
	var id string
	if err := a.DB.QueryRow(ctx, `SELECT id::text FROM users WHERE lower(username) = lower($1)`, username).Scan(&id); err != nil {
		return err
	}
	if err := a.setPassword(ctx, a.DB, id, pw); err != nil {
		return err
	}
	if _, err := a.DB.Exec(ctx, `UPDATE users SET active = true WHERE id = $1`, id); err != nil {
		return err
	}
	a.Limiter.Reset("user:" + strings.ToLower(username))
	a.Audit(ctx, nil, "setup", "setup.password.reset", "", map[string]any{"username": username})
	return nil
}

// ensureFirstSpace creates the initial Factory Buildout space when no space
// exists yet.
func (a *App) ensureFirstSpace(ctx context.Context, adminID string) error {
	var n int
	if err := a.DB.QueryRow(ctx, `SELECT count(*) FROM spaces`).Scan(&n); err != nil || n > 0 {
		return err
	}
	u, err := a.loadUser(ctx, adminID)
	if err != nil {
		return err
	}
	_, err = a.CreateSpace(ctx, u, "setup", "Factory Buildout", "Standing up the first factory.")
	return err
}
