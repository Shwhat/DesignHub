package model

import (
	"fmt"
	"sort"
	"strings"
)

// Issue is a validation finding. Errors block approval; warnings do not.
type Issue struct {
	Level   string `json:"level"` // error or warning
	Item    string `json:"item"`
	Find    int    `json:"find,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

func errorf(item, field, format string, args ...any) Issue {
	return Issue{Level: "error", Item: item, Field: field, Message: fmt.Sprintf(format, args...)}
}

func warnf(item, field, format string, args ...any) Issue {
	return Issue{Level: "warning", Item: item, Field: field, Message: fmt.Sprintf(format, args...)}
}

// Link types allowed on git records, with the types they may point to.
var LinkTypes = map[string]struct {
	From, To []string
	Reverse  string
}{
	"satisfies":    {[]string{TypePart}, []string{TypeRequirement}, "satisfied by"},
	"verifies":     {[]string{TypeProcedure}, []string{TypeRequirement}, "verified by"},
	"derived from": {[]string{TypeRequirement}, []string{TypeRequirement}, "derives"},
	"replaced by":  {nil, nil, "replaces"},
	"applies to":   {[]string{TypeProcedure, TypeDocument}, []string{TypePart, TypeRequirement, TypeProcedure, TypeDocument}, "has procedure or document"},
	"relates to":   {nil, nil, "relates to"},
}

// Required checks the fields every saved item must have.
func Required(it *Item) []Issue {
	var out []Issue
	req := func(field, value, label string) {
		if strings.TrimSpace(value) == "" {
			out = append(out, errorf(it.ID, field, "%s is required", label))
		}
	}
	req("title", it.Title, map[bool]string{true: "Name", false: "Title"}[it.Type == TypePart])
	req("owner", it.Owner, "Owner")
	req("space", it.Space, map[bool]string{true: "Space", false: "Home space"}[it.Type == TypePage])
	max := map[string]int{TypePart: 50, TypeRequirement: 100, TypeProcedure: 100, TypeDocument: 100, TypePage: 150}[it.Type]
	if max > 0 && len([]rune(it.Title)) > max {
		out = append(out, errorf(it.ID, "title", "Name or title must be at most %d characters", max))
	}
	switch it.Type {
	case TypePart:
		if k := PartKindOf(it.ID); k == "" || k != it.Kind {
			out = append(out, errorf(it.ID, "kind", "Item number does not match part kind %q", it.Kind))
		}
		req("unit", it.Unit, "Unit of measure")
		req("tracking", it.Tracking, "Tracking")
		if it.Kind == KindCOTS {
			req("manufacturer", it.Manufacturer, "Manufacturer")
			req("orderNumber", it.OrderNumber, "Order number")
			req("lifecycle", it.Lifecycle, "Lifecycle status")
		}
		if len(it.BOM) > 0 && !it.HasBOM() {
			out = append(out, errorf(it.ID, "bom", "Only assemblies (-2XX, -3XX, -5XX) have a BOM"))
		}
	case TypeRequirement:
		req("statement", it.Statement, "Statement")
		req("category", it.Category, "Category")
		req("priority", it.Priority, "Priority")
		if len(it.Verification) == 0 {
			out = append(out, errorf(it.ID, "verification", "Verification method is required"))
		}
	case TypeProcedure:
		req("procedureType", it.ProcedureType, "Procedure type")
		req("purpose", it.Purpose, "Purpose and scope")
		if len(it.Steps) == 0 {
			out = append(out, errorf(it.ID, "steps", "At least one step is required"))
		}
		for i, s := range it.Steps {
			if strings.TrimSpace(s.Instruction) == "" {
				out = append(out, errorf(it.ID, "steps", "Step %d has no instruction", i+1))
			}
		}
	case TypeDocument:
		req("documentType", it.DocumentType, "Document type")
	case TypePage:
		req("pageType", it.PageType, "Page type")
	default:
		out = append(out, errorf(it.ID, "type", "Unknown type %q", it.Type))
	}
	switch it.Status {
	case StatusDraft, StatusInReview, StatusReleased, StatusSuperseded, StatusObsolete:
	case "":
		if it.Type != TypePage {
			out = append(out, errorf(it.ID, "status", "Status is required"))
		}
	default:
		out = append(out, errorf(it.ID, "status", "Unknown status %q", it.Status))
	}
	if t := TypeOf(it.ID); t != it.Type {
		out = append(out, errorf(it.ID, "id", "Identifier %s does not match type %s", it.ID, it.Type))
	}
	return out
}

// Validate checks an item against the records it refers to.
func Validate(it *Item, get Lookup) []Issue {
	out := Required(it)
	seenLinks := map[string]bool{}
	for _, l := range it.Links {
		key := l.Type + " " + l.To
		if seenLinks[key] {
			out = append(out, warnf(it.ID, "links", "Duplicate link %s %s", l.Type, l.To))
		}
		seenLinks[key] = true
		lt, known := LinkTypes[l.Type]
		if !known {
			out = append(out, errorf(it.ID, "links", "Unknown link type %q", l.Type))
			continue
		}
		target := get(l.To)
		if target == nil {
			// Links to work items and change orders are checked in the database.
			if t := TypeOf(l.To); t == "" || t == TypePart || t == TypeRequirement || t == TypeProcedure || t == TypeDocument || t == TypePage {
				out = append(out, errorf(it.ID, "links", "Broken link: %s %s does not exist", l.Type, l.To))
			}
			continue
		}
		if lt.From != nil && !contains(lt.From, it.Type) {
			out = append(out, errorf(it.ID, "links", "A %s cannot have a %q link", it.Type, l.Type))
		}
		if lt.To != nil && !contains(lt.To, target.Type) {
			out = append(out, errorf(it.ID, "links", "%q cannot point to a %s", l.Type, target.Type))
		}
		if l.Type == "replaced by" && target.Type != it.Type {
			out = append(out, errorf(it.ID, "links", "Replaced by must point to a record of the same type"))
		}
	}
	if it.HasBOM() {
		finds := map[int]bool{}
		for _, l := range it.BOM {
			if finds[l.Find] {
				out = append(out, Issue{Level: "warning", Item: it.ID, Find: l.Find, Field: "bom", Message: fmt.Sprintf("Find number %d is used twice", l.Find)})
			}
			finds[l.Find] = true
			if l.Qty <= 0 {
				out = append(out, Issue{Level: "error", Item: it.ID, Find: l.Find, Field: "bom", Message: fmt.Sprintf("Line %d: quantity must be greater than zero", l.Find)})
			}
			if !CanBeBOMLine(l.Item) {
				out = append(out, Issue{Level: "error", Item: it.ID, Find: l.Find, Field: "bom", Message: fmt.Sprintf("Line %d: %s (-9XX) cannot be a BOM line", l.Find, l.Item)})
			}
			if n := RefDesCount(l.RefDes); n > 0 && float64(n) != l.Qty {
				out = append(out, Issue{Level: "warning", Item: it.ID, Find: l.Find, Field: "bom", Message: fmt.Sprintf("Line %d: %d reference designators for quantity %g", l.Find, n, l.Qty)})
			}
			child := get(l.Item)
			if child == nil {
				out = append(out, Issue{Level: "error", Item: it.ID, Find: l.Find, Field: "bom", Message: fmt.Sprintf("Line %d: %s does not exist", l.Find, l.Item)})
				continue
			}
			if child.Type != TypePart {
				out = append(out, Issue{Level: "error", Item: it.ID, Find: l.Find, Field: "bom", Message: fmt.Sprintf("Line %d: %s is not a part or assembly", l.Find, l.Item)})
			}
			if (it.Status == StatusReleased) && child.Status != StatusReleased {
				out = append(out, Issue{Level: "error", Item: it.ID, Find: l.Find, Field: "bom", Message: fmt.Sprintf("Line %d: released assembly contains %s, which is %s", l.Find, l.Item, child.Status)})
			}
		}
		if c := FindCycle(it, get); c != nil {
			out = append(out, errorf(it.ID, "bom", "Cycle: %s", strings.Join(c, " contains ")))
		}
	}
	return out
}

// FindCycle returns a path from the item back to itself, or nil.
func FindCycle(root *Item, get Lookup) []string {
	visiting := map[string]bool{}
	done := map[string]bool{}
	var path []string
	var dfs func(it *Item) []string
	dfs = func(it *Item) []string {
		visiting[it.ID] = true
		path = append(path, it.ID)
		for _, l := range it.BOM {
			if l.Item == root.ID {
				return append(append([]string(nil), path...), root.ID)
			}
			if visiting[l.Item] || done[l.Item] {
				continue
			}
			c := get(l.Item)
			if c == nil {
				continue
			}
			if p := dfs(c); p != nil {
				return p
			}
		}
		path = path[:len(path)-1]
		visiting[it.ID] = false
		done[it.ID] = true
		return nil
	}
	return dfs(root)
}

// ExpectedFileRoles returns the roles that should be attached to an item,
// given each role's "expected on" keys.
func ExpectedFileRoles(id string, roles map[string][]string) []string {
	key := FileRoleKey(id)
	var out []string
	for role, keys := range roles {
		if key != "" && contains(keys, key) {
			out = append(out, role)
		}
	}
	sort.Strings(out)
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
