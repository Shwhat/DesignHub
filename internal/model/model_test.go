package model

import (
	"strings"
	"testing"
)

func seedItems() map[string]*Item {
	part := func(id, title, status string) *Item {
		it := &Item{ID: id, Type: TypePart, Kind: PartKindOf(id), Title: title, Status: status, Owner: "editor1", Space: "SP-0001", Unit: "Each", Tracking: "Lot", CreatedBy: "editor1"}
		if status == StatusReleased {
			it.Revision = "01"
		}
		if it.Kind == KindCOTS {
			it.Manufacturer, it.OrderNumber, it.Lifecycle = strings.Split(id, ".")[0], strings.Split(id, ".")[1], "Active"
		}
		return it
	}
	m := map[string]*Item{}
	for _, it := range []*Item{
		part("100001-001", "Bracket, mounting", StatusReleased),
		part("100002-001", "Back plate", StatusReleased),
		part("100003-201", "DIN rail subassembly", StatusReleased),
		part("100004-201", "Control panel", StatusDraft),
		part("100005-001", "Door cover", StatusDraft),
		part("100006-901", "Mounting interface", StatusDraft),
		part("100007-001", "Shim", StatusReleased),
		part("PXC.3031212", "Terminal block", StatusReleased),
		part("SIE.3RT2015-1BB41", "Contactor", StatusReleased),
		part("RIT.8606000", "Enclosure", StatusReleased),
	} {
		m[it.ID] = it
	}
	m["100003-201"].BOM = []BOMLine{{10, "PXC.3031212", 10, nil, ""}, {20, "SIE.3RT2015-1BB41", 2, []string{"K1 K2"}, ""}, {30, "100001-001", 2, nil, ""}}
	m["100004-201"].BOM = []BOMLine{{10, "RIT.8606000", 1, nil, ""}, {20, "100002-001", 1, nil, ""}, {30, "100003-201", 3, nil, ""},
		{40, "100001-001", 4, nil, ""}, {50, "100005-001", 1, nil, ""}, {60, "100007-001", 2, nil, ""}}
	return m
}

func TestSeedBOMTotals(t *testing.T) {
	m := seedItems()
	get := func(id string) *Item { return m[id] }
	tree := Tree(m["100004-201"], get)
	flat := Flatten(tree)
	want := map[string]float64{"PXC.3031212": 30, "SIE.3RT2015-1BB41": 6, "100001-001": 10, "100003-201": 3, "100007-001": 2, "RIT.8606000": 1, "100002-001": 1, "100005-001": 1}
	if len(flat) != len(want) {
		t.Fatalf("flat has %d lines, want %d", len(flat), len(want))
	}
	for _, f := range flat {
		if want[f.ID] != f.Total {
			t.Errorf("%s total %g, want %g", f.ID, f.Total, want[f.ID])
		}
	}
	// 100001-001 under 100003-201 has extended quantity 6
	for _, c := range tree.Children {
		if c.ID == "100003-201" {
			for _, g := range c.Children {
				if g.ID == "100001-001" && g.Extended != 6 {
					t.Errorf("nested extended %g, want 6", g.Extended)
				}
			}
		}
	}
	for id, it := range m {
		for _, is := range Validate(it, get) {
			if is.Level == "error" {
				t.Errorf("seed %s: %s", id, is.Message)
			}
		}
	}
	if wu := WhereUsed("100001-001", values(m)); len(wu) != 2 {
		t.Errorf("where-used: %d", len(wu))
	}
}

func values(m map[string]*Item) []*Item {
	var out []*Item
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func TestCycleAndReleaseRule(t *testing.T) {
	m := seedItems()
	get := func(id string) *Item { return m[id] }
	m["100003-201"].BOM = append(m["100003-201"].BOM, BOMLine{Find: 40, Item: "100004-201", Qty: 1})
	if FindCycle(m["100003-201"], get) == nil {
		t.Fatal("cycle not found")
	}
	Tree(m["100004-201"], get) // must terminate
	m["100003-201"].BOM = m["100003-201"].BOM[:3]
	m["100004-201"].Status = StatusReleased
	found := false
	for _, is := range Validate(m["100004-201"], get) {
		if is.Level == "error" && strings.Contains(is.Message, "100005-001") {
			found = true
		}
	}
	if !found {
		t.Fatal("released assembly with draft child not reported")
	}
}

func TestRoundTrip(t *testing.T) {
	m := seedItems()
	it := m["100003-201"]
	it.Body = "Body with [[100001-001]] link.\n"
	b, err := it.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	b2, _ := back.Marshal()
	if string(b) != string(b2) {
		t.Fatalf("round trip differs:\n%s\n---\n%s", b, b2)
	}
	if _, err := Parse([]byte("no frontmatter")); err == nil {
		t.Fatal("file without frontmatter accepted")
	}
}

func TestIDs(t *testing.T) {
	cases := map[string]string{"100042-001": TypePart, "1000042": TypePart, "PXC.3031212": TypePart, "SIE.3RT2015-1BB41": TypePart,
		"REQ-0001": TypeRequirement, "PG-0012": TypePage, "NCR-12345": "ncr", "bogus": ""}
	for id, want := range cases {
		if got := TypeOf(id); got != want {
			t.Errorf("TypeOf(%s) = %q, want %q", id, got, want)
		}
	}
	if _, _, err := ParseProduction("100042-701"); err == nil {
		t.Error("reserved suffix accepted")
	}
	if CanBeBOMLine("100006-901") {
		t.Error("-9XX allowed in BOM")
	}
}
