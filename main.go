package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

const Version = "1.0.0"

var (
	dataDir string
	user    string
	repo    Git
	writeMu sync.Mutex
	md      = goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList))
)

func esc(s string) string { return html.EscapeString(s) }
func q(s string) string   { return url.PathEscape(s) }

func fnum(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

func badge(st string) string {
	cls := strings.ToLower(regexp.MustCompile(`\W`).ReplaceAllString(st, ""))
	return fmt.Sprintf("<span class='badge s-%s'>%s</span>", cls, esc(st))
}

func itemLink(s *Store, id string) string {
	if it, ok := s.Items[id]; ok {
		return fmt.Sprintf("<a href='/item/%s'>%s</a> %s", q(id), esc(id), esc(it.S("title")))
	}
	return fmt.Sprintf("<span class='bad'>%s (missing)</span>", esc(id))
}

// ------------------------------------------------------------ rendering

func bomHTML(s *Store, root string) string {
	if _, ok := s.Items[root]; !ok {
		return fmt.Sprintf("<p class='bad'>BOM source %s not found</p>", esc(root))
	}
	rows, parts, _ := s.BomTree(root)
	if len(rows) == 0 {
		return fmt.Sprintf("<p class='muted'>%s has no BOM lines.</p>", esc(root))
	}
	var b strings.Builder
	b.WriteString("<table><tr><th>Level</th><th>Item</th><th>Title</th><th>Rev</th><th>Status</th><th>Qty</th><th>Ext qty</th><th>Ref</th></tr>")
	for _, r := range rows {
		if r.Item == nil {
			fmt.Fprintf(&b, "<tr><td>%d</td><td class='bad'>%s</td><td colspan='6' class='bad'>missing</td></tr>", r.Depth, esc(r.Part))
			continue
		}
		fmt.Fprintf(&b, "<tr><td>%d</td><td style='padding-left:%dpx'><a href='/item/%s'>%s</a></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			r.Depth, 8+r.Depth*18, q(r.Part), esc(r.Part), esc(r.Item.S("title")), esc(r.Item.S("rev")), badge(r.Item.S("status")), fnum(r.Qty), fnum(r.Ext), esc(r.Ref))
	}
	b.WriteString("</table>")
	if len(parts) > 0 {
		var ids []string
		for id := range parts {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		fmt.Fprintf(&b, "<h4>Flattened parts for %s</h4><table><tr><th>Part</th><th>Title</th><th>Total qty</th></tr>", esc(root))
		for _, id := range ids {
			fmt.Fprintf(&b, "<tr><td><a href='/item/%s'>%s</a></td><td>%s</td><td>%s</td></tr>", q(id), esc(id), esc(s.Items[id].S("title")), fnum(parts[id]))
		}
		b.WriteString("</table>")
	}
	return b.String()
}

func renderMD(s *Store, body string) string {
	var blocks []string
	body = reBomBlk.ReplaceAllStringFunc(body, func(m string) string {
		id := reBomBlk.FindStringSubmatch(m)[1]
		blocks = append(blocks, id)
		return fmt.Sprintf("\n\nDHBOMBLOCK%dDH\n\n", len(blocks)-1)
	})
	body = reRef.ReplaceAllStringFunc(body, func(m string) string {
		id := reRef.FindStringSubmatch(m)[1]
		if _, ok := s.Items[id]; ok {
			return fmt.Sprintf("[%s](/item/%s)", id, q(id))
		}
		return fmt.Sprintf("**%s (missing)**", id)
	})
	var buf bytes.Buffer
	if err := md.Convert([]byte(body), &buf); err != nil {
		return "<pre>" + esc(body) + "</pre>"
	}
	out := buf.String()
	for i, id := range blocks {
		out = strings.Replace(out, fmt.Sprintf("<p>DHBOMBLOCK%dDH</p>", i), bomHTML(s, id), 1)
	}
	return out
}

const css = `
:root{--bg:#f6f7f9;--card:#fff;--ink:#1c2430;--mute:#6b7683;--line:#dde1e7;--acc:#1f5fbf;--bad:#b3261e;--add:#2e7d3222;--del:#b3261e22}
@media(prefers-color-scheme:dark){:root{--bg:#14181e;--card:#1b2128;--ink:#e6e9ee;--mute:#93a0ae;--line:#2c353f;--acc:#6aa5ff;--bad:#ff8a80;--add:#2e7d3240;--del:#b3261e40}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--ink);font:15px/1.5 "Segoe UI",system-ui,sans-serif}
header{background:var(--card);border-bottom:1px solid var(--line);padding:10px 24px;display:flex;gap:18px;align-items:center;flex-wrap:wrap}
header b{font-size:17px}header a{color:var(--ink);text-decoration:none}header a:hover{color:var(--acc)}
header form{margin-left:auto}main{max-width:1100px;margin:22px auto;padding:0 20px}
.card{background:var(--card);border:1px solid var(--line);border-radius:8px;padding:18px 22px;margin-bottom:18px;overflow-x:auto}
a{color:var(--acc)}h1{margin:.1em 0 .3em;font-size:24px}h2{font-size:18px;margin-top:0}h4{margin:18px 0 6px}
table{border-collapse:collapse;width:100%;margin:8px 0}th,td{border-bottom:1px solid var(--line);padding:6px 10px;text-align:left;vertical-align:top}
th{font-size:12px;text-transform:uppercase;letter-spacing:.04em;color:var(--mute)}
.muted{color:var(--mute)}.bad{color:var(--bad)}.ok{color:#2e7d32}
.badge{display:inline-block;padding:1px 9px;border-radius:10px;font-size:12px;border:1px solid var(--line)}
.s-released{background:#2e7d3222;color:#2e7d32;border-color:#2e7d32}.s-inreview{background:#b8860b22;color:#b8860b;border-color:#b8860b}
.s-draft{color:var(--mute)}.s-obsolete{color:var(--bad);border-color:var(--bad)}
.btn,button{background:var(--acc);color:#fff;border:0;border-radius:6px;padding:6px 14px;font:inherit;cursor:pointer;text-decoration:none;display:inline-block}
.btn.alt{background:transparent;color:var(--acc);border:1px solid var(--acc)}
textarea{width:100%;height:60vh;font:13px/1.45 Consolas,monospace;background:var(--bg);color:var(--ink);border:1px solid var(--line);border-radius:6px;padding:10px}
input[type=text],select{font:inherit;padding:5px 8px;background:var(--bg);color:var(--ink);border:1px solid var(--line);border-radius:6px}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:12px}
.stat{padding:12px;border:1px solid var(--line);border-radius:8px;text-decoration:none;color:inherit}.stat b{font-size:24px;display:block}
pre,code{font-family:Consolas,monospace;background:var(--bg);border-radius:4px}pre{padding:10px;overflow:auto}code{padding:1px 4px}
.diff{font:13px/1.45 Consolas,monospace;white-space:pre-wrap;margin:0}.diff div{padding:0 8px}.diff .a{background:var(--add)}.diff .d{background:var(--del)}
`

func layout(title, content, search string) string {
	nav := ""
	for _, n := range [][2]string{{"/", "Dashboard"}, {"/items", "Items"}, {"/trace", "Traceability"}, {"/check", "Validation"}, {"/new", "New item"}} {
		nav += fmt.Sprintf("<a href='%s'>%s</a>", n[0], n[1])
	}
	return fmt.Sprintf("<!doctype html><html><head><meta charset='utf-8'><meta name='viewport' content='width=device-width,initial-scale=1'>"+
		"<title>%s | DesignHub</title><style>%s</style></head><body><header><b>DesignHub</b>%s"+
		"<form action='/search'><input type='text' name='q' placeholder='Search' value='%s'></form></header><main>%s</main></body></html>",
		esc(title), css, nav, esc(search), content)
}

// ------------------------------------------------------------ views

func viewDashboard(s *Store) string {
	issues := s.Validate()
	nErr := 0
	for _, i := range issues {
		if i.Level == "error" {
			nErr++
		}
	}
	byType, byStatus := map[string]int{}, map[string]int{}
	for _, it := range s.Items {
		byType[it.S("type")]++
		byStatus[it.S("status")]++
	}
	var b strings.Builder
	b.WriteString("<h1>Dashboard</h1><div class='card'><h2>Items by type</h2><div class='grid'>")
	for _, t := range Types {
		fmt.Fprintf(&b, "<a class='stat' href='/items?type=%s'><b>%d</b>%s</a>", t, byType[t], esc(strings.Title(t)))
	}
	b.WriteString("</div></div><div class='card'><h2>Status</h2><div class='grid'>")
	for _, st := range Statuses {
		fmt.Fprintf(&b, "<a class='stat' href='/items?status=%s'><b>%d</b>%s</a>", url.QueryEscape(st), byStatus[st], esc(st))
	}
	b.WriteString("</div></div><div class='card'><h2>Validation</h2>")
	if nErr > 0 {
		fmt.Fprintf(&b, "<p class='bad'>%d errors, %d warnings. <a href='/check'>Review</a></p>", nErr, len(issues)-nErr)
	} else {
		fmt.Fprintf(&b, "<p class='ok'>No errors. %d warnings. <a href='/check'>Details</a></p>", len(issues))
	}
	head := repo.Head()
	if len(head) > 8 {
		head = head[:8]
	}
	fmt.Fprintf(&b, "</div><div class='card'><h2>Storage</h2><p class='muted'>Data folder: %s<br>Version history: embedded git, current commit %s<br>DesignHub %s</p></div>", esc(dataDir), esc(head), Version)
	return layout("Dashboard", b.String(), "")
}

func options(list []string, sel string) string {
	out := "<option value=''>All</option>"
	for _, v := range list {
		s := ""
		if v == sel {
			s = " selected"
		}
		out += fmt.Sprintf("<option%s>%s</option>", s, esc(v))
	}
	return out
}

func viewItems(s *Store, typ, status string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<h1>Items</h1><div class='card'><form method='get'>Type <select name='type'>%s</select> Status <select name='status'>%s</select> <button>Filter</button></form>", options(Types, typ), options(Statuses, status))
	b.WriteString("<table><tr><th>ID</th><th>Title</th><th>Type</th><th>Rev</th><th>Status</th><th>Owner</th></tr>")
	n := 0
	for _, it := range s.Sorted() {
		if (typ != "" && it.S("type") != typ) || (status != "" && it.S("status") != status) {
			continue
		}
		n++
		fmt.Fprintf(&b, "<tr><td><a href='/item/%s'>%s</a></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>",
			q(it.ID), esc(it.ID), esc(it.S("title")), esc(it.S("type")), esc(it.S("rev")), badge(it.S("status")), esc(it.S("owner")))
	}
	fmt.Fprintf(&b, "</table><p class='muted'>%d items</p></div>", n)
	return layout("Items", b.String(), "")
}

func viewItem(s *Store, id string) string {
	it := s.Items[id]
	var b strings.Builder
	fmt.Fprintf(&b, "<div class='card'><span class='muted'>%s</span><h1>%s</h1><p>%s &nbsp; Rev %s &nbsp; %s &nbsp; Owner: %s</p>",
		esc(id), esc(it.S("title")), esc(it.S("type")), esc(it.S("rev")), badge(it.S("status")), esc(it.S("owner")))
	fmt.Fprintf(&b, "<a class='btn' href='/edit/%s'>Edit</a> <a class='btn alt' href='/history/%s'>History</a></div>", q(id), q(id))
	fmt.Fprintf(&b, "<div class='card'>%s</div>", renderMD(s, it.Body))
	if HasBom(it) {
		fmt.Fprintf(&b, "<div class='card'><h2>Bill of design</h2>%s</div>", bomHTML(s, id))
	}
	if out := it.Outgoing(); len(out) > 0 {
		b.WriteString("<div class='card'><h2>Links</h2><table><tr><th>Relation</th><th>Item</th><th>Status</th></tr>")
		for _, l := range out {
			st := ""
			if t, ok := s.Items[l.Target]; ok {
				st = badge(t.S("status"))
			}
			fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td></tr>", esc(l.Rel), itemLink(s, l.Target), st)
		}
		b.WriteString("</table></div>")
	}
	b.WriteString("<div class='card'><h2>Referenced by (where used)</h2>")
	if refs := s.Backlinks()[id]; len(refs) > 0 {
		b.WriteString("<table><tr><th>Item</th><th>Relation</th><th>Status</th></tr>")
		for _, r := range refs {
			fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td></tr>", itemLink(s, r.Source), esc(r.Rel), badge(s.Items[r.Source].S("status")))
		}
		b.WriteString("</table>")
	} else {
		b.WriteString("<p class='muted'>Nothing references this item.</p>")
	}
	b.WriteString("</div>")
	var mine []Issue
	for _, i := range s.Validate() {
		if i.Item == id {
			mine = append(mine, i)
		}
	}
	if len(mine) > 0 {
		b.WriteString("<div class='card'><h2>Validation</h2>")
		for _, i := range mine {
			cls := "muted"
			if i.Level == "error" {
				cls = "bad"
			}
			fmt.Fprintf(&b, "<p class='%s'>%s: %s</p>", cls, i.Level, esc(i.Msg))
		}
		b.WriteString("</div>")
	}
	return layout(id, b.String(), "")
}

func hashText(t string) string { h := sha1.Sum([]byte(t)); return hex.EncodeToString(h[:]) }

func viewEdit(id, text, base, errMsg string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<h1>Edit %s</h1>", esc(id))
	if errMsg != "" {
		fmt.Fprintf(&b, "<div class='card bad'>%s</div>", esc(errMsg))
	}
	fmt.Fprintf(&b, "<form method='post' action='/save/%s' class='card'><input type='hidden' name='base' value='%s'><textarea name='text' spellcheck='false'>%s</textarea>"+
		"<p>Change note <input type='text' name='note' size='50' placeholder='What changed and why'> <button>Save</button> <a class='btn alt' href='/item/%s'>Cancel</a></p></form>",
		q(id), esc(base), esc(text), q(id))
	return layout("Edit "+id, b.String(), "")
}

func relPath(p string) string {
	r, _ := filepath.Rel(dataDir, p)
	return filepath.ToSlash(r)
}

func viewHistory(s *Store, id, sel string) string {
	it := s.Items[id]
	hist, err := repo.FileHistory(relPath(it.Path), 200)
	var b strings.Builder
	fmt.Fprintf(&b, "<h1>History of %s</h1><div class='card'>", esc(id))
	if len(hist) == 0 {
		b.WriteString("<p class='muted'>No commits yet.</p>")
	} else {
		b.WriteString("<table><tr><th>Commit</th><th>When</th><th>Author</th><th>Note</th><th></th></tr>")
		for _, h := range hist {
			fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td><td><a href='/history/%s?c=%s'>Changes</a></td></tr>",
				esc(h.ID[:8]), h.When.Format("2006-01-02 15:04"), esc(h.Author), esc(h.Message), q(id), h.ID)
		}
		b.WriteString("</table>")
	}
	if err != nil {
		fmt.Fprintf(&b, "<p class='muted'>Older history not shown: %s</p>", esc(err.Error()))
	}
	b.WriteString("</div>")
	for _, h := range hist {
		if h.ID == sel {
			title := "Changes in " + h.ID[:8]
			if h.PrevBlob == "" {
				title += " (first version)"
			}
			fmt.Fprintf(&b, "<div class='card'><h2>%s</h2><p class='muted'>%s, %s: %s</p><div class='diff'>%s</div></div>",
				esc(title), esc(h.Author), h.When.Format("2006-01-02 15:04"), esc(h.Message), diffHTML(repo.ReadBlob(h.PrevBlob), repo.ReadBlob(h.Blob)))
		}
	}
	fmt.Fprintf(&b, "<a class='btn alt' href='/item/%s'>Back</a>", q(id))
	return layout("History "+id, b.String(), "")
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), "\n"), "\n")
}

func diffHTML(a, b string) string {
	x, y := splitLines(a), splitLines(b)
	if len(x)*len(y) > 4_000_000 {
		return "<div class='muted'>File too large to compare.</div>"
	}
	n, m := len(x), len(y)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if x[i] == y[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var out strings.Builder
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && x[i] == y[j]:
			fmt.Fprintf(&out, "<div>  %s</div>", esc(x[i]))
			i++
			j++
		case j < m && (i == n || lcs[i][j+1] >= lcs[i+1][j]):
			fmt.Fprintf(&out, "<div class='a'>+ %s</div>", esc(y[j]))
			j++
		default:
			fmt.Fprintf(&out, "<div class='d'>- %s</div>", esc(x[i]))
			i++
		}
	}
	return out.String()
}

func viewNew(msg string) string {
	var b strings.Builder
	b.WriteString("<h1>New item</h1>")
	if msg != "" {
		fmt.Fprintf(&b, "<div class='card bad'>%s</div>", esc(msg))
	}
	b.WriteString("<form method='post' action='/create' class='card'><p>Type <select name='type'>")
	for _, t := range Types {
		fmt.Fprintf(&b, "<option>%s</option>", t)
	}
	b.WriteString("</select></p><p>Title <input type='text' name='title' size='60'></p><button>Create</button></form>")
	return layout("New item", b.String(), "")
}

func viewCheck(s *Store) string {
	issues := s.Validate()
	var b strings.Builder
	b.WriteString("<h1>Validation</h1><div class='card'>")
	if len(issues) == 0 {
		fmt.Fprintf(&b, "<p class='ok'>No problems found across %d items.</p>", len(s.Items))
	} else {
		sort.SliceStable(issues, func(i, j int) bool {
			if (issues[i].Level == "error") != (issues[j].Level == "error") {
				return issues[i].Level == "error"
			}
			return issues[i].Item < issues[j].Item
		})
		b.WriteString("<table><tr><th>Item</th><th>Level</th><th>Problem</th></tr>")
		for _, i := range issues {
			idl := esc(i.Item)
			if _, ok := s.Items[i.Item]; ok {
				idl = fmt.Sprintf("<a href='/item/%s'>%s</a>", q(i.Item), esc(i.Item))
			}
			cls := "muted"
			if i.Level == "error" {
				cls = "bad"
			}
			fmt.Fprintf(&b, "<tr><td>%s</td><td class='%s'>%s</td><td>%s</td></tr>", idl, cls, i.Level, esc(i.Msg))
		}
		b.WriteString("</table>")
	}
	b.WriteString("</div>")
	return layout("Validation", b.String(), "")
}

func viewTrace(s *Store) string {
	back := s.Backlinks()
	var b strings.Builder
	b.WriteString("<h1>Requirement traceability</h1><div class='card'><table><tr><th>Requirement</th><th>Status</th><th>Satisfied by</th><th>Verified by</th><th>Coverage</th></tr>")
	for _, it := range s.Sorted() {
		if it.S("type") != "requirement" {
			continue
		}
		var sat, ver []string
		for _, r := range back[it.ID] {
			switch r.Rel {
			case "satisfies":
				sat = append(sat, itemLink(s, r.Source))
			case "verifies":
				ver = append(ver, itemLink(s, r.Source))
			}
		}
		cov := "<span class='ok'>Complete</span>"
		if len(sat) == 0 {
			cov = "<span class='bad'>No design</span>"
		} else if len(ver) == 0 {
			cov = "<span class='bad'>No test</span>"
		}
		join := func(l []string) string {
			if len(l) == 0 {
				return "-"
			}
			return strings.Join(l, "<br>")
		}
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", itemLink(s, it.ID), badge(it.S("status")), join(sat), join(ver), cov)
	}
	b.WriteString("</table></div>")
	return layout("Traceability", b.String(), "")
}

func viewSearch(s *Store, term string) string {
	t := strings.ToLower(strings.TrimSpace(term))
	var b strings.Builder
	b.WriteString("<h1>Search</h1><div class='card'><table><tr><th>ID</th><th>Title</th><th>Type</th></tr>")
	n := 0
	if t != "" {
		for _, it := range s.Sorted() {
			if strings.Contains(strings.ToLower(it.ID), t) || strings.Contains(strings.ToLower(it.S("title")), t) || strings.Contains(strings.ToLower(it.Body), t) {
				n++
				fmt.Fprintf(&b, "<tr><td><a href='/item/%s'>%s</a></td><td>%s</td><td>%s</td></tr>", q(it.ID), esc(it.ID), esc(it.S("title")), esc(it.S("type")))
			}
		}
	}
	fmt.Fprintf(&b, "</table><p class='muted'>%d results</p></div>", n)
	return layout("Search", b.String(), term)
}

// ------------------------------------------------------------ handlers

func send(w http.ResponseWriter, code int, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src 'self' data:; form-action 'self'")
	w.WriteHeader(code)
	w.Write([]byte(body))
}

func notFound(w http.ResponseWriter, what string) {
	send(w, 404, layout("Not found", "<h1>"+esc(what)+" not found</h1>", ""))
}

func handler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if e := recover(); e != nil {
			send(w, 500, layout("Error", fmt.Sprintf("<h1>Error</h1><pre>%s</pre>", esc(fmt.Sprint(e))), ""))
		}
	}()
	// Only answer requests addressed to this machine (blocks DNS rebinding).
	if h, _, err := net.SplitHostPort(r.Host); err != nil || (h != "127.0.0.1" && h != "localhost") {
		send(w, 403, "forbidden")
		return
	}
	parts := strings.SplitN(strings.Trim(r.URL.Path, "/"), "/", 2)
	route, id := parts[0], ""
	if len(parts) > 1 {
		id = parts[1]
	}
	s := Load(dataDir)
	if r.Method == http.MethodPost {
		if o := r.Header.Get("Origin"); o != "" && !strings.HasPrefix(o, "http://127.0.0.1:") && !strings.HasPrefix(o, "http://localhost:") {
			send(w, 403, "forbidden")
			return
		}
		r.ParseForm()
		switch route {
		case "save":
			save(w, r, s, id)
		case "create":
			create(w, r, s)
		default:
			notFound(w, "Page")
		}
		return
	}
	switch route {
	case "":
		send(w, 200, viewDashboard(s))
	case "items":
		send(w, 200, viewItems(s, r.URL.Query().Get("type"), r.URL.Query().Get("status")))
	case "trace":
		send(w, 200, viewTrace(s))
	case "check":
		send(w, 200, viewCheck(s))
	case "new":
		send(w, 200, viewNew(""))
	case "search":
		send(w, 200, viewSearch(s, r.URL.Query().Get("q")))
	case "item", "edit", "history":
		if _, ok := s.Items[id]; !ok {
			notFound(w, "Item "+id)
			return
		}
		switch route {
		case "item":
			send(w, 200, viewItem(s, id))
		case "edit":
			b, err := os.ReadFile(s.Items[id].Path)
			if err != nil {
				send(w, 500, layout("Error", esc(err.Error()), ""))
				return
			}
			send(w, 200, viewEdit(id, string(b), hashText(string(b)), ""))
		default:
			send(w, 200, viewHistory(s, id, r.URL.Query().Get("c")))
		}
	default:
		notFound(w, "Page")
	}
}

func writeFile(p, text string) error {
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(text), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func save(w http.ResponseWriter, r *http.Request, s *Store, id string) {
	it, ok := s.Items[id]
	if !ok {
		notFound(w, "Item "+id)
		return
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	text := strings.ReplaceAll(r.FormValue("text"), "\r\n", "\n")
	base := r.FormValue("base")
	current, _ := os.ReadFile(it.Path)
	if base != "" && hashText(string(current)) != base {
		send(w, 409, viewEdit(id, text, hashText(string(current)), "Not saved: this item was changed by someone else after you opened it. Copy your edits, reload the editor, and apply them again."))
		return
	}
	meta, _, err := ParseItem(text)
	if err == nil && fmt.Sprint(meta["id"]) != id {
		err = fmt.Errorf("the id must stay %s", id)
	}
	if err != nil {
		send(w, 400, viewEdit(id, text, base, "Not saved: "+err.Error()))
		return
	}
	if err := writeFile(it.Path, text); err != nil {
		send(w, 500, viewEdit(id, text, base, "Not saved: "+err.Error()))
		return
	}
	note := strings.TrimSpace(r.FormValue("note"))
	if note == "" {
		note = "Update " + id
	}
	if _, err := repo.CommitAll(user, id+": "+note); err != nil {
		log.Println("commit failed:", err)
	}
	http.Redirect(w, r, "/item/"+q(id), http.StatusSeeOther)
}

func create(w http.ResponseWriter, r *http.Request, s *Store) {
	writeMu.Lock()
	defer writeMu.Unlock()
	typ, title := r.FormValue("type"), strings.TrimSpace(r.FormValue("title"))
	if _, ok := TypePrefix[typ]; !ok || title == "" {
		send(w, 400, viewNew("Choose a type and enter a title."))
		return
	}
	s = Load(dataDir)
	id := s.NextID(typ)
	p := filepath.Join(dataDir, typ, id+".md")
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := writeFile(p, Template(id, typ, title, user)); err != nil {
		send(w, 500, viewNew(err.Error()))
		return
	}
	if _, err := repo.CommitAll(user, id+": created"); err != nil {
		log.Println("commit failed:", err)
	}
	http.Redirect(w, r, "/edit/"+q(id), http.StatusSeeOther)
}

func openBrowser(u string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		cmd = exec.Command("open", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	cmd.Start()
}

func safeName(s string) string {
	s = regexp.MustCompile(`[^A-Za-z0-9._-]`).ReplaceAllString(s, "")
	if s == "" {
		return "user"
	}
	return s
}

func main() {
	exe, _ := os.Executable()
	defData := filepath.Join(filepath.Dir(exe), "data")
	flag.StringVar(&dataDir, "data", defData, "data folder")
	port := flag.Int("port", 8765, "first port to try")
	noBrowser := flag.Bool("no-browser", false, "do not open a browser")
	flag.Parse()
	dataDir, _ = filepath.Abs(dataDir)

	user = os.Getenv("USERNAME")
	if user == "" {
		user = os.Getenv("USER")
	}
	user = safeName(user)

	fail := func(err error) {
		fmt.Println("Error:", err)
		fmt.Println("Press Enter to close.")
		fmt.Scanln()
		os.Exit(1)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fail(err)
	}
	repo = Git{dataDir}
	if len(Load(dataDir).Items) == 0 && len(Load(dataDir).Errors) == 0 {
		if err := SeedData(dataDir, user); err != nil {
			fail(err)
		}
	}
	if err := repo.Init(); err != nil {
		fail(err)
	}
	if repo.Head() == "" {
		if _, err := repo.CommitAll(user, "Initial import"); err != nil {
			fail(err)
		}
	}

	var ln net.Listener
	var err error
	for p := *port; p < *port+20; p++ {
		ln, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			break
		}
	}
	if ln == nil {
		fail(fmt.Errorf("no free port found: %v", err))
	}
	u := fmt.Sprintf("http://127.0.0.1:%d/", ln.Addr().(*net.TCPAddr).Port)
	fmt.Printf("DesignHub %s running at %s\nData folder: %s\nClose this window to stop.\n", Version, u, dataDir)
	if !*noBrowser {
		go func() { time.Sleep(500 * time.Millisecond); openBrowser(u) }()
	}
	srv := &http.Server{Handler: http.HandlerFunc(handler), ReadHeaderTimeout: 10 * time.Second}
	fail(srv.Serve(ln))
}
