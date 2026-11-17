package model

import (
	"strings"
	"testing"
)

func reviewFields() []FormField {
	return []FormField{
		{Key: "date", Label: "Date", Type: "date", Required: true},
		{Key: "decision", Label: "Decision", Type: "select", Required: true, Options: []string{"Approved", "Not approved"}},
		{Key: "attendees", Label: "Attendees", Type: "users", Required: true},
		{Key: "follow-up", Label: "Follow-up", Type: "toggle"},
		{Key: "findings", Label: "Findings", Type: "table", Required: true, Columns: []FormField{
			{Key: "n", Label: "#", Type: "index"},
			{Key: "finding", Label: "Finding", Type: "text", Required: true},
			{Key: "severity", Label: "Severity", Type: "select", Options: []string{"Major", "Minor"}},
		}},
	}
}

const reviewLayout = "# {title}\n\n| Date | {{date}} |\n\n{{decision}}\n\n{{attendees}}\n\nFollow-up: {{follow-up}}\n\n{{findings}}\n"

func TestValidateFields(t *testing.T) {
	if err := ValidateFields(reviewFields(), reviewLayout); err != nil {
		t.Fatal(err)
	}
	bad := []struct {
		name   string
		edit   func(f []FormField) []FormField
		layout string
		want   string
	}{
		{"bad key", func(f []FormField) []FormField { f[0].Key = "Date"; return f }, "", "key must start"},
		{"duplicate key", func(f []FormField) []FormField { f[1].Key = "date"; return f }, "", "used twice"},
		{"unknown type", func(f []FormField) []FormField { f[0].Type = "colour"; return f }, "", "unknown type"},
		{"one choice", func(f []FormField) []FormField { f[1].Options = []string{"Approved"}; return f }, "", "two choices"},
		{"no columns", func(f []FormField) []FormField { f[4].Columns = nil; return f }, "", "at least one column"},
		{"table column type", func(f []FormField) []FormField { f[4].Columns[1].Type = "table"; return f }, "", "unknown type"},
		{"unknown token", func(f []FormField) []FormField { return f }, reviewLayout + "{{other}}\n", "not a field"},
		{"not placed", func(f []FormField) []FormField { return f }, strings.Replace(reviewLayout, "{{decision}}", "", 1), "not placed"},
		{"inline table", func(f []FormField) []FormField { return f }, strings.Replace(reviewLayout, "\n{{findings}}", "\nSee {{findings}}", 1), "line of its own"},
	}
	for _, c := range bad {
		err := ValidateFields(c.edit(reviewFields()), c.layout)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v", c.name, err)
		}
	}
}

func TestFormMissing(t *testing.T) {
	f := &PageForm{Layout: reviewLayout, Fields: reviewFields(), Values: map[string]any{}}
	if m := f.Missing(); len(m) != 4 {
		t.Errorf("empty form: %v", m)
	}
	f.Values = map[string]any{
		"date":      "2026-11-17",
		"decision":  "Approved",
		"attendees": []any{"Devon"},
		"findings": []any{
			map[string]any{"finding": "Guard missing", "severity": "Major"},
			map[string]any{"finding": "", "severity": ""}, // blank row is ignored
		},
	}
	if m := f.Missing(); len(m) != 0 {
		t.Errorf("filled form: %v", m)
	}
	f.Values["findings"] = []any{map[string]any{"severity": "Minor"}}
	if m := f.Missing(); len(m) != 1 || !strings.Contains(m[0], "row 1: Finding") {
		t.Errorf("row without required column: %v", m)
	}
}

func TestPageFormRoundTrip(t *testing.T) {
	p := &Item{ID: "PG-0001", Type: TypePage, Title: "Review", Owner: "devon", CreatedBy: "devon", Space: "SP-0001", PageType: "Design review",
		Body: "# Review\n", Form: &PageForm{Layout: reviewLayout, Fields: reviewFields(), Values: map[string]any{
			"date": "2026-11-17", "decision": "Approved", "attendees": []any{"Devon", "Alex"}, "follow-up": true,
			"findings": []any{map[string]any{"finding": "Guard | missing", "severity": "Major"}},
		}}}
	b, err := p.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	q, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if q.Form == nil || q.Form.Layout != reviewLayout || len(q.Form.Fields) != 5 || q.Form.Values["follow-up"] != true {
		t.Fatalf("round trip: %+v", q.Form)
	}
	b2, _ := q.Marshal()
	if string(b) != string(b2) {
		t.Error("marshal is not stable")
	}
	if issues := errorsOnly(Required(q)); len(issues) != 0 {
		t.Errorf("required: %v", issues)
	}
	q.Form.Values["decision"] = ""
	if issues := errorsOnly(Required(q)); len(issues) != 1 {
		t.Errorf("missing decision: %v", issues)
	}
}

func errorsOnly(in []Issue) []Issue {
	var out []Issue
	for _, i := range in {
		if i.Level == "error" {
			out = append(out, i)
		}
	}
	return out
}
