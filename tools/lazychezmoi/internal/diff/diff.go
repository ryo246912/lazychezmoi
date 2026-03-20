package diff

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/pmezard/go-difflib/difflib"
)

const maxBytes = 512 * 1024 // 512 KB

// Compute returns a unified diff between src and dst content.
// fromName and toName are labels (e.g. source path and target path).
func Compute(fromName string, from []byte, toName string, to []byte) string {
	// Binary detection
	if isBinary(from) || isBinary(to) {
		return fmt.Sprintf("Binary files differ\n  source: %s\n  target: %s\n", fromName, toName)
	}
	// Size limit
	if len(from) > maxBytes || len(to) > maxBytes {
		return fmt.Sprintf("File too large to diff (limit %d KB)\n", maxBytes/1024)
	}

	fromLines := toLines(string(from))
	toLines := toLines(string(to))

	udiff := difflib.UnifiedDiff{
		A:        fromLines,
		B:        toLines,
		FromFile: fromName,
		ToFile:   toName,
		Context:  3,
	}
	result, err := difflib.GetUnifiedDiffString(udiff)
	if err != nil {
		return fmt.Sprintf("diff error: %v\n", err)
	}
	if result == "" {
		return "(no differences)\n"
	}
	return result
}

func toLines(s string) []string {
	lines := strings.SplitAfter(s, "\n")
	return lines
}

func isBinary(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	sample := data
	if len(sample) > 8000 {
		sample = sample[:8000]
	}
	for len(sample) > 0 {
		r, size := utf8.DecodeRune(sample)
		if r == utf8.RuneError && size == 1 {
			return true
		}
		if r == 0 { // null byte
			return true
		}
		sample = sample[size:]
	}
	return false
}
