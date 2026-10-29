package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var Types = []string{"requirement", "part", "assembly", "procedure", "issue", "page"}
var TypePrefix = map[string]string{"requirement": "REQ", "part": "PRT", "assembly": "ASM", "procedure": "PRC", "issue": "ISS", "page": "PG"}
var Statuses = []string{"Draft", "In Review", "Released", "Obsolete"}

const idPat = `[A-Za-z]+-\d+`

var (
	reFront   = regexp.MustCompile(`(?s)^---\r?\n(.*?)\r?\n---\r?\n?(.*)$`)
	reRef     = regexp.MustCompile(`\[\[(` + idPat + `)\]\]`)
	reBomOpen = regexp.MustCompile("```bom[ \\t]+(" + idPat + ")")
	reBomBlk  = regexp.MustCompile("```bom[ \\t]+(" + idPat + ")[ \\t]*\\r?\\n?```")
)

type Item struct {
	ID   string
	Meta map[string]interface{}
	Body string
	Path string // absolute
}

func (it *Item) S(k string) string {
	v, ok := it.Meta[k]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

type BomLine struct {
	Part string
	Qty  float64
	Ref  string
	Bad  string
}

func (it *Item) Bom() []BomLine {
	raw, ok := it.Meta["bom"].([]interface{})
	if !ok {
		return nil
	}
	var out []BomLine
	for i, l := range raw {
		m, ok := l.(map[string]interface{})
		if !ok {
			out = append(out, BomLine{Bad: fmt.Sprintf("bom line %d is not a mapping", i+1)})
			continue
		}
		bl := BomLine{Qty: 1}
		if p, ok := m["part"]; ok && p != nil {
			bl.Part = fmt.Sprint(p)
		}
		if r, ok := m["ref"]; ok && r != nil {
			bl.Ref = fmt.Sprint(r)
		}
		if q, ok := m["qty"]; ok {
			switch v := q.(type) {
			case int:
				bl.Qty = float64(v)
			case float64:
				bl.Qty = v
			default:
				f, err := strconv.ParseFloat(fmt.Sprint(v), 64)
				if err != nil {
					bl.Qty = 0
					bl.Bad = fmt.Sprintf("bom line %d has non-numeric qty", i+1)
				} else {
					bl.Qty = f
				}
			}
		}
		if bl.Part == "" && bl.Bad == "" {
			bl.Bad = fmt.Sprintf("bom line %d has no part", i+1)
		}
		out = append(out, bl)
	}
	return out
}

func HasBom(it *Item) bool { return len(it.Bom()) > 0 }

type Link struct{ Rel, Target string }

func (it *Item) Outgoing() []Link {
	var out []Link
	seen := map[Link]bool{}
	add := func(l Link) {
		if !seen[l] {
			seen[l] = true
			out = append(out, l)
		}
	}
	if links, ok := it.Meta["links"].(map[string]interface{}); ok {
		var keys []string
		for k := range links {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, rel := range keys {
			switch v := links[rel].(type) {
			case string:
				add(Link{rel, v})
			case []interface{}:
				for _, t := range v {
					if t != nil {
						add(Link{rel, fmt.Sprint(t)})
					}
				}
			}
		}
	}
	for _, b := range it.Bom() {
		if b.Part != "" {
			add(Link{"bom", b.Part})
		}
	}
	for _, m := range reRef.FindAllStringSubmatch(it.Body, -1) {
		add(Link{"mentions", m[1]})
	}
	for _, m := range reBomOpen.FindAllStringSubmatch(it.Body, -1) {
		add(Link{"embeds", m[1]})
	}
	return out
}

func ParseItem(text string) (map[string]interface{}, string, error) {
	m := reFront.FindStringSubmatch(text)
	if m == nil {
		return nil, "", errors.New("file must start with a --- frontmatter block")
	}
	meta := map[string]interface{}{}
	if err := yaml.Unmarshal([]byte(m[1]), &meta); err != nil {
		return nil, "", fmt.Errorf("frontmatter is not valid YAML: %v", err)
	}
	if meta == nil {
		meta = map[string]interface{}{}
	}
	return meta, m[2], nil
}

type FileErr struct{ Path, Msg string }

type Store struct {
	Items  map[string]*Item
	Errors []FileErr
}

func (s *Store) Sorted() []*Item {
	var out []*Item
	for _, it := range s.Items {
		out = append(out, it)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func Load(dir string) *Store {
	s := &Store{Items: map[string]*Item{}}
	var paths []string
	filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") && p != dir {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
			paths = append(paths, p)
		}
		return nil
	})
	sort.Strings(paths)
	for _, p := range paths {
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		b, err := os.ReadFile(p)
		if err != nil {
			s.Errors = append(s.Errors, FileErr{rel, "unreadable: " + err.Error()})
			continue
		}
		meta, body, err := ParseItem(string(b))
		if err != nil {
			s.Errors = append(s.Errors, FileErr{rel, err.Error()})
			continue
		}
		id := ""
		if v, ok := meta["id"]; ok && v != nil {
			id = fmt.Sprint(v)
		}
		if id == "" {
			s.Errors = append(s.Errors, FileErr{rel, "missing id"})
			continue
		}
		if _, dup := s.Items[id]; dup {
			s.Errors = append(s.Errors, FileErr{rel, "duplicate id " + id})
			continue
		}
		s.Items[id] = &Item{ID: id, Meta: meta, Body: body, Path: p}
	}
	return s
}

type BackRef struct{ Source, Rel string }

func (s *Store) Backlinks() map[string][]BackRef {
	back := map[string][]BackRef{}
	for _, it := range s.Sorted() {
		for _, l := range it.Outgoing() {
			back[l.Target] = append(back[l.Target], BackRef{it.ID, l.Rel})
		}
	}
	return back
}

type BomRow struct {
	Depth    int
	Part     string
	Item     *Item
	Qty, Ext float64
	Ref      string
}

func (s *Store) BomTree(root string) ([]BomRow, map[string]float64, []string) {
	var rows []BomRow
	parts := map[string]float64{}
	var problems []string
	var walk func(id string, mult float64, depth int, stack map[string]bool)
	walk = func(id string, mult float64, depth int, stack map[string]bool) {
		for _, l := range s.Items[id].Bom() {
			if l.Bad != "" {
				continue
			}
			ext := l.Qty * mult
			it := s.Items[l.Part]
			rows = append(rows, BomRow{depth, l.Part, it, l.Qty, ext, l.Ref})
			if it == nil {
				problems = append(problems, "BOM references missing item "+l.Part)
				continue
			}
			if stack[l.Part] {
				problems = append(problems, "BOM cycle through "+l.Part)
				continue
			}
			if HasBom(it) {
				stack[l.Part] = true
				walk(l.Part, ext, depth+1, stack)
				delete(stack, l.Part)
			} else {
				parts[l.Part] += ext
			}
		}
	}
	if _, ok := s.Items[root]; ok {
		walk(root, 1, 0, map[string]bool{root: true})
	}
	return rows, parts, problems
}

type Issue struct{ Item, Level, Msg string }

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func (s *Store) Validate() []Issue {
	var out []Issue
	for _, e := range s.Errors {
		out = append(out, Issue{"(file) " + e.Path, "error", e.Msg})
	}
	for _, it := range s.Sorted() {
		for _, f := range []string{"type", "title", "status"} {
			if it.S(f) == "" {
				out = append(out, Issue{it.ID, "error", "missing field: " + f})
			}
		}
		t := it.S("type")
		if t != "" && !contains(Types, t) {
			out = append(out, Issue{it.ID, "error", "unknown type: " + t})
		} else if t != "" && !strings.HasPrefix(it.ID, TypePrefix[t]+"-") {
			out = append(out, Issue{it.ID, "warning", fmt.Sprintf("id prefix does not match type %s (expected %s-)", t, TypePrefix[t])})
		}
		if st := it.S("status"); st != "" && !contains(Statuses, st) {
			out = append(out, Issue{it.ID, "error", "unknown status: " + st})
		}
		for _, l := range it.Outgoing() {
			tgt, ok := s.Items[l.Target]
			if !ok {
				out = append(out, Issue{it.ID, "error", fmt.Sprintf("%s points to missing item %s", l.Rel, l.Target)})
			} else if l.Rel == "bom" && it.S("status") == "Released" && tgt.S("status") != "Released" {
				out = append(out, Issue{it.ID, "error", fmt.Sprintf("Released but BOM contains non-released %s (%s)", l.Target, tgt.S("status"))})
			}
		}
		for _, b := range it.Bom() {
			if b.Bad != "" {
				out = append(out, Issue{it.ID, "error", b.Bad})
			}
		}
		if HasBom(it) {
			_, _, probs := s.BomTree(it.ID)
			seen := map[string]bool{}
			for _, p := range probs {
				if !seen[p] && strings.Contains(p, "cycle") {
					seen[p] = true
					out = append(out, Issue{it.ID, "error", p})
				}
			}
		}
	}
	return out
}

func (s *Store) NextID(typ string) string {
	pre := TypePrefix[typ]
	re := regexp.MustCompile(`^` + pre + `-(\d+)$`)
	n := 0
	for id := range s.Items {
		if m := re.FindStringSubmatch(id); m != nil {
			v, _ := strconv.Atoi(m[1])
			if v > n {
				n = v
			}
		}
	}
	return fmt.Sprintf("%s-%04d", pre, n+1)
}

func Template(id, typ, title, owner string) string {
	links := "links: {}\n"
	switch typ {
	case "assembly", "part":
		links = "links:\n  satisfies: []\n"
	case "procedure":
		links = "links:\n  verifies: []\n"
	}
	bom := ""
	if typ == "assembly" {
		bom = "bom: []\n"
	}
	t, _ := yaml.Marshal(map[string]string{"title": strings.ReplaceAll(title, "\n", " ")})
	return fmt.Sprintf("---\nid: %s\ntype: %s\n%srev: A\nstatus: Draft\nowner: %s\n%s%s---\n\nDescribe this item here.\n", id, typ, string(t), owner, links, bom)
}

var Seed = map[string]string{
	"requirement/REQ-0001.md": "---\nid: REQ-0001\ntype: requirement\ntitle: Enclosure ingress protection\nrev: A\nstatus: Released\nowner: {u}\nlinks: {}\n---\n\nThe control enclosure shall be rated NEMA 12 or better.\n",
	"requirement/REQ-0002.md": "---\nid: REQ-0002\ntype: requirement\ntitle: Emergency stop removes motor power\nrev: A\nstatus: Released\nowner: {u}\nlinks: {}\n---\n\nPressing the emergency stop shall remove power from all motor contactors within 100 ms.\n",
	"part/PRT-0001.md":        "---\nid: PRT-0001\ntype: part\ntitle: 24 V contactor, 3 pole\nrev: A\nstatus: Released\nowner: {u}\nlinks:\n  satisfies: [REQ-0002]\n---\n\nApproved vendor details go here.\n",
	"part/PRT-0002.md":        "---\nid: PRT-0002\ntype: part\ntitle: Emergency stop pushbutton, twist release\nrev: A\nstatus: Released\nowner: {u}\nlinks:\n  satisfies: [REQ-0002]\n---\n\nPart notes.\n",
	"part/PRT-0003.md":        "---\nid: PRT-0003\ntype: part\ntitle: Wall mount enclosure 600x400x200\nrev: A\nstatus: Released\nowner: {u}\nlinks:\n  satisfies: [REQ-0001]\n---\n\nPart notes.\n",
	"assembly/ASM-0001.md":    "---\nid: ASM-0001\ntype: assembly\ntitle: Motor control panel\nrev: A\nstatus: Draft\nowner: {u}\nlinks:\n  satisfies: [REQ-0001, REQ-0002]\nbom:\n  - {part: PRT-0003, qty: 1}\n  - {part: PRT-0001, qty: 2, ref: K1 K2}\n  - {part: PRT-0002, qty: 1, ref: S1}\n---\n\nTop level control panel for the line.\n",
	"procedure/PRC-0001.md":   "---\nid: PRC-0001\ntype: procedure\ntitle: Emergency stop response test\nrev: A\nstatus: Draft\nowner: {u}\nlinks:\n  verifies: [REQ-0002]\n---\n\n1. Energize the panel.\n2. Press the emergency stop.\n3. Confirm contactors open in under 100 ms.\n",
	"page/PG-0001.md":         "---\nid: PG-0001\ntype: page\ntitle: Control panel overview\nrev: A\nstatus: Draft\nowner: {u}\nlinks: {}\n---\n\n# Control panel overview\n\nThis page tracks [[ASM-0001]]. The live bill of design below updates when the assembly changes.\n\n```bom ASM-0001\n```\n\nEmergency stop behavior is verified by [[PRC-0001]].\n",
}

func SeedData(dir, user string) error {
	for rel, text := range Seed {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(strings.ReplaceAll(text, "{u}", user)), 0o644); err != nil {
			return err
		}
	}
	return nil
}
