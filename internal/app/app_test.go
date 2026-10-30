package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"designhub/internal/config"
	"designhub/internal/db"
	"designhub/internal/model"
)

// These tests need a PostgreSQL database they may wipe, given in
// DESIGNHUB_TEST_DATABASE_URL. They load the seed data set and exercise the
// API like the browser does.

const seedPW = "test-password-123"

var testApp *App
var testServer *httptest.Server

func TestMain(m *testing.M) {
	url := os.Getenv("DESIGNHUB_TEST_DATABASE_URL")
	if url == "" {
		fmt.Println("DESIGNHUB_TEST_DATABASE_URL not set; skipping app tests")
		os.Exit(0)
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url, 10*time.Second)
	if err != nil {
		panic(err)
	}
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public`); err != nil {
		panic(err)
	}
	if _, err := db.Migrate(ctx, pool); err != nil {
		panic(err)
	}
	dir, _ := os.MkdirTemp("", "designhub-test-")
	defer os.RemoveAll(dir)
	cfg := &config.Config{DatabaseURL: url, RepoDir: dir + "/repo", FilesDir: dir + "/files", BackupsDir: dir + "/backups", SecretsDir: dir + "/secrets", SecureCookie: "never", Version: "test"}
	os.MkdirAll(cfg.SecretsDir, 0o755)
	a, err := New(ctx, cfg, pool, nil)
	if err != nil {
		panic(err)
	}
	a.LoadSigningKey()
	if err := a.SetupSuperAdmin(ctx, "superadmin", "Super Admin", "", seedPW); err != nil {
		panic(err)
	}
	if err := a.LoadSeed(ctx, seedPW); err != nil {
		panic(err)
	}
	testApp = a
	testServer = httptest.NewServer(a.Handler())
	code := m.Run()
	testServer.Close()
	pool.Close()
	os.Exit(code)
}

type client struct {
	t    *testing.T
	http *http.Client
	csrf string
}

func signIn(t *testing.T, user string) *client {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &client{t: t, http: &http.Client{Jar: jar}}
	var p struct {
		CSRF string `json:"csrf"`
	}
	if st := c.do("POST", "/api/signin", map[string]any{"username": user, "password": seedPW}, &p); st != 200 {
		t.Fatalf("sign in %s: %d", user, st)
	}
	c.csrf = p.CSRF
	return c
}

func (c *client) do(method, path string, body any, out any) int {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, testServer.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if out != nil && resp.StatusCode == 200 {
		if err := json.Unmarshal(b, out); err != nil {
			c.t.Fatalf("%s %s: %v\n%s", method, path, err, b)
		}
	}
	if resp.StatusCode >= 500 {
		c.t.Errorf("%s %s: %d %s", method, path, resp.StatusCode, b)
	}
	return resp.StatusCode
}

func (c *client) must(method, path string, body any, out any) {
	c.t.Helper()
	if st := c.do(method, path, body, out); st != 200 {
		var e map[string]any
		c.do(method, path, body, &e)
		c.t.Fatalf("%s %s: status %d", method, path, st)
	}
}

func TestSignIn(t *testing.T) {
	jar, _ := cookiejar.New(nil)
	c := &client{t: t, http: &http.Client{Jar: jar}}
	if st := c.do("POST", "/api/signin", map[string]any{"username": "editor1", "password": "wrong-password"}, nil); st != 401 {
		t.Fatalf("wrong password: %d", st)
	}
	if st := c.do("POST", "/api/signin", map[string]any{"username": "nobody", "password": "wrong-password"}, nil); st != 401 {
		t.Fatalf("unknown user: %d", st)
	}
	if st := c.do("GET", "/api/home", nil, nil); st != 401 {
		t.Fatalf("unauthenticated API: %d", st)
	}
	for i := 0; i < 6; i++ {
		c.do("POST", "/api/signin", map[string]any{"username": "viewer1", "password": "wrong-password"}, nil)
	}
	if st := c.do("POST", "/api/signin", map[string]any{"username": "viewer1", "password": seedPW}, nil); st != 429 {
		t.Fatalf("rate limit: %d", st)
	}
	testApp.Limiter.Reset("user:viewer1", "ip:127.0.0.1")
	e := signIn(t, "editor1")
	// CSRF required
	csrf := e.csrf
	e.csrf = ""
	if st := e.do("PATCH", "/api/me", map[string]any{"theme": "dark"}, nil); st != 403 {
		t.Fatalf("missing CSRF: %d", st)
	}
	e.csrf = csrf
	e.must("PATCH", "/api/me", map[string]any{"theme": "dark"}, nil)
}

func TestSeedBOMThroughAPI(t *testing.T) {
	c := signIn(t, "editor1")
	var bom struct {
		Flat []model.FlatLine `json:"flat"`
	}
	c.must("GET", "/api/items/100004-201/bom", nil, &bom)
	got := map[string]float64{}
	for _, f := range bom.Flat {
		got[f.ID] = f.Total
	}
	for id, want := range map[string]float64{"PXC.3031212": 30, "SIE.3RT2015-1BB41": 6, "100001-001": 10, "100007-001": 2} {
		if got[id] != want {
			t.Errorf("%s: %g, want %g", id, got[id], want)
		}
	}
	// editor2 is not on the access list of 100007-001
	c2 := signIn(t, "editor2")
	c2.must("GET", "/api/items/100004-201/bom", nil, &bom)
	for _, f := range bom.Flat {
		if f.ID == "100007-001" {
			t.Error("locked item visible to editor2")
		}
	}
	if st := c2.do("GET", "/api/items/DOC-0002", nil, nil); st != 404 {
		t.Errorf("locked document: %d", st)
	}
	var sr struct {
		Results []searchResult `json:"results"`
	}
	c2.must("GET", "/api/search?q=Pricing", nil, &sr)
	if len(sr.Results) != 0 {
		t.Error("locked record found by search")
	}
	var lib []libraryRow
	c.must("GET", "/api/library/validation", nil, &lib)
}

func TestSeedState(t *testing.T) {
	c := signIn(t, "editor1")
	var v itemView
	c.must("GET", "/api/items/100003-201", nil, &v)
	if v.Item.Revision != "01" || v.Status != "Released" || v.WorkCount != 2 {
		t.Errorf("100003-201: rev %q status %q work %d", v.Item.Revision, v.Status, v.WorkCount)
	}
	var tr []traceRow
	c.must("GET", "/api/traceability", nil, &tr)
	cov := map[string]string{}
	for _, r := range tr {
		cov[r.ID] = r.Coverage
	}
	if cov["REQ-0001"] != "Complete" || cov["REQ-0002"] != "No test" || cov["REQ-0003"] != "No design and no test" {
		t.Errorf("coverage: %v", cov)
	}
	var mw []workRow
	c.must("GET", "/api/my-work", nil, &mw)
	if len(mw) < 3 || mw[0].ID != "TSK-0001" || !mw[0].Overdue {
		t.Errorf("my work: %+v", mw)
	}
	var ep map[string]any
	c.must("GET", "/api/work/EPC-0001", nil, &ep)
	if p := ep["progress"].(map[string]any)["percent"].(float64); p != 33 {
		t.Errorf("epic progress %v", p)
	}
	var n struct {
		Items []notificationRow `json:"items"`
	}
	c.must("GET", "/api/notifications", nil, &n)
	if len(n.Items) != 1 || n.Items[0].Event != "Mentioned" {
		t.Errorf("notifications: %+v", n.Items)
	}
	if out, err := exec.Command("git", "--git-dir", testApp.Repo.Dir(), "fsck", "--strict").CombinedOutput(); err != nil {
		t.Errorf("git fsck: %v %s", err, out)
	}
}

func TestNumbering(t *testing.T) {
	c := signIn(t, "editor1")
	var offer map[string]string
	c.must("GET", "/api/items/new", nil, &offer)
	if offer["production"] != "100008-001" || offer["reference"] != "1000002" || offer["requirement"] != "REQ-0004" {
		t.Fatalf("offers: %v", offer)
	}
	item := func(kind, number string) (int, string) {
		var out map[string]string
		st := c.do("POST", "/api/items", map[string]any{"number": number, "item": map[string]any{"type": "part", "kind": kind, "title": "Test " + number, "space": "SP-0001"}}, &out)
		return st, out["id"]
	}
	if st, _ := item("production", "100001-001"); st != 400 {
		t.Errorf("used number accepted: %d", st)
	}
	if st, id := item("production", "100001-003"); st != 200 || id != "100001-003" {
		t.Errorf("free suffix on used base: %d %s", st, id)
	}
	if st, id := item("production", "100010-001"); st != 200 || id != "100010-001" {
		t.Errorf("chosen base: %d %s", st, id)
	}
	if _, id := item("production", ""); id != "100008-001" {
		t.Errorf("first offered: %s", id)
	}
	if _, id := item("production", ""); id != "100009-001" {
		t.Errorf("second offered: %s", id)
	}
	if _, id := item("production", ""); id != "100011-001" {
		t.Errorf("skip used base: %s", id)
	}
	// delete never-released draft, number retired
	c.must("DELETE", "/api/items/100001-002", nil, nil)
	if st, _ := item("production", "100001-002"); st != 400 {
		t.Errorf("retired number reused: %d", st)
	}
	if st := c.do("DELETE", "/api/items/100001-001", nil, nil); st == 200 {
		t.Error("released item deleted")
	}
	var out map[string]string
	c.must("POST", "/api/items", map[string]any{"orderNumber": "1234567", "item": map[string]any{"type": "part", "kind": "cots", "manufacturer": "PXC", "title": "Test COTS", "space": "SP-0001"}}, &out)
	if out["id"] != "PXC.1234567" {
		t.Errorf("COTS id %s", out["id"])
	}
	v := signIn(t, "viewer1")
	if st := v.do("POST", "/api/items", map[string]any{"item": map[string]any{"type": "part", "kind": "production", "title": "x", "space": "SP-0001"}}, nil); st != 403 {
		t.Errorf("viewer created item: %d", st)
	}
}

func TestEditConflict(t *testing.T) {
	a, b := signIn(t, "editor1"), signIn(t, "editor2")
	var v1, v2 itemView
	a.must("GET", "/api/items/100005-001", nil, &v1)
	b.must("GET", "/api/items/100005-001", nil, &v2)
	v1.Item.Title = "Door cover A"
	a.must("PUT", "/api/items/100005-001", map[string]any{"item": v1.Item, "base": v1.Hash, "comment": "rename"}, nil)
	v2.Item.Title = "Door cover B"
	if st := b.do("PUT", "/api/items/100005-001", map[string]any{"item": v2.Item, "base": v2.Hash, "comment": "rename"}, nil); st != 409 {
		t.Fatalf("stale save: %d", st)
	}
	var v itemView
	a.must("GET", "/api/items/100005-001", nil, &v)
	if v.Item.Title != "Door cover A" {
		t.Errorf("first save lost: %s", v.Item.Title)
	}
	v.Item.Title = ""
	if st := a.do("PUT", "/api/items/100005-001", map[string]any{"item": v.Item, "base": v.Hash}, nil); st != 400 {
		t.Errorf("empty name accepted: %d", st)
	}
	var hist []historyRow
	a.must("GET", "/api/records/100005-001/history", nil, &hist)
	if len(hist) < 2 || !strings.Contains(hist[0].Message, "rename") {
		t.Errorf("history: %+v", hist)
	}
}

func TestChangeOrderRelease(t *testing.T) {
	appr2 := signIn(t, "appr2")
	if st := appr2.do("POST", "/api/change-orders/CO-0002/decision", map[string]any{"decision": "Approved", "password": "wrong", "comment": "ok"}, nil); st != 401 {
		t.Fatalf("wrong signature password: %d", st)
	}
	appr2.must("POST", "/api/change-orders/CO-0002/decision", map[string]any{"decision": "Approved", "password": seedPW, "comment": "ok"}, nil)
	var co struct {
		CO    coRecord `json:"co"`
		Perms coPerms  `json:"perms"`
	}
	appr2.must("GET", "/api/change-orders/CO-0002", nil, &co)
	if co.CO.State != "Approved" {
		t.Fatalf("state %s", co.CO.State)
	}
	appr1 := signIn(t, "appr1")
	if st := appr1.do("POST", "/api/change-orders/CO-0002/release", map[string]any{"password": seedPW}, nil); st != 403 {
		t.Errorf("approver released: %d", st)
	}
	ra := signIn(t, "ra1")
	ra.must("POST", "/api/change-orders/CO-0002/release", map[string]any{"password": seedPW}, nil)
	var v itemView
	ra.must("GET", "/api/items/100002-001", nil, &v)
	if v.Item.Revision != "02" || v.Status != "Released" {
		t.Errorf("after release: %s %s", v.Item.Revision, v.Status)
	}
	ra.must("GET", "/api/items/100002-001?rev=01", nil, &v)
	if v.Status != "Superseded" {
		t.Errorf("rev 01 status %s", v.Status)
	}
	var cmp []map[string]any
	ra.must("GET", "/api/baselines/CO-0002?compare=CO-0001", nil, &cmp)
	if len(cmp) != 1 || cmp[0]["id"] != "100002-001" {
		t.Errorf("baseline compare: %v", cmp)
	}
	raw, _ := exec.Command("git", "--git-dir", testApp.Repo.Dir(), "cat-file", "-p", "baseline/CO-0002").CombinedOutput()
	if !strings.Contains(string(raw), "BEGIN PGP SIGNATURE") {
		t.Errorf("baseline tag not signed:\n%s", raw)
	}
}

func TestCOValidationBlocksApproval(t *testing.T) {
	e := signIn(t, "editor1")
	var out map[string]string
	e.must("POST", "/api/change-orders", map[string]any{"title": "Release panel", "reason": "r", "description": "d",
		"items": []map[string]string{{"item": "100004-201", "action": "Release"}}}, &out)
	id := out["id"]
	var us []userRow
	e.must("GET", "/api/users", nil, &us)
	ids := map[string]string{}
	for _, u := range us {
		ids[u.Username] = u.ID
	}
	e.must("PUT", "/api/change-orders/"+id+"/approvers", map[string]any{"approvers": []string{ids["appr1"]}}, nil)
	e.must("POST", "/api/change-orders/"+id+"/submit", nil, nil)
	a1 := signIn(t, "appr1")
	if st := a1.do("POST", "/api/change-orders/"+id+"/decision", map[string]any{"decision": "Approved", "password": seedPW}, nil); st != 400 {
		t.Errorf("approval with validation errors: %d", st)
	}
}

func TestWorkflowAndNCR(t *testing.T) {
	e := signIn(t, "editor1")
	if st := e.do("POST", "/api/work/TSK-0003/status", map[string]any{"status": "In Review"}, nil); st != 400 {
		t.Errorf("Done to In Review allowed: %d", st)
	}
	e.must("POST", "/api/work/TSK-0002/status", map[string]any{"status": "In Progress"}, nil)
	c := signIn(t, "contrib1")
	if st := c.do("POST", "/api/work/NCR-0001/status", map[string]any{"status": "Dispositioned"}, nil); st != 400 {
		t.Errorf("NCR dispositioned without disposition: %d", st)
	}
	if st := e.do("POST", "/api/work/NCR-0002/status", map[string]any{"status": "Corrective Action"}, nil); st != 200 {
		t.Fatalf("NCR to corrective action: %d", st)
	}
	var w map[string]any
	e.must("GET", "/api/work/NCR-0002", nil, &w)
	ver := int(w["item"].(map[string]any)["version"].(float64))
	e.must("PATCH", "/api/work/NCR-0002", map[string]any{"version": ver, "fields": map[string]any{"root_cause": "Drill jig worn", "corrective_action": "Replace jig"}}, nil)
	e.must("POST", "/api/work/NCR-0002/status", map[string]any{"status": "Verified"}, nil)
	e.must("GET", "/api/work/NCR-0002", nil, &w)
	ver = int(w["item"].(map[string]any)["version"].(float64))
	e.must("PATCH", "/api/work/NCR-0002", map[string]any{"version": ver, "fields": map[string]any{"verification": "Ten plates checked"}}, nil)
	if st := e.do("POST", "/api/work/NCR-0002/status", map[string]any{"status": "Closed"}, nil); st != 400 {
		t.Errorf("NCR closed without change order: %d", st)
	}
	var out map[string]string
	e.must("POST", "/api/work/NCR-0002/create-co", nil, &out)
	e.must("POST", "/api/work/NCR-0002/status", map[string]any{"status": "Closed"}, nil)
	v := signIn(t, "viewer1")
	if st := v.do("POST", "/api/work/TSK-0002/status", map[string]any{"status": "Open"}, nil); st != 403 {
		t.Errorf("viewer changed status: %d", st)
	}
}

func TestPagesAndSpaces(t *testing.T) {
	e := signIn(t, "editor1")
	var tree struct {
		Home  string     `json:"home"`
		Pages []treeNode `json:"pages"`
	}
	e.must("GET", "/api/spaces/SP-0001/tree", nil, &tree)
	if tree.Home != "PG-0001" || len(tree.Pages) != 4 {
		t.Errorf("tree: %+v", tree)
	}
	var pv pageView
	e.must("GET", "/api/pages/PG-0003", nil, &pv)
	if len(pv.Breadcrumbs) != 2 || pv.Breadcrumbs[0].ID != "PG-0001" || pv.Breadcrumbs[1].ID != "PG-0002" {
		t.Errorf("breadcrumbs: %+v", pv.Breadcrumbs)
	}
	e.must("GET", "/api/pages/PG-0004", nil, &pv)
	if !pv.PagePerms.EditUnderCO {
		t.Errorf("controlled page perms: %+v", pv.PagePerms)
	}
	var out map[string]string
	e.must("POST", "/api/pages/PG-0004/edit-under-co", map[string]any{}, &out)
	e.must("GET", "/api/pages/PG-0004", nil, &pv)
	if pv.Perms.EditTarget != "co" || pv.CO == nil || pv.CO.ID != out["co"] {
		t.Errorf("after edit under CO: %+v %+v", pv.Perms, pv.CO)
	}
	v := signIn(t, "viewer1")
	v.must("GET", "/api/pages/PG-0004", nil, &pv)
	if pv.Item.Revision != "01" {
		t.Errorf("viewer sees revision %q", pv.Item.Revision)
	}
	e2 := signIn(t, "editor2")
	if st := e2.do("GET", "/api/pages/PG-0006", nil, nil); st != 404 {
		t.Errorf("editor2 sees SP-0002 page: %d", st)
	}
	e.must("POST", "/api/pages", map[string]any{"space": "SP-0001", "parent": "PG-0002", "title": "Child"}, &out)
	e.must("POST", "/api/pages/"+out["id"]+"/move", map[string]any{"parent": "", "index": 0, "title": "Moved child"}, nil)
}
