package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeGoogle stands in for Google's authorization and token endpoints. The
// next sign-in returns the claims in next.
type fakeGoogle struct {
	mu    sync.Mutex
	next  map[string]any
	codes map[string][2]string // code to nonce and PKCE challenge
	n     int
}

func (f *fakeGoogle) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f.mu.Lock()
		f.n++
		code := fmt.Sprintf("code-%d", f.n)
		f.codes[code] = [2]string{q.Get("nonce"), q.Get("code_challenge")}
		f.mu.Unlock()
		http.Redirect(w, r, testServer.URL+googleCallbackPath+"?code="+code+"&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		f.mu.Lock()
		c, ok := f.codes[r.Form.Get("code")]
		delete(f.codes, r.Form.Get("code"))
		claims := map[string]any{}
		for k, v := range f.next {
			claims[k] = v
		}
		f.mu.Unlock()
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if !ok || base64.RawURLEncoding.EncodeToString(sum[:]) != c[1] || r.Form.Get("client_secret") != "test-secret" {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		claims["nonce"] = c[0]
		b, _ := json.Marshal(claims)
		tok := "eyJhbGciOiJSUzI1NiJ9." + base64.RawURLEncoding.EncodeToString(b) + ".sig"
		json.NewEncoder(w).Encode(map[string]string{"id_token": tok})
	})
	return mux
}

func (f *fakeGoogle) claims(sub, email, hd, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next = map[string]any{"iss": "https://accounts.google.com", "aud": "test-client", "sub": sub, "email": email,
		"email_verified": true, "name": name, "exp": time.Now().Add(time.Hour).Unix()}
	if hd != "" {
		f.next["hd"] = hd
	}
}

// googleSignIn runs the browser side of a Google sign-in and returns the
// client and where DesignHub sent the browser at the end.
func googleSignIn(t *testing.T) (*client, string) {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	hc := &http.Client{Jar: jar, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if strings.HasPrefix(req.URL.String(), testServer.URL) && !strings.HasPrefix(req.URL.Path, "/api/auth/google") {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	resp, err := hc.Get(testServer.URL + "/api/auth/google/start?next=/library")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	loc := resp.Header.Get("Location")
	c := &client{t: t, http: &http.Client{Jar: jar}}
	if strings.HasPrefix(loc, "/signin") {
		return c, loc
	}
	var p struct {
		CSRF string `json:"csrf"`
	}
	c.must("GET", "/api/me", nil, &p)
	c.csrf = p.CSRF
	return c, loc
}

func TestGoogleSignIn(t *testing.T) {
	ctx := context.Background()
	fg := &fakeGoogle{codes: map[string][2]string{}}
	gs := httptest.NewServer(fg.handler())
	defer gs.Close()
	testApp.GoogleAuthURL, testApp.GoogleTokenURL = gs.URL+"/auth", gs.URL+"/token"
	t.Cleanup(func() {
		testApp.DB.Exec(ctx, `DELETE FROM settings WHERE key IN ('google_signin', 'password_recovery')`)
		testApp.DB.Exec(ctx, `UPDATE users SET google_sub = NULL, sign_in_method = 'local', email = '' WHERE username = 'superadmin'`)
		testApp.loadSettings(ctx)
		testApp.ResetPassword(ctx, "superadmin", seedPW)
		testApp.GoogleAuthURL, testApp.GoogleTokenURL = "", ""
	})

	sa := signIn(t, "superadmin")
	if st := sa.do("PUT", "/api/admin/google", map[string]any{"enabled": true}, nil); st != 400 {
		t.Fatalf("enable without settings: %d", st)
	}
	if st := sa.do("PUT", "/api/admin/google", map[string]any{"allowedDomains": []string{"gmail.com"}}, nil); st != 400 {
		t.Fatalf("gmail.com as a domain: %d", st)
	}
	if st := sa.do("PUT", "/api/admin/google", map[string]any{"publicUrl": "http://dh.example.com"}, nil); st != 400 {
		t.Fatalf("http public address: %d", st)
	}
	var v googleView
	sa.must("PUT", "/api/admin/google", map[string]any{"enabled": true, "clientId": "test-client", "clientSecret": "test-secret",
		"publicUrl": "https://DH.example.com/", "allowedDomains": []string{"@StandardArray.space"}, "allowedEmails": []string{"Solo@gmail.com"}}, &v)
	if !v.Enabled || !v.SecretSet || v.RedirectURI != "https://dh.example.com/api/auth/google/callback" || v.AllowedDomains[0] != "standardarray.space" || v.AllowedEmails[0] != "solo@gmail.com" {
		t.Fatalf("settings: %+v", v)
	}
	var raw map[string]any
	sa.must("GET", "/api/admin/google", nil, &raw)
	if _, leaked := raw["clientSecret"]; leaked {
		t.Fatal("client secret returned to the browser")
	}
	var audit int
	testApp.DB.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE detail::text LIKE '%test-secret%'`).Scan(&audit)
	if audit != 0 {
		t.Fatal("client secret written to the audit log")
	}
	if st := sa.do("PUT", "/api/admin/google", map[string]any{"required": true}, nil); st != 400 {
		t.Fatalf("password off before linking Google: %d", st)
	}
	if st := signIn(t, "editor1").do("GET", "/api/admin/google", nil, nil); st != 403 {
		t.Fatalf("editor reads Google settings: %d", st)
	}

	// A new company account is created as a Viewer with no spaces.
	fg.claims("g-new", "New.Person@standardarray.space", "standardarray.space", "New Person")
	c, loc := googleSignIn(t)
	if loc != "/library" {
		t.Fatalf("new account landed at %q", loc)
	}
	var me profile
	c.must("GET", "/api/me", nil, &me)
	if me.LibraryRole != RoleViewer || me.SignIn != "google" || len(me.Spaces) != 0 || me.Username != "new.person" || me.Email != "new.person@standardarray.space" {
		t.Fatalf("new Google user: %+v", me)
	}
	// The same Google account signs in to the same user.
	_, loc = googleSignIn(t)
	var n int
	testApp.DB.QueryRow(ctx, `SELECT count(*) FROM users WHERE google_sub = 'g-new'`).Scan(&n)
	if loc != "/library" || n != 1 {
		t.Fatalf("second sign-in: %q, %d accounts", loc, n)
	}
	// A personal account using the company address as its email has no
	// Workspace domain claim and is refused.
	fg.claims("g-personal", "someone@standardarray.space", "", "Someone")
	if _, loc = googleSignIn(t); loc != "/signin?error=google_not_allowed" {
		t.Fatalf("personal account with company email: %q", loc)
	}
	// Other domains are refused.
	fg.claims("g-other", "x@other.com", "other.com", "X")
	if _, loc = googleSignIn(t); loc != "/signin?error=google_not_allowed" {
		t.Fatalf("other domain: %q", loc)
	}
	// An individually allowed personal address is accepted.
	fg.claims("g-solo", "solo@gmail.com", "", "Solo")
	if _, loc = googleSignIn(t); loc != "/library" {
		t.Fatalf("allowed address: %q", loc)
	}
	// Unverified addresses are refused.
	fg.claims("g-unverified", "unverified@standardarray.space", "standardarray.space", "U")
	fg.next["email_verified"] = false
	if _, loc = googleSignIn(t); loc != "/signin?error=google_not_allowed" {
		t.Fatalf("unverified email: %q", loc)
	}
	// Deactivated accounts are refused.
	testApp.DB.Exec(ctx, `UPDATE users SET active = false WHERE google_sub = 'g-solo'`)
	fg.claims("g-solo", "solo@gmail.com", "", "Solo")
	if _, loc = googleSignIn(t); loc != "/signin?error=google_inactive" {
		t.Fatalf("inactive account: %q", loc)
	}

	// An email shared by several accounts is not linked to any of them.
	testApp.DB.Exec(ctx, `UPDATE users SET email = 'dup@standardarray.space' WHERE username IN ('editor1', 'editor2')`)
	fg.claims("g-dup", "dup@standardarray.space", "standardarray.space", "Dup")
	_, loc = googleSignIn(t)
	testApp.DB.Exec(ctx, `UPDATE users SET email = username || '@designhub.test' WHERE username IN ('editor1', 'editor2')`)
	if loc != "/signin?error=google_duplicate" {
		t.Fatalf("shared email: %q", loc)
	}

	// The Super Admin links their account by email, then turns password
	// sign-in off.
	testApp.DB.Exec(ctx, `UPDATE users SET email = 'admin@standardarray.space' WHERE username = 'superadmin'`)
	fg.claims("g-admin", "admin@standardarray.space", "standardarray.space", "Admin")
	sg, _ := googleSignIn(t)
	sg.must("GET", "/api/me", nil, &me)
	if me.Username != "superadmin" || me.SignIn != "google" {
		t.Fatalf("Super Admin link: %+v", me)
	}
	sg.must("PUT", "/api/admin/google", map[string]any{"required": true}, &v)
	if !v.Required || !v.MeLinked {
		t.Fatalf("password off: %+v", v)
	}
	var info map[string]any
	sg.must("GET", "/api/signin", nil, &info)
	if info["google"] != true || info["passwordOff"] != true {
		t.Fatalf("sign-in info: %v", info)
	}
	jar, _ := cookiejar.New(nil)
	pc := &client{t: t, http: &http.Client{Jar: jar}}
	if st := pc.do("POST", "/api/signin", map[string]any{"username": "editor1", "password": seedPW}, nil); st != 401 {
		t.Fatalf("password sign-in while off: %d", st)
	}

	// Recovery from the host allows one password sign-in for the Super Admin.
	if _, err := testApp.Recover(ctx, "editor1", seedPW); err == nil {
		t.Fatal("recovery for a non Super Admin")
	}
	if _, err := testApp.Recover(ctx, "superadmin", seedPW); err != nil {
		t.Fatal(err)
	}
	if st := pc.do("POST", "/api/signin", map[string]any{"username": "editor1", "password": seedPW}, nil); st != 401 {
		t.Fatalf("recovery opened sign-in for others: %d", st)
	}
	testApp.Limiter.Reset("ip:127.0.0.1", "user:superadmin")
	if st := pc.do("POST", "/api/signin", map[string]any{"username": "superadmin", "password": seedPW}, nil); st != 200 {
		t.Fatalf("recovery sign-in: %d", st)
	}
	if st := pc.do("POST", "/api/signin", map[string]any{"username": "superadmin", "password": seedPW}, nil); st != 401 {
		t.Fatalf("second recovery sign-in: %d", st)
	}
	testApp.Limiter.Reset("ip:127.0.0.1", "user:superadmin", "user:editor1")
}
