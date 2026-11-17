package model

import (
	"fmt"
	"regexp"
	"strings"
)

// Page forms. A template can define fields; a page created from it keeps
// the layout, the field list and the values in its frontmatter, so the page
// can be edited through the same form later. The page body is the markdown
// generated from the layout and the values, so the file stays readable
// without DesignHub.

// FormField is one field of a page form. Columns are the columns of a table
// field and use the column types.
type FormField struct {
	Key      string      `yaml:"key" json:"key"`
	Label    string      `yaml:"label" json:"label"`
	Type     string      `yaml:"type" json:"type"`
	Required bool        `yaml:"required,omitempty" json:"required,omitempty"`
	Options  []string    `yaml:"options,omitempty" json:"options,omitempty"`
	Default  string      `yaml:"default,omitempty" json:"default,omitempty"`
	Help     string      `yaml:"help,omitempty" json:"help,omitempty"`
	Kinds    string      `yaml:"kinds,omitempty" json:"kinds,omitempty"`
	Columns  []FormField `yaml:"columns,omitempty" json:"columns,omitempty"`
}

// PageForm is the form stored on a page.
type PageForm struct {
	Layout string         `yaml:"layout" json:"layout"`
	Fields []FormField    `yaml:"fields" json:"fields"`
	Values map[string]any `yaml:"values,omitempty" json:"values"`
}

// FieldTypes are the types of form fields.
var FieldTypes = []string{"text", "textarea", "date", "user", "users", "select", "toggle", "record", "list", "table"}

// ColumnTypes are the types of table columns.
var ColumnTypes = []string{"index", "text", "date", "user", "select", "toggle", "record"}

var (
	fieldKey   = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)
	fieldToken = regexp.MustCompile(`\{\{\s*([a-z][a-z0-9-]*)\s*\}\}`)
)

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func checkField(f FormField, types []string, what string) error {
	if strings.TrimSpace(f.Label) == "" {
		return fmt.Errorf("Every %s needs a label", what)
	}
	if !fieldKey.MatchString(f.Key) {
		return fmt.Errorf("%s %q: the key must start with a letter and use only lowercase letters, digits and hyphens", f.Label, f.Key)
	}
	if !has(types, f.Type) {
		return fmt.Errorf("%s: unknown type %q", f.Label, f.Type)
	}
	if f.Type == "select" {
		n := 0
		for _, o := range f.Options {
			if strings.TrimSpace(o) != "" {
				n++
			}
		}
		if n < 2 {
			return fmt.Errorf("%s: a dropdown needs at least two choices", f.Label)
		}
	}
	return nil
}

// ValidateFields checks a field list and, when layout is not empty, that
// every field is placed in the layout and every token names a field.
func ValidateFields(fields []FormField, layout string) error {
	seen := map[string]bool{}
	byKey := map[string]FormField{}
	for _, f := range fields {
		if err := checkField(f, FieldTypes, "field"); err != nil {
			return err
		}
		if seen[f.Key] {
			return fmt.Errorf("The key %q is used twice", f.Key)
		}
		seen[f.Key] = true
		byKey[f.Key] = f
		if f.Type == "table" {
			if len(f.Columns) == 0 {
				return fmt.Errorf("%s: a table needs at least one column", f.Label)
			}
			cols := map[string]bool{}
			for _, c := range f.Columns {
				if err := checkField(c, ColumnTypes, "column"); err != nil {
					return fmt.Errorf("%s: %w", f.Label, err)
				}
				if cols[c.Key] {
					return fmt.Errorf("%s: the column key %q is used twice", f.Label, c.Key)
				}
				cols[c.Key] = true
			}
		} else if len(f.Columns) > 0 {
			return fmt.Errorf("%s: only tables have columns", f.Label)
		}
	}
	if layout == "" {
		return nil
	}
	placed := map[string]bool{}
	for _, line := range strings.Split(layout, "\n") {
		for _, m := range fieldToken.FindAllStringSubmatch(line, -1) {
			f, ok := byKey[m[1]]
			if !ok {
				return fmt.Errorf("The layout uses {{%s}}, which is not a field", m[1])
			}
			placed[m[1]] = true
			if f.Type == "table" && strings.TrimSpace(line) != m[0] {
				return fmt.Errorf("{{%s}} is a table and must be on a line of its own", m[1])
			}
		}
	}
	for _, f := range fields {
		if !placed[f.Key] {
			return fmt.Errorf("%s is not placed in the layout. Add {{%s}} to the body", f.Label, f.Key)
		}
	}
	return nil
}

// empty reports whether a stored value counts as not filled in.
func empty(typ string, v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	case bool:
		return !x
	case []any:
		for _, e := range x {
			if !empty("", e) {
				return false
			}
		}
		return true
	case map[string]any:
		if typ == "record" || x["id"] != nil {
			return empty("", x["id"])
		}
		for _, e := range x {
			if !empty("", e) {
				return false
			}
		}
		return true
	}
	return false
}

// Missing returns a message for every required field or column without a
// value. A table row with nothing filled in is ignored.
func (f *PageForm) Missing() []string {
	var out []string
	for _, fd := range f.Fields {
		v := f.Values[fd.Key]
		if fd.Type != "table" {
			if fd.Required && empty(fd.Type, v) {
				out = append(out, fd.Label+" is required")
			}
			continue
		}
		rows, _ := v.([]any)
		filled := 0
		for i, r := range rows {
			row, _ := r.(map[string]any)
			if row == nil {
				continue
			}
			used := false
			for _, c := range fd.Columns {
				if c.Type != "index" && !empty(c.Type, row[c.Key]) {
					used = true
				}
			}
			if !used {
				continue
			}
			filled++
			for _, c := range fd.Columns {
				if c.Required && c.Type != "index" && empty(c.Type, row[c.Key]) {
					out = append(out, fmt.Sprintf("%s row %d: %s is required", fd.Label, i+1, c.Label))
				}
			}
		}
		if fd.Required && filled == 0 {
			out = append(out, fd.Label+" needs at least one row")
		}
	}
	return out
}
