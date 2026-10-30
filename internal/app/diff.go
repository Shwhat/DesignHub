package app

import "strings"

// DiffLine is one line of a line-by-line comparison.
type DiffLine struct {
	Op   string `json:"op"` // "=", "+", "-"
	Text string `json:"text"`
}

// lineDiff compares two texts line by line using the longest common
// subsequence. Very large inputs fall back to a plain replace.
func lineDiff(a, b string) []DiffLine {
	x, y := splitLines(a), splitLines(b)
	// trim common prefix and suffix
	pre := 0
	for pre < len(x) && pre < len(y) && x[pre] == y[pre] {
		pre++
	}
	suf := 0
	for suf < len(x)-pre && suf < len(y)-pre && x[len(x)-1-suf] == y[len(y)-1-suf] {
		suf++
	}
	var out []DiffLine
	for _, l := range x[:pre] {
		out = append(out, DiffLine{"=", l})
	}
	mx, my := x[pre:len(x)-suf], y[pre:len(y)-suf]
	if len(mx)*len(my) > 4_000_000 {
		for _, l := range mx {
			out = append(out, DiffLine{"-", l})
		}
		for _, l := range my {
			out = append(out, DiffLine{"+", l})
		}
	} else {
		n, m := len(mx), len(my)
		lcs := make([][]int32, n+1)
		for i := range lcs {
			lcs[i] = make([]int32, m+1)
		}
		for i := n - 1; i >= 0; i-- {
			for j := m - 1; j >= 0; j-- {
				if mx[i] == my[j] {
					lcs[i][j] = lcs[i+1][j+1] + 1
				} else if lcs[i+1][j] >= lcs[i][j+1] {
					lcs[i][j] = lcs[i+1][j]
				} else {
					lcs[i][j] = lcs[i][j+1]
				}
			}
		}
		i, j := 0, 0
		for i < n && j < m {
			switch {
			case mx[i] == my[j]:
				out = append(out, DiffLine{"=", mx[i]})
				i++
				j++
			case lcs[i+1][j] >= lcs[i][j+1]:
				out = append(out, DiffLine{"-", mx[i]})
				i++
			default:
				out = append(out, DiffLine{"+", my[j]})
				j++
			}
		}
		for ; i < n; i++ {
			out = append(out, DiffLine{"-", mx[i]})
		}
		for ; j < m; j++ {
			out = append(out, DiffLine{"+", my[j]})
		}
	}
	for _, l := range x[len(x)-suf:] {
		out = append(out, DiffLine{"=", l})
	}
	return out
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}
