package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"designhub/internal/auth"
)

// Google sign-in uses the OpenID Connect authorization code flow with PKCE.
// The ID token is taken directly from Google's token endpoint over HTTPS,
// so its claims are trusted without a signature check (Google's documented
// rule for tokens received this way). Who may sign in is decided by the
// allowed domains and allowed addresses an Admin sets in Admin, System.

const (
	googleAuthEndpoint  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenEndpoint = "https://oauth2.googleapis.com/token"
	googleCallbackPath  = "/api/auth/google/callback"
	googleStateCookie   = "dh_google_state"
	googleStateLifetime = 10 * time.Minute
	googleMaxStates     = 2000
	recoveryWindow      = time.Hour
)

// googleSettings is stored under the setting key "google_signin".
type googleSettings struct {
	Enabled        bool     `json:"enabled"`
	Required       bool     `json:"required"`
	ClientID       string   `json:"clientId"`
	ClientSecret   string   `json:"clientSecret"`
	PublicURL      string   `json:"publicUrl"`
	AllowedDomains []string `json:"allowedDomains"`
	AllowedEmails  []string `json:"allowedEmails"`
}

func (a *App) googleSettings() googleSettings {
	g := googleSettings{AllowedDomains: []string{}, AllowedEmails: []string{}}
	a.Setting("google_signin", &g)
	if g.AllowedDomains == nil {
		g.AllowedDomains = []string{}
	}
	if g.AllowedEmails == nil {
		g.AllowedEmails = []string{}
	}
	return g
}

// googleReady reports whether Google sign-in is turned on and configured.
func (g googleSettings) ready() bool {
	return g.Enabled && g.ClientID != "" && g.ClientSecret != "" && g.PublicURL != ""
}

// passwordSignInOff reports whether password sign-in is turned off.
func (a *App) passwordSignInOff() bool {
	g := a.googleSettings()
	return g.ready() && g.Required
}

func (g googleSettings) redirectURI() string {
	if g.PublicURL == "" {
		return ""
	}
	return g.PublicURL + googleCallbackPath
}

// allows reports whether a verified Google address may sign in. A domain
// entry matches only Google Workspace accounts of that domain (the hd
// claim), so personal accounts cannot pass as company accounts.
func (g googleSettings) allows(email, hd string) bool {
	email = strings.ToLower(strings.TrimSpace(email))
	hd = strings.ToLower(strings.TrimSpace(hd))
	for _, e := range g.AllowedEmails {
		if email == e {
			return true
		}
	}
	at := strings.LastIndex(email, "@")
	if at < 0 || hd == "" {
		return false
	}
	for _, d := range g.AllowedDomains {
		if hd == d && email[at+1:] == d {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Admin settings

type googleView struct {
	Enabled        bool     `json:"enabled"`
	Required       bool     `json:"required"`
	ClientID       string   `json:"clientId"`
	SecretSet      bool     `json:"secretSet"`
	PublicURL      string   `json:"publicUrl"`
	RedirectURI    string   `json:"redirectUri"`
	AllowedDomains []string `json:"allowedDomains"`
	AllowedEmails  []string `json:"allowedEmails"`
	MeLinked       bool     `json:"meLinked"`
}

func (a *App) googleViewFor(ctx context.Context, u *User) googleView {
	g := a.googleSettings()
	v := googleView{Enabled: g.Enabled, Required: g.Required, ClientID: g.ClientID, SecretSet: g.ClientSecret != "",
		PublicURL: g.PublicURL, RedirectURI: g.redirectURI(), AllowedDomains: g.AllowedDomains, AllowedEmails: g.AllowedEmails}
	var sub *string
	if err := a.DB.QueryRow(ctx, `SELECT google_sub FROM users WHERE id = $1`, u.ID).Scan(&sub); err == nil && sub != nil {
		v.MeLinked = true
	}
	return v
}

func (a *App) apiGetGoogle(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermSystem) {
		return errForbidden
	}
	return ok(w, a.googleViewFor(r.Context(), u))
}

var (
	domainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
	emailRe  = regexp.MustCompile(`^[^@\s]+@[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)
)

// consumerDomains have no Workspace domain claim, so a domain entry for them
// could never match. Their users are added as individual addresses.
var consumerDomains = map[string]bool{"gmail.com": true, "googlemail.com": true}

func (a *App) apiPutGoogle(w http.ResponseWriter, r *http.Request) error {
	u := userFrom(r)
	if !u.Can(PermSystem) {
		return errForbidden
	}
	var in struct {
		Enabled        *bool     `json:"enabled"`
		Required       *bool     `json:"required"`
		ClientID       *string   `json:"clientId"`
		ClientSecret   *string   `json:"clientSecret"`
		PublicURL      *string   `json:"publicUrl"`
		AllowedDomains *[]string `json:"allowedDomains"`
		AllowedEmails  *[]string `json:"allowedEmails"`
	}
	if err := decode(r, &in); err != nil {
		return err
	}
	ctx := r.Context()
	g := a.googleSettings()
	if in.ClientID != nil {
		g.ClientID = strings.TrimSpace(*in.ClientID)
	}
	if in.ClientSecret != nil && strings.TrimSpace(*in.ClientSecret) != "" {
		g.ClientSecret = strings.TrimSpace(*in.ClientSecret)
	}
	if in.PublicURL != nil {
		p := strings.TrimRight(strings.TrimSpace(*in.PublicURL), "/")
		if p != "" {
			pu, err := url.Parse(p)
			if err != nil || pu.Scheme != "https" || pu.Host == "" || (pu.Path != "" && pu.Path != "/") || pu.RawQuery != "" {
				return badRequest("Public address must look like https://designhub.example.com, with no path.")
			}
			p = "https://" + strings.ToLower(pu.Host)
		}
		g.PublicURL = p
	}
	if in.AllowedDomains != nil {
		ds := []string{}
		for _, d := range *in.AllowedDomains {
			d = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(d)), "@")
			if d == "" {
				continue
			}
			if consumerDomains[d] {
				return badRequest("%s is not a Google Workspace domain. Add those people as individual addresses instead.", d)
			}
			if !domainRe.MatchString(d) {
				return badRequest("%s is not a valid domain.", d)
			}
			if !contains(ds, d) {
				ds = append(ds, d)
			}
		}
		g.AllowedDomains = ds
	}
	if in.AllowedEmails != nil {
		es := []string{}
		for _, e := range *in.AllowedEmails {
			e = strings.ToLower(strings.TrimSpace(e))
			if e == "" {
				continue
			}
			if !emailRe.MatchString(e) {
				return badRequest("%s is not a valid email address.", e)
			}
			if !contains(es, e) {
				es = append(es, e)
			}
		}
		g.AllowedEmails = es
	}
	if in.Enabled != nil {
		g.Enabled = *in.Enabled
	}
	if in.Required != nil {
		g.Required = *in.Required
	}
	if g.Enabled {
		if g.ClientID == "" || g.ClientSecret == "" || g.PublicURL == "" {
			return badRequest("Enter the client ID, client secret and public address before turning on Google sign-in.")
		}
		if len(g.AllowedDomains)+len(g.AllowedEmails) == 0 {
			return badRequest("Add at least one allowed domain or address before turning on Google sign-in.")
		}
	} else {
		g.Required = false
	}
	if g.Required && in.Required != nil && *in.Required {
		// Turning password sign-in off is only allowed for a Super Admin who
		// has already signed in with Google, so the setting cannot lock out
		// every administrator.
		if !u.IsSuperAdmin() {
			return badRequest("Only a Super Admin can turn off password sign-in.")
		}
		if !a.googleViewFor(ctx, u).MeLinked {
			return badRequest("Sign in with Google once before turning off password sign-in.")
		}
	}
	audit := g
	audit.ClientSecret = ""
	if in.ClientSecret != nil && strings.TrimSpace(*in.ClientSecret) != "" {
		audit.ClientSecret = "(changed)"
	}
	if err := a.setSettingAudited(ctx, u, clientIP(r), "google_signin", g, audit); err != nil {
		return err
	}
	return ok(w, a.googleViewFor(ctx, u))
}

// ---------------------------------------------------------------------------
// Sign-in flow

type googleState struct {
	verifier string
	nonce    string
	keep     bool
	next     string
	created  time.Time
}

type googleStates struct {
	mu sync.Mutex
	m  map[string]googleState
}

func (s *googleStates) put(key string, st googleState) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.m == nil {
		s.m = map[string]googleState{}
	}
	for k, v := range s.m {
		if time.Since(v.created) > googleStateLifetime {
			delete(s.m, k)
		}
	}
	if len(s.m) >= googleMaxStates {
		return false
	}
	s.m[key] = st
	return true
}

func (s *googleStates) take(key string) (googleState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.m[key]
	delete(s.m, key)
	if !ok || time.Since(st.created) > googleStateLifetime {
		return googleState{}, false
	}
	return st, true
}

// safeNext keeps a post-sign-in path inside DesignHub.
func safeNext(p string) string {
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.HasPrefix(p, "/\\") || strings.HasPrefix(p, "/signin") {
		return "/"
	}
	return p
}

func signInError(w http.ResponseWriter, r *http.Request, code string) error {
	http.Redirect(w, r, "/signin?error="+url.QueryEscape(code), http.StatusFound)
	return nil
}

func (a *App) apiGoogleStart(w http.ResponseWriter, r *http.Request) error {
	g := a.googleSettings()
	if !g.ready() {
		return signInError(w, r, "google_off")
	}
	state, _, err := auth.NewToken()
	if err != nil {
		return err
	}
	verifier, _, err := auth.NewToken()
	if err != nil {
		return err
	}
	nonce, _, err := auth.NewToken()
	if err != nil {
		return err
	}
	q := r.URL.Query()
	if !a.google.put(state, googleState{verifier: verifier, nonce: nonce, keep: q.Get("keep") == "1", next: safeNext(q.Get("next")), created: time.Now()}) {
		return signInError(w, r, "google_busy")
	}
	http.SetCookie(w, &http.Cookie{Name: googleStateCookie, Value: state, Path: "/api/auth/google", HttpOnly: true,
		Secure: a.secureCookie(r), SameSite: http.SameSiteLaxMode, MaxAge: int(googleStateLifetime.Seconds())})
	sum := sha256.Sum256([]byte(verifier))
	v := url.Values{
		"client_id":             {g.ClientID},
		"redirect_uri":          {g.redirectURI()},
		"response_type":         {"code"},
		"scope":                 {"openid email profile"},
		"state":                 {state},
		"nonce":                 {nonce},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
		"prompt":                {"select_account"},
	}
	if len(g.AllowedDomains) == 1 && len(g.AllowedEmails) == 0 {
		v.Set("hd", g.AllowedDomains[0])
	}
	endpoint := a.GoogleAuthURL
	if endpoint == "" {
		endpoint = googleAuthEndpoint
	}
	http.Redirect(w, r, endpoint+"?"+v.Encode(), http.StatusFound)
	return nil
}

type googleClaims struct {
	Iss           string `json:"iss"`
	Aud           string `json:"aud"`
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"`
	HD            string `json:"hd"`
	Name          string `json:"name"`
	Nonce         string `json:"nonce"`
	Exp           int64  `json:"exp"`
}

func (c googleClaims) verified() bool {
	switch v := c.EmailVerified.(type) {
	case bool:
		return v
	case string:
		return v == "true"
	}
	return false
}

// exchange trades the authorization code for Google's ID token claims.
func (a *App) googleExchange(ctx context.Context, g googleSettings, code, verifier, nonce string) (*googleClaims, error) {
	endpoint := a.GoogleTokenURL
	if endpoint == "" {
		endpoint = googleTokenEndpoint
	}
	form := url.Values{
		"code":          {code},
		"client_id":     {g.ClientID},
		"client_secret": {g.ClientSecret},
		"redirect_uri":  {g.redirectURI()},
		"grant_type":    {"authorization_code"},
		"code_verifier": {verifier},
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request: status %d: %s", res.StatusCode, truncate(string(body), 300))
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &tok); err != nil || tok.IDToken == "" {
		return nil, errors.New("token response has no ID token")
	}
	parts := strings.Split(tok.IDToken, ".")
	if len(parts) != 3 {
		return nil, errors.New("ID token is malformed")
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return nil, errors.New("ID token is malformed")
	}
	var c googleClaims
	if err := json.Unmarshal(payload, &c); err != nil {
		return nil, errors.New("ID token is malformed")
	}
	switch {
	case c.Iss != "accounts.google.com" && c.Iss != "https://accounts.google.com":
		return nil, errors.New("ID token issuer is not Google")
	case c.Aud != g.ClientID:
		return nil, errors.New("ID token is for another client")
	case time.Now().Unix() > c.Exp+60:
		return nil, errors.New("ID token has expired")
	case c.Nonce != nonce:
		return nil, errors.New("ID token nonce does not match")
	case c.Sub == "":
		return nil, errors.New("ID token has no subject")
	}
	return &c, nil
}

func (a *App) apiGoogleCallback(w http.ResponseWriter, r *http.Request) error {
	ctx, ip := r.Context(), clientIP(r)
	q := r.URL.Query()
	state := q.Get("state")
	c, err := r.Cookie(googleStateCookie)
	http.SetCookie(w, &http.Cookie{Name: googleStateCookie, Value: "", Path: "/api/auth/google", HttpOnly: true, MaxAge: -1})
	if err != nil || state == "" || c.Value != state {
		return signInError(w, r, "google_expired")
	}
	st, found := a.google.take(state)
	if !found {
		return signInError(w, r, "google_expired")
	}
	if q.Get("error") != "" {
		return signInError(w, r, "google_cancelled")
	}
	g := a.googleSettings()
	if !g.ready() {
		return signInError(w, r, "google_off")
	}
	claims, err := a.googleExchange(ctx, g, q.Get("code"), st.verifier, st.nonce)
	if err != nil {
		a.Audit(ctx, nil, ip, "signin.google.failed", "", map[string]any{"reason": err.Error()})
		return signInError(w, r, "google_failed")
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	if !claims.verified() || !g.allows(email, claims.HD) {
		a.Audit(ctx, nil, ip, "signin.google.refused", "", map[string]any{"email": email, "hd": claims.HD})
		return signInError(w, r, "google_not_allowed")
	}
	id, err := a.googleUser(ctx, ip, claims, email)
	if err != nil {
		var ae *Error
		if errors.As(err, &ae) {
			return signInError(w, r, ae.Code)
		}
		return err
	}
	if _, err := a.startSession(ctx, w, r, id, st.keep); err != nil {
		return err
	}
	u, err := a.loadUser(ctx, id)
	if err != nil {
		return err
	}
	a.Audit(ctx, u, ip, "signin", "", map[string]any{"keep": st.keep, "method": "google"})
	http.Redirect(w, r, st.next, http.StatusFound)
	return nil
}

// googleUser finds the account for a Google sign-in: first by the linked
// Google account, then by email (linking it), else it creates a Viewer with
// no space access for an Admin to set up.
func (a *App) googleUser(ctx context.Context, ip string, c *googleClaims, email string) (string, error) {
	var id string
	var active bool
	err := a.DB.QueryRow(ctx, `SELECT id::text, active FROM users WHERE google_sub = $1`, c.Sub).Scan(&id, &active)
	if err == nil {
		if !active {
			return "", &Error{Status: http.StatusForbidden, Code: "google_inactive"}
		}
		a.DB.Exec(ctx, `UPDATE users SET email = $2 WHERE id = $1 AND email <> $2`, id, email)
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}
	// Link by email only when exactly one account has that address, so a
	// user cannot claim another person's Google account by copying it.
	var matches int
	if err := a.DB.QueryRow(ctx, `SELECT count(*) FROM users WHERE lower(email) = $1`, email).Scan(&matches); err != nil {
		return "", err
	}
	if matches > 1 {
		a.Audit(ctx, nil, ip, "signin.google.refused", "", map[string]any{"email": email, "reason": "several accounts have this email"})
		return "", &Error{Status: http.StatusConflict, Code: "google_duplicate"}
	}
	var linked *string
	err = a.DB.QueryRow(ctx, `SELECT id::text, active, google_sub FROM users WHERE lower(email) = $1`, email).Scan(&id, &active, &linked)
	if err == nil {
		if linked != nil {
			a.Audit(ctx, nil, ip, "signin.google.refused", "", map[string]any{"email": email, "reason": "email belongs to an account linked to another Google account"})
			return "", &Error{Status: http.StatusConflict, Code: "google_duplicate"}
		}
		if !active {
			return "", &Error{Status: http.StatusForbidden, Code: "google_inactive"}
		}
		if _, err := a.DB.Exec(ctx, `UPDATE users SET google_sub = $2, sign_in_method = 'google', password_hash = '' WHERE id = $1`, id, c.Sub); err != nil {
			return "", err
		}
		a.Audit(ctx, nil, ip, "user.google.link", "", map[string]any{"userId": id, "email": email})
		return id, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}
	viewer := a.roleByName(RoleViewer)
	if viewer == nil {
		return "", errors.New("viewer role missing")
	}
	display := strings.TrimSpace(c.Name)
	if display == "" {
		display = email
	}
	err = a.tx(ctx, func(tx pgx.Tx) error {
		username, err := uniqueUsername(ctx, tx, email)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO users (username, display_name, email, sign_in_method, google_sub, library_role_id)
			VALUES ($1, $2, $3, 'google', $4, $5) RETURNING id::text`, username, truncate(display, 100), email, c.Sub, viewer.ID).Scan(&id); err != nil {
			return err
		}
		a.auditQ(ctx, tx, nil, ip, "user.create", "", map[string]any{"username": username, "libraryRole": viewer.Name, "method": "google", "email": email})
		return nil
	})
	return id, err
}

var usernameStrip = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// uniqueUsername derives a free username from the part of an email address
// before the @.
func uniqueUsername(ctx context.Context, q querier, email string) (string, error) {
	base := email
	if i := strings.Index(base, "@"); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimLeft(usernameStrip.ReplaceAllString(base, ""), "._-")
	if len(base) > 34 {
		base = base[:34]
	}
	if len(base) < 2 {
		base = "user" + base
	}
	for i := 1; i < 1000; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s-%d", base, i)
		}
		var n int
		if err := q.QueryRow(ctx, `SELECT count(*) FROM users WHERE lower(username) = lower($1)`, name).Scan(&n); err != nil {
			return "", err
		}
		if n == 0 {
			return name, nil
		}
	}
	return "", errors.New("no free username")
}

// ---------------------------------------------------------------------------
// Recovery

type recoveryGrant struct {
	UserID string    `json:"userId"`
	Until  time.Time `json:"until"`
}

// recoveryUser returns the account allowed to use password sign-in while
// password sign-in is off, if a recovery window is open. It reads the
// database because the recovery command runs in a separate process.
func (a *App) recoveryUser(ctx context.Context) string {
	var g recoveryGrant
	var raw []byte
	if err := a.DB.QueryRow(ctx, `SELECT value FROM settings WHERE key = 'password_recovery'`).Scan(&raw); err != nil {
		return ""
	}
	if json.Unmarshal(raw, &g) != nil || time.Now().After(g.Until) {
		return ""
	}
	return g.UserID
}

// Recover sets a new password for a Super Admin from the host and lets that
// account use password sign-in for one hour, even when password sign-in is
// turned off.
func (a *App) Recover(ctx context.Context, username, pw string) (time.Time, error) {
	var id, role string
	var active bool
	err := a.DB.QueryRow(ctx, `SELECT us.id::text, ro.name, us.active FROM users us JOIN roles ro ON ro.id = us.library_role_id
		WHERE lower(us.username) = lower($1)`, strings.TrimSpace(username)).Scan(&id, &role, &active)
	if err == pgx.ErrNoRows {
		return time.Time{}, fmt.Errorf("user %s does not exist", username)
	}
	if err != nil {
		return time.Time{}, err
	}
	if role != RoleSuperAdmin {
		return time.Time{}, fmt.Errorf("user %s is not a Super Admin", username)
	}
	if err := a.setPassword(ctx, a.DB, id, pw); err != nil {
		var ae *Error
		if errors.As(err, &ae) {
			return time.Time{}, errors.New(ae.Message)
		}
		return time.Time{}, err
	}
	until := time.Now().Add(recoveryWindow).UTC().Truncate(time.Second)
	b, _ := json.Marshal(recoveryGrant{UserID: id, Until: until})
	if _, err := a.DB.Exec(ctx, `UPDATE users SET active = true WHERE id = $1`, id); err != nil {
		return time.Time{}, err
	}
	if _, err := a.DB.Exec(ctx, `INSERT INTO settings (key, value) VALUES ('password_recovery', $1)
		ON CONFLICT (key) DO UPDATE SET value = $1, updated_by = NULL, updated_at = now()`, b); err != nil {
		return time.Time{}, err
	}
	a.Limiter.Reset("user:" + strings.ToLower(username))
	a.Audit(ctx, nil, "setup", "setup.recovery", "", map[string]any{"username": username, "until": until})
	return until, nil
}
