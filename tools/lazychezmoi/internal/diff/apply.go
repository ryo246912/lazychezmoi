package diff

import (
	"regexp"
	"strconv"
	"strings"
)

type hunkLine struct {
	kind byte   // ' ', '-', '+'
	text string // line content without the prefix character
}

type patchHunk struct {
	oldStart int
	lines    []hunkLine
}

var hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,\d+)? \+\d+(?:,\d+)? @@`)

func parseHunks(patch string) []patchHunk {
	var hunks []patchHunk
	lines := strings.Split(patch, "\n")
	i := 0
	for i < len(lines) && !strings.HasPrefix(lines[i], "@@") {
		i++
	}
	for i < len(lines) {
		m := hunkHeaderRe.FindStringSubmatch(lines[i])
		if m == nil {
			i++
			continue
		}
		oldStart, _ := strconv.Atoi(m[1])
		h := patchHunk{oldStart: oldStart}
		i++
		for i < len(lines) && !strings.HasPrefix(lines[i], "@@") {
			l := lines[i]
			if len(l) > 0 {
				switch l[0] {
				case ' ', '-', '+':
					h.lines = append(h.lines, hunkLine{kind: l[0], text: l[1:]})
				}
			} else {
				// empty context line
				h.lines = append(h.lines, hunkLine{kind: ' ', text: ""})
			}
			i++
		}
		hunks = append(hunks, h)
	}
	return hunks
}

// ApplyWithConflicts applies a unified diff patch to src line by line.
// Context lines (' ') and matching change blocks are applied normally.
// Change blocks where the removed ('-') lines don't match the source
// (e.g. because the source contains template variables instead of the
// rendered values) are written as inline conflict markers:
//
//	<<<<<<< source (template)
//	<current source lines>
//	=======
//	<new target lines>
//	>>>>>>> target
//
// Returns the modified content and whether any conflicts were inserted.
func ApplyWithConflicts(src []byte, patch string) ([]byte, bool) {
	if patch == "" || patch == "(no differences)\n" {
		return src, false
	}
	hunks := parseHunks(patch)
	if len(hunks) == 0 {
		return src, false
	}

	srcStr := string(src)
	srcLines := strings.Split(srcStr, "\n")
	hasTrailingNewline := strings.HasSuffix(srcStr, "\n")
	if hasTrailingNewline && len(srcLines) > 0 && srcLines[len(srcLines)-1] == "" {
		srcLines = srcLines[:len(srcLines)-1]
	}

	var out []string
	srcIdx := 0
	hasConflicts := false

	for _, h := range hunks {
		// Copy lines before this hunk's expected start position.
		expectedPos := h.oldStart - 1 // convert to 0-based
		for srcIdx < expectedPos && srcIdx < len(srcLines) {
			out = append(out, srcLines[srcIdx])
			srcIdx++
		}

		// Process hunk line by line, grouping consecutive -/+ blocks.
		i := 0
		for i < len(h.lines) {
			l := h.lines[i]
			switch l.kind {
			case ' ': // context line — pass through from source
				if srcIdx < len(srcLines) {
					out = append(out, srcLines[srcIdx])
					srcIdx++
				}
				i++

			case '-', '+':
				// Collect the full change block (consecutive - and + lines).
				var removed, added []string
				for i < len(h.lines) && (h.lines[i].kind == '-' || h.lines[i].kind == '+') {
					if h.lines[i].kind == '-' {
						removed = append(removed, h.lines[i].text)
					} else {
						added = append(added, h.lines[i].text)
					}
					i++
				}

				// Try to match removed lines against source at current position.
				matches := srcIdx+len(removed) <= len(srcLines)
				for j := 0; matches && j < len(removed); j++ {
					if srcLines[srcIdx+j] != removed[j] {
						matches = false
					}
				}

				if matches {
					// Clean apply: skip removed lines, emit added lines.
					out = append(out, added...)
					srcIdx += len(removed)
				} else {
					// Conflict: source differs from rendered (e.g. template variable).
					// Emit conflict markers so the user can resolve manually.
					hasConflicts = true
					end := min(srcIdx+len(removed), len(srcLines))
					out = append(out, "<<<<<<< source (template)")
					out = append(out, srcLines[srcIdx:end]...)
					out = append(out, "=======")
					out = append(out, added...)
					out = append(out, ">>>>>>> target")
					srcIdx = end
				}
			}
		}
	}

	// Copy any remaining source lines after the last hunk.
	out = append(out, srcLines[srcIdx:]...)

	output := strings.Join(out, "\n")
	if hasTrailingNewline {
		output += "\n"
	}
	return []byte(output), hasConflicts
}
