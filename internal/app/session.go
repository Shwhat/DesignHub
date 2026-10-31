package app

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"designhub/internal/auth"
)

const (
	sessionCookie     = "dh_session"
	inactivityTimeout = 30 * time.Minute
	keepSignedIn      = 24 * time.Hour
)

func (a *App) secureCookie(r *http.Request) bool {
	switch a.Cfg.SecureCookie {
	case "always":
		return true
	case "never":
		return false
	}
	return isHTTPS(r)
}

func (a *App) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, keep bool) {
	c := &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: a.secureCookie(r), SameSite: http.SameSiteLaxMode}
	if keep {
		c.Expires = time.Now().Add(keepSignedIn)
	}
	http.SetCookie(w, c)
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})
}

// withUser loads the session user for API requests. Requests without a
// valid session get 401. State-changing requests must carry the CSRF token
// and come from the same origin.
func (a *App) withUser(next handlerFunc) handlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		c, err := r.Cookie(sessionCookie)
		if err != nil || c.Value == "" {
			return errUnauthorized
		}
		ctx := r.Context()
		hash := auth.HashToken(c.Value)
		var userID, csrf string
		var keep bool
		var lastSeen, expires time.Time
		err = a.DB.QueryRow(ctx, `SELECT user_id::text, csrf_token, keep, last_seen, expires_at FROM sessions WHERE token_hash = $1`, hash).
			Scan(&userID, &csrf, &keep, &lastSeen, &expires)
		if err != nil {
			clearSessionCookie(w)
			return errUnauthorized
		}
		now := time.Now()
		if now.After(expires) || (!keep && now.Sub(lastSeen) > inactivityTimeout) {
			a.DB.Exec(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hash)
			clearSessionCookie(w)
			return errUnauthorized
		}
		u, err := a.loadUser(ctx, userID)
		if err != nil {
			clearSessionCookie(w)
			return errUnauthorized
		}
		u.CSRF, u.SessionHash = csrf, hash
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if !sameOrigin(r) {
				return errForbidden
			}
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(csrf)) != 1 {
				return &Error{Status: http.StatusForbidden, Code: "csrf", Message: "Your session token is missing or out of date. Reload the page."}
			}
		}
		// Background polling (events, unread counts) must not keep an idle
		// session alive; the browser marks those requests.
		if r.Header.Get("X-Background") != "1" && now.Sub(lastSeen) > 30*time.Second {
			a.DB.Exec(ctx, `UPDATE sessions SET last_seen = now() WHERE token_hash = $1`, hash)
		}
		return next(w, r.WithContext(context.WithValue(ctx, userKey, u)))
	}
}

func (a *App) apiSignInInfo(w http.ResponseWriter, r *http.Request) error {
	var msg string
	a.Setting("sign_in_message", &msg)
	g := a.googleSettings()
	return ok(w, map[string]any{"message": msg, "version": a.Cfg.Version, "google": g.ready(), "passwordOff": g.ready() && g.Required})
}

// startSession creates a session for a signed-in user, sets the cookie and
// returns the CSRF token.
func (a *App) startSession(ctx context.Context, w http.ResponseWriter, r *http.Request, userID string, keep bool) (string, error) {
	token, th, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	csrf, _, err := auth.NewToken()
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(keepSignedIn)
	if _, err := a.DB.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, csrf_token, keep, expires_at, user_agent, ip) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		th, userID, csrf, keep, expires, truncate(r.UserAgent(), 300), clientIP(r)); err != nil {
		return "", err
	}
	a.DB.Exec(ctx, `UPDATE users SET last_sign_in = now() WHERE id = $1`, userID)
	a.DB.Exec(ctx, `DELETE FROM sessions WHERE expires_at < now() OR (NOT keep AND last_seen < now() - interval '1 day')`)
	a.setSessionCookie(w, r, token, keep)
	return csrf, nil
}

func (a *App) apiSignIn(w http.ResponseWriter, r *http.Request) error {
	if !sameOrigin(r) {
		return errForbidden
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Keep     bool   `json:"keep"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	ctx, ip := r.Context(), clientIP(r)
	name := strings.ToLower(strings.TrimSpace(in.Username))
	keys := []string{"user:" + name, "ip:" + ip}
	if a.Limiter.Blocked(keys...) {
		a.Audit(ctx, nil, ip, "signin.blocked", "", map[string]any{"username": name})
		return &Error{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "Too many failed attempts. Try again in 15 minutes."}
	}
	// While password sign-in is off, only a Super Admin given a recovery
	// window from the host may use it. While Google sign-in is on,
	// Google-linked accounts sign in with Google only.
	g := a.googleSettings()
	off := g.ready() && g.Required
	recovery := a.recoveryUser(ctx)
	var id, hash, method string
	var active bool
	err := a.DB.QueryRow(ctx, `SELECT id::text, password_hash, active, sign_in_method FROM users WHERE lower(username) = $1`, name).Scan(&id, &hash, &active, &method)
	if err != nil {
		hash = ""
	}
	isRecovery := recovery != "" && id == recovery
	if (off || (g.ready() && method == "google")) && !isRecovery {
		hash = ""
	}
	if !auth.CheckPassword(hash, in.Password) || !active {
		a.Limiter.Fail(keys...)
		a.Audit(ctx, nil, ip, "signin.failed", "", map[string]any{"username": name})
		if off {
			return &Error{Status: http.StatusUnauthorized, Code: "password_off", Message: "Password sign-in is turned off. Sign in with Google."}
		}
		return &Error{Status: http.StatusUnauthorized, Code: "bad_credentials", Message: "Username or password is incorrect."}
	}
	a.Limiter.Reset(keys...)
	csrf, err := a.startSession(ctx, w, r, id, in.Keep)
	if err != nil {
		return err
	}
	if isRecovery {
		a.DB.Exec(ctx, `DELETE FROM settings WHERE key = 'password_recovery'`)
	}
	u, err := a.loadUser(ctx, id)
	if err != nil {
		return err
	}
	u.CSRF = csrf
	a.Audit(ctx, u, ip, "signin", "", map[string]any{"keep": in.Keep, "method": "password", "recovery": isRecovery})
	p, err := a.profileOf(ctx, u)
	if err != nil {
		return err
	}
	return ok(w, p)
}

func (a *App) apiSignOut(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if _, err := a.DB.Exec(r.Context(), `DELETE FROM sessions WHERE token_hash = $1`, u.SessionHash); err != nil {
		return err
	}
	clearSessionCookie(w)
	a.Audit(r.Context(), u, clientIP(r), "signout", "", nil)
	return ok(w, nil)
}

type sessionInfo struct {
	ID        string    `json:"id"`
	Current   bool      `json:"current"`
	CreatedAt time.Time `json:"createdAt"`
	LastSeen  time.Time `json:"lastSeen"`
	UserAgent string    `json:"userAgent"`
	IP        string    `json:"ip"`
	Keep      bool      `json:"keep"`
}

func (a *App) apiSessions(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	rows, err := a.DB.Query(r.Context(), `SELECT encode(token_hash, 'hex'), created_at, last_seen, user_agent, ip, keep FROM sessions
		WHERE user_id = $1 AND expires_at > now() ORDER BY last_seen DESC`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	cur := strings.ToLower(hexString(u.SessionHash))
	out := []sessionInfo{}
	for rows.Next() {
		var s sessionInfo
		if err := rows.Scan(&s.ID, &s.CreatedAt, &s.LastSeen, &s.UserAgent, &s.IP, &s.Keep); err != nil {
			return err
		}
		if !s.Keep && time.Since(s.LastSeen) > inactivityTimeout {
			continue
		}
		s.Current = s.ID == cur
		s.ID = s.ID[:16]
		out = append(out, s)
	}
	return ok(w, out)
}

func (a *App) apiSignOutOthers(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if _, err := a.DB.Exec(r.Context(), `DELETE FROM sessions WHERE user_id = $1 AND token_hash <> $2`, u.ID, u.SessionHash); err != nil {
		return err
	}
	a.Audit(r.Context(), u, clientIP(r), "signout.others", "", nil)
	return ok(w, nil)
}

func (a *App) apiChangePassword(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	var in struct {
		Current string `json:"current"`
		New     string `json:"new"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	if !a.checkUserPassword(ctx, u.ID, in.Current) {
		return badRequest("Current password is incorrect.")
	}
	if err := a.setPassword(ctx, a.DB, u.ID, in.New); err != nil {
		return err
	}
	// setPassword ends every session; start a fresh one for this browser.
	token, th, err := auth.NewToken()
	if err != nil {
		return err
	}
	if _, err := a.DB.Exec(ctx, `INSERT INTO sessions (token_hash, user_id, csrf_token, keep, expires_at, user_agent, ip) VALUES ($1, $2, $3, false, $4, $5, $6)`,
		th, u.ID, u.CSRF, time.Now().Add(keepSignedIn), truncate(r.UserAgent(), 300), clientIP(r)); err != nil {
		return err
	}
	a.setSessionCookie(w, r, token, false)
	a.Audit(ctx, u, clientIP(r), "password.change", "", nil)
	return ok(w, nil)
}

// checkUserPassword verifies a password for electronic signatures. Failures
// count toward the sign-in limit.
func (a *App) checkUserPassword(ctx context.Context, userID, pw string) bool {
	var hash, name string
	if err := a.DB.QueryRow(ctx, `SELECT password_hash, lower(username) FROM users WHERE id = $1 AND active`, userID).Scan(&hash, &name); err != nil {
		return false
	}
	if a.Limiter.Blocked("user:" + name) {
		return false
	}
	if !auth.CheckPassword(hash, pw) {
		a.Limiter.Fail("user:" + name)
		return false
	}
	return true
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func hexString(b []byte) string {
	const h = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, c := range b {
		out[i*2], out[i*2+1] = h[c>>4], h[c&15]
	}
	return string(out)
}
