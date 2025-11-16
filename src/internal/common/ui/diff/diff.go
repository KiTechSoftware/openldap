package diff

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/fatih/color"
	"github.com/kitechsoftware/ldappy/internal/common/config"

	"github.com/pelletier/go-toml/v2"
)

// Patch represents a structured change between two configs.
type Patch struct {
	Path     string `json:"path"`                // e.g. "ldap.domain" or "tls.cert_file"
	OldValue string `json:"old_value,omitempty"` // previous value
	NewValue string `json:"new_value,omitempty"` // new value
}

// DiffMode determines output format.
type DiffMode int

const (
	HumanReadable DiffMode = iota
	JSONPatch
)

// String implements fmt.Stringer for readability / logging.
func (d DiffMode) String() string {
	switch d {
	case HumanReadable:
		return "human"
	case JSONPatch:
		return "json"
	default:
		return "unknown"
	}
}

// Diff returns a diff between current and desired configs.
// If mode == HumanReadable, returns a colorized diff string.
// If mode == JSONPatch, returns a JSON patch array string.
func Diff(current, desired *config.Config, mode DiffMode) string {
	var aBuf, bBuf bytes.Buffer

	// Encode deterministically (struct order is consistent in Go)
	_ = toml.NewEncoder(&aBuf).Encode(current)
	_ = toml.NewEncoder(&bBuf).Encode(desired)

	// Fast path: byte-for-byte identical → no diff.
	if bytes.Equal(aBuf.Bytes(), bBuf.Bytes()) {
		if mode == JSONPatch {
			return "[]"
		}
		return ""
	}

	// Split & normalize lines.
	a := normalize(strings.Split(aBuf.String(), "\n"))
	b := normalize(strings.Split(bBuf.String(), "\n"))

	switch mode {
	case JSONPatch:
		patches := makeJSONPatches(a, b)
		data, _ := json.MarshalIndent(patches, "", "  ")
		return string(data)
	default:
		return makePrettyDiff(a, b)
	}
}

// ---------- Human-readable Diff ----------

func makePrettyDiff(a, b []string) string {
	var out []string
	out = append(out, color.HiCyanString("--- state"), color.HiCyanString("+++ config"))

	diff := lcsDiff(a, b)
	for _, line := range diff {
		switch {
		case strings.HasPrefix(line, "-"):
			out = append(out, color.RedString(line))
		case strings.HasPrefix(line, "+"):
			out = append(out, color.GreenString(line))
		default:
			out = append(out, color.HiBlackString("  "+line))
		}
	}
	return strings.Join(out, "\n")
}

// ---------- JSON Patch Builder ----------

func makeJSONPatches(a, b []string) []Patch {
	var patches []Patch
	diff := lcsDiff(a, b)

	var currentSection string
	for _, line := range diff {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "[") && strings.HasSuffix(trim, "]") {
			currentSection = strings.Trim(trim, "[]")
			continue
		}

		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "+ ") {
			kv := strings.TrimPrefix(trim, "-")
			kv = strings.TrimPrefix(kv, "+")
			kv = strings.TrimSpace(kv)
			parts := strings.SplitN(kv, "=", 2)
			if len(parts) != 2 {
				continue
			}

			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), `"`)
			path := key
			if currentSection != "" {
				path = currentSection + "." + key
			}

			// Try to merge with existing patch entry
			found := false
			for i := range patches {
				if patches[i].Path == path {
					if strings.HasPrefix(line, "-") {
						patches[i].OldValue = val
					} else {
						patches[i].NewValue = val
					}
					found = true
					break
				}
			}
			if !found {
				if strings.HasPrefix(line, "-") {
					patches = append(patches, Patch{Path: path, OldValue: val})
				} else {
					patches = append(patches, Patch{Path: path, NewValue: val})
				}
			}
		}
	}
	return patches
}

// ---------- Utility Helpers ----------

// normalize trims whitespace and removes empty trailing lines.
func normalize(lines []string) []string {
	var out []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	return out
}

// lcsDiff implements a simple longest common subsequence diff algorithm.
func lcsDiff(a, b []string) []string {
	type cell struct {
		len int
		dir rune // '↖', '↑', or '←'
	}
	m, n := len(a), len(b)
	table := make([][]cell, m+1)
	for i := range table {
		table[i] = make([]cell, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				table[i][j] = cell{table[i-1][j-1].len + 1, '↖'}
			} else if table[i-1][j].len >= table[i][j-1].len {
				table[i][j] = cell{table[i-1][j].len, '↑'}
			} else {
				table[i][j] = cell{table[i][j-1].len, '←'}
			}
		}
	}

	var res []string
	var backtrack func(i, j int)
	backtrack = func(i, j int) {
		if i == 0 && j == 0 {
			return
		}
		switch table[i][j].dir {
		case '↖':
			backtrack(i-1, j-1)
			res = append(res, a[i-1])
		case '↑':
			backtrack(i-1, j)
			res = append(res, "- "+a[i-1])
		case '←':
			backtrack(i, j-1)
			res = append(res, "+ "+b[j-1])
		}
	}
	backtrack(m, n)
	return res
}
