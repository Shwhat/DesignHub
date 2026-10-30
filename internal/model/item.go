// Package model defines the records stored in git (engineering items and
// pages), their file format, identifiers, Bill of Design calculations and
// validation. It has no database or HTTP dependencies.
package model

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Engineering item types.
const (
	TypePart        = "part"
	TypeRequirement = "requirement"
	TypeProcedure   = "procedure"
	TypeDocument    = "document"
	TypePage        = "page"
)

// Part kinds.
const (
	KindProduction = "production"
	KindReference  = "reference"
	KindCOTS       = "cots"
)

// Engineering statuses.
const (
	StatusDraft      = "Draft"
	StatusInReview   = "In Review"
	StatusReleased   = "Released"
	StatusSuperseded = "Superseded"
	StatusObsolete   = "Obsolete"
)

// Link is a typed, directional link stored on the source record.
type Link struct {
	Type string `yaml:"type" json:"type"`
	To   string `yaml:"to" json:"to"`
	Rev  string `yaml:"rev,omitempty" json:"rev,omitempty"`
}

// BOMLine is one line of an assembly's Bill of Design.
type BOMLine struct {
	Find   int      `yaml:"find" json:"find"`
	Item   string   `yaml:"item" json:"item"`
	Qty    float64  `yaml:"qty" json:"qty"`
	RefDes []string `yaml:"refdes,omitempty" json:"refdes"`
	Notes  string   `yaml:"notes,omitempty" json:"notes"`
}

// File is an attachment on a revision, stored in the file store by checksum.
type File struct {
	SHA256 string `yaml:"sha256" json:"sha256"`
	Name   string `yaml:"name" json:"name"`
	Role   string `yaml:"role" json:"role"`
	Size   int64  `yaml:"size" json:"size"`
	Mime   string `yaml:"mime" json:"mime"`
}

// Step is one step of a procedure.
type Step struct {
	Instruction string `yaml:"instruction" json:"instruction"`
	Acceptance  string `yaml:"acceptance,omitempty" json:"acceptance"`
}

// Item is an engineering item or a page as stored in git. Fields that do not
// apply to a type stay empty and are omitted from the file.
type Item struct {
	ID        string   `yaml:"id" json:"id"`
	Type      string   `yaml:"type" json:"type"`
	Kind      string   `yaml:"kind,omitempty" json:"kind,omitempty"`
	Title     string   `yaml:"title" json:"title"`
	Revision  string   `yaml:"revision,omitempty" json:"revision"`
	Status    string   `yaml:"status" json:"status"`
	Owner     string   `yaml:"owner" json:"owner"`
	Assignee  string   `yaml:"assignee,omitempty" json:"assignee"`
	CreatedBy string   `yaml:"created_by" json:"createdBy"`
	Space     string   `yaml:"space" json:"space"`
	Labels    []string `yaml:"labels,omitempty" json:"labels"`

	// Part
	Unit           string `yaml:"unit,omitempty" json:"unit,omitempty"`
	Tracking       string `yaml:"tracking,omitempty" json:"tracking,omitempty"`
	Commodity      string `yaml:"commodity,omitempty" json:"commodity,omitempty"`
	VendorDownload bool   `yaml:"vendor_download,omitempty" json:"vendorDownload,omitempty"`
	Description    string `yaml:"description,omitempty" json:"description,omitempty"`

	// COTS part
	Manufacturer string   `yaml:"manufacturer,omitempty" json:"manufacturer,omitempty"`
	OrderNumber  string   `yaml:"order_number,omitempty" json:"orderNumber,omitempty"`
	TypeNumber   string   `yaml:"type_number,omitempty" json:"typeNumber,omitempty"`
	Lifecycle    string   `yaml:"lifecycle,omitempty" json:"lifecycle,omitempty"`
	Alternates   []string `yaml:"alternates,omitempty" json:"alternates,omitempty"`
	UnitCost     string   `yaml:"unit_cost,omitempty" json:"unitCost,omitempty"`
	LeadTimeDays int      `yaml:"lead_time_days,omitempty" json:"leadTimeDays,omitempty"`

	// Requirement
	Statement     string   `yaml:"statement,omitempty" json:"statement,omitempty"`
	Rationale     string   `yaml:"rationale,omitempty" json:"rationale,omitempty"`
	Category      string   `yaml:"category,omitempty" json:"category,omitempty"`
	Source        string   `yaml:"source,omitempty" json:"source,omitempty"`
	Verification  []string `yaml:"verification,omitempty" json:"verification,omitempty"`
	Priority      string   `yaml:"priority,omitempty" json:"priority,omitempty"`

	// Procedure
	ProcedureType string `yaml:"procedure_type,omitempty" json:"procedureType,omitempty"`
	Purpose       string `yaml:"purpose,omitempty" json:"purpose,omitempty"`
	Tools         []string `yaml:"tools,omitempty" json:"tools,omitempty"`
	Safety        string `yaml:"safety,omitempty" json:"safety,omitempty"`
	DurationMin   int    `yaml:"duration_min,omitempty" json:"durationMin,omitempty"`
	Steps         []Step `yaml:"steps,omitempty" json:"steps,omitempty"`

	// Document
	DocumentType string `yaml:"document_type,omitempty" json:"documentType,omitempty"`
	Summary      string `yaml:"summary,omitempty" json:"summary,omitempty"`
	ExternalRef  string `yaml:"external_ref,omitempty" json:"externalRef,omitempty"`

	// Page
	Parent       string `yaml:"parent,omitempty" json:"parent,omitempty"`
	Sort         int    `yaml:"sort,omitempty" json:"sort,omitempty"`
	PageType     string `yaml:"page_type,omitempty" json:"pageType,omitempty"`
	Controlled   string `yaml:"controlled,omitempty" json:"controlled,omitempty"` // default, yes, no
	ReleasedOnly bool   `yaml:"released_only,omitempty" json:"releasedOnly,omitempty"`

	Links []Link    `yaml:"links,omitempty" json:"links"`
	BOM   []BOMLine `yaml:"bom,omitempty" json:"bom"`
	Files []File    `yaml:"files,omitempty" json:"files"`

	// Body is the markdown after the frontmatter.
	Body string `yaml:"-" json:"body"`
}

var sep = []byte("---\n")

// Parse reads an item file: YAML frontmatter between --- lines, then markdown.
func Parse(b []byte) (*Item, error) {
	b = bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(b, sep) {
		return nil, errors.New("file has no frontmatter")
	}
	rest := b[len(sep):]
	end := bytes.Index(rest, []byte("\n---\n"))
	var fm, body []byte
	switch {
	case end >= 0:
		fm, body = rest[:end+1], rest[end+5:]
	case bytes.HasSuffix(rest, []byte("\n---")):
		fm = rest[:len(rest)-3]
	default:
		return nil, errors.New("frontmatter is not closed")
	}
	it := &Item{}
	dec := yaml.NewDecoder(bytes.NewReader(fm))
	dec.KnownFields(true)
	if err := dec.Decode(it); err != nil {
		return nil, fmt.Errorf("frontmatter: %w", err)
	}
	it.Body = strings.TrimPrefix(string(body), "\n")
	if it.ID == "" {
		return nil, errors.New("frontmatter has no id")
	}
	if it.Type == "" {
		return nil, errors.New("frontmatter has no type")
	}
	return it, nil
}

// Marshal writes the item in its file format. Output is stable, so saving
// an unchanged item produces an identical file.
func (it *Item) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(sep)
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(it); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	buf.Write(sep)
	body := strings.TrimRight(strings.ReplaceAll(it.Body, "\r\n", "\n"), "\n")
	if body != "" {
		buf.WriteString("\n")
		buf.WriteString(body)
		buf.WriteString("\n")
	}
	return buf.Bytes(), nil
}

// Clone returns a deep copy.
func (it *Item) Clone() *Item {
	b, err := it.Marshal()
	if err != nil {
		c := *it
		return &c
	}
	c, err := Parse(b)
	if err != nil {
		c2 := *it
		return &c2
	}
	return c
}

// IsEngineering reports whether the item belongs to the library.
func (it *Item) IsEngineering() bool { return it.Type != TypePage }

// HasBOM reports whether the item is an assembly.
func (it *Item) HasBOM() bool { return it.Type == TypePart && HasBOM(it.ID) }

// SearchText returns the text indexed for full-text search.
func (it *Item) SearchText() string {
	parts := []string{it.Description, it.Statement, it.Rationale, it.Purpose, it.Safety, it.Summary,
		it.Manufacturer, it.OrderNumber, it.TypeNumber, it.Source, it.ExternalRef, it.Body}
	for _, s := range it.Steps {
		parts = append(parts, s.Instruction, s.Acceptance)
	}
	return strings.Join(nonEmpty(parts), "\n")
}

func nonEmpty(ss []string) []string {
	var out []string
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

// Path returns the git path of an item's current file, and of a draft of
// its next revision.
func Path(it *Item) string { return PathFor(it.Type, it.ID, it.Space) }

// PathFor returns the git path of a record's current file.
func PathFor(typ, id, space string) string {
	if typ == TypePage {
		return "pages/" + space + "/" + id + ".md"
	}
	return "items/" + id + ".md"
}

// NextPath returns the path of the draft of an item's next revision.
func NextPath(current string) string {
	return strings.TrimSuffix(current, ".md") + ".next.md"
}

// NextRevision returns the revision after rev ("" gives "01").
func NextRevision(rev string) string {
	n := 0
	fmt.Sscanf(rev, "%d", &n)
	return fmt.Sprintf("%02d", n+1)
}
