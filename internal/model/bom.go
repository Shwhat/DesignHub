package model

import (
	"sort"
	"strings"
)

// Lookup returns the current file of a record by id, or nil.
type Lookup func(id string) *Item

// Node is a line in a BOM tree. The root node is the assembly itself.
type Node struct {
	Find      int      `json:"find"`
	ID        string   `json:"id"`
	Revision  string   `json:"revision"`
	Title     string   `json:"title"`
	Status    string   `json:"status"`
	Qty       float64  `json:"qty"`
	Unit      string   `json:"unit"`
	RefDes    []string `json:"refdes"`
	Notes     string   `json:"notes"`
	Extended  float64  `json:"extended"`
	Level     int      `json:"level"`
	Missing   bool     `json:"missing,omitempty"`
	Cycle     bool     `json:"cycle,omitempty"`
	Hidden    bool     `json:"hidden,omitempty"`
	Assembly  bool     `json:"assembly,omitempty"`
	Children  []*Node  `json:"children,omitempty"`
	Issues    []string `json:"issues,omitempty"`
}

// Tree builds the multi-level BOM of an assembly. Extended quantities
// multiply down the levels. A part used in several sub-assemblies appears
// under each one. Cycles are cut and flagged.
func Tree(root *Item, get Lookup) *Node {
	n := &Node{ID: root.ID, Revision: root.Revision, Title: root.Title, Status: root.Status, Qty: 1, Unit: root.Unit, Extended: 1, Assembly: root.HasBOM()}
	build(n, root, get, map[string]bool{root.ID: true}, 1)
	return n
}

func build(parent *Node, it *Item, get Lookup, path map[string]bool, mult float64) {
	lines := append([]BOMLine(nil), it.BOM...)
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Find < lines[j].Find })
	for _, l := range lines {
		c := &Node{Find: l.Find, ID: l.Item, Qty: l.Qty, RefDes: l.RefDes, Notes: l.Notes, Extended: l.Qty * mult, Level: parent.Level + 1}
		child := get(l.Item)
		if child == nil {
			c.Missing = true
			c.Issues = append(c.Issues, "Item "+l.Item+" does not exist")
		} else {
			c.Revision, c.Title, c.Status, c.Unit, c.Assembly = child.Revision, child.Title, child.Status, child.Unit, child.HasBOM()
			if path[l.Item] {
				c.Cycle = true
				c.Issues = append(c.Issues, "Cycle: "+l.Item+" contains itself")
			} else if child.HasBOM() {
				path[l.Item] = true
				build(c, child, get, path, c.Extended)
				delete(path, l.Item)
			}
		}
		parent.Children = append(parent.Children, c)
	}
}

// FlatLine is one unique item in a flattened BOM.
type FlatLine struct {
	ID       string  `json:"id"`
	Revision string  `json:"revision"`
	Title    string  `json:"title"`
	Status   string  `json:"status"`
	Unit     string  `json:"unit"`
	Total    float64 `json:"total"`
	Assembly bool    `json:"assembly,omitempty"`
	Hidden   bool    `json:"hidden,omitempty"`
}

// Flatten lists every unique item below the root once with its total
// quantity across all levels.
func Flatten(tree *Node) []FlatLine {
	idx := map[string]*FlatLine{}
	var order []string
	var walk func(n *Node)
	walk = func(n *Node) {
		for _, c := range n.Children {
			f := idx[c.ID]
			if f == nil {
				f = &FlatLine{ID: c.ID, Revision: c.Revision, Title: c.Title, Status: c.Status, Unit: c.Unit, Assembly: c.Assembly, Hidden: c.Hidden}
				idx[c.ID] = f
				order = append(order, c.ID)
			}
			f.Total += c.Extended
			if !c.Cycle {
				walk(c)
			}
		}
	}
	walk(tree)
	out := make([]FlatLine, 0, len(order))
	for _, id := range order {
		out = append(out, *idx[id])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Hide marks lines whose item the viewer cannot see. Their quantities stay
// in the totals of the visible lines' parents, and the line itself is
// removed from the tree shown to the viewer.
func Hide(n *Node, visible func(id string) bool) {
	var keep []*Node
	for _, c := range n.Children {
		if !visible(c.ID) {
			continue
		}
		Hide(c, visible)
		keep = append(keep, c)
	}
	n.Children = keep
}

// WhereUsed returns the assemblies whose BOM contains id directly.
func WhereUsed(id string, all []*Item) []*Item {
	var out []*Item
	for _, it := range all {
		for _, l := range it.BOM {
			if l.Item == id {
				out = append(out, it)
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Ancestors returns every assembly that contains id at any level.
func Ancestors(id string, all []*Item) []string {
	parents := map[string][]string{}
	for _, it := range all {
		for _, l := range it.BOM {
			parents[l.Item] = append(parents[l.Item], it.ID)
		}
	}
	seen := map[string]bool{}
	var out []string
	queue := []string{id}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, p := range parents[cur] {
			if !seen[p] && p != id {
				seen[p] = true
				out = append(out, p)
				queue = append(queue, p)
			}
		}
	}
	sort.Strings(out)
	return out
}

// NextFind returns the next find number in steps of 10.
func NextFind(lines []BOMLine) int {
	max := 0
	for _, l := range lines {
		if l.Find > max {
			max = l.Find
		}
	}
	return (max/10 + 1) * 10
}

// RefDesCount counts reference designators, accepting space or comma lists.
func RefDesCount(rd []string) int {
	n := 0
	for _, r := range rd {
		n += len(strings.FieldsFunc(r, func(c rune) bool { return c == ' ' || c == ',' }))
	}
	return n
}
