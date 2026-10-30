package model

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	productionRe = regexp.MustCompile(`^([0-9]{6})-([0-9]{3})$`)
	referenceRe  = regexp.MustCompile(`^[0-9]{7}$`)
	cotsRe       = regexp.MustCompile(`^([A-Z]{2,5})\.([A-Za-z0-9][A-Za-z0-9/_+-]*(\.[A-Za-z0-9/_+-]+)*)$`)
	prefixRe     = regexp.MustCompile(`^([A-Z]{2,3})-([0-9]{4,})$`)
)

// Record prefixes.
var Prefixes = map[string]string{
	"REQ": TypeRequirement, "PRC": TypeProcedure, "DOC": TypeDocument, "PG": TypePage,
	"CO": "change_order", "SP": "space", "TSK": "task", "ISS": "issue", "NCR": "ncr", "ACT": "action", "EPC": "epic",
}

// SuffixCategory describes a production suffix range.
type SuffixCategory struct {
	Name    string
	HasBOM  bool
	Allowed bool
	InBOM   bool // can be added as a BOM line
	Key     string
}

// Category returns the category of a 3-digit production suffix.
func Category(suffix string) SuffixCategory {
	if len(suffix) != 3 {
		return SuffixCategory{}
	}
	if suffix == "000" {
		return SuffixCategory{"Layout", false, true, true, "000"}
	}
	switch suffix[0] {
	case '0':
		return SuffixCategory{"Piece part", false, true, true, "0XX"}
	case '1':
		return SuffixCategory{"Piece part", false, true, true, "1XX"}
	case '2':
		return SuffixCategory{"Assembly", true, true, true, "2XX"}
	case '3':
		return SuffixCategory{"Harness", true, true, true, "3XX"}
	case '4':
		return SuffixCategory{"PCB", false, true, true, "4XX"}
	case '5':
		return SuffixCategory{"PCBA", true, true, true, "5XX"}
	case '6':
		return SuffixCategory{"Internal electronic component", false, true, true, "6XX"}
	case '9':
		return SuffixCategory{"Interface or make-on-assembly", false, true, false, "9XX"}
	}
	return SuffixCategory{Name: "Reserved", Key: string(suffix[0]) + "XX"}
}

// PartKindOf returns the part kind implied by an item number, or "".
func PartKindOf(id string) string {
	switch {
	case productionRe.MatchString(id):
		return KindProduction
	case referenceRe.MatchString(id):
		return KindReference
	case cotsRe.MatchString(id):
		return KindCOTS
	}
	return ""
}

// HasBOM reports whether a part number is an assembly (-2XX, -3XX, -5XX).
func HasBOM(id string) bool {
	m := productionRe.FindStringSubmatch(id)
	return m != nil && Category(m[2]).HasBOM
}

// CanBeBOMLine reports whether a part may be added to a BOM.
func CanBeBOMLine(id string) bool {
	m := productionRe.FindStringSubmatch(id)
	if m == nil {
		return PartKindOf(id) != ""
	}
	return Category(m[2]).InBOM
}

// FileRoleKey returns the key used by file role "expected on" lists.
func FileRoleKey(id string) string {
	if m := productionRe.FindStringSubmatch(id); m != nil {
		return Category(m[2]).Key
	}
	if cotsRe.MatchString(id) {
		return "COTS"
	}
	return ""
}

// ParseProduction validates a production number and returns its base.
func ParseProduction(id string) (base, suffix string, err error) {
	m := productionRe.FindStringSubmatch(id)
	if m == nil {
		return "", "", fmt.Errorf("%q is not a production number (6 digits, dash, 3 digits)", id)
	}
	if !Category(m[2]).Allowed {
		return "", "", fmt.Errorf("suffix -%s is reserved", m[2])
	}
	return m[1], m[2], nil
}

// ParseCOTS splits a COTS number into manufacturer code and order number.
func ParseCOTS(id string) (mfr, order string, err error) {
	m := cotsRe.FindStringSubmatch(id)
	if m == nil {
		return "", "", fmt.Errorf("%q is not a purchased part number (CODE.ORDERNUMBER)", id)
	}
	return m[1], m[2], nil
}

// COTSNumber builds a COTS part number.
func COTSNumber(mfr, order string) (string, error) {
	order = strings.TrimSpace(order)
	id := strings.ToUpper(strings.TrimSpace(mfr)) + "." + order
	if !cotsRe.MatchString(id) {
		return "", fmt.Errorf("order number may contain letters, digits, period, dash, slash, plus and underscore")
	}
	return id, nil
}

// PrefixOf returns the prefix of a PREFIX-NUMBER identifier.
func PrefixOf(id string) string {
	if m := prefixRe.FindStringSubmatch(id); m != nil {
		return m[1]
	}
	return ""
}

// FormatPrefixed formats a PREFIX-NUMBER identifier.
func FormatPrefixed(prefix string, n int64) string {
	return fmt.Sprintf("%s-%04d", prefix, n)
}

// TypeOf returns the record type implied by an identifier.
func TypeOf(id string) string {
	if PartKindOf(id) != "" {
		return TypePart
	}
	return Prefixes[PrefixOf(id)]
}

// ValidID reports whether id has any known identifier format.
func ValidID(id string) bool { return TypeOf(id) != "" }
