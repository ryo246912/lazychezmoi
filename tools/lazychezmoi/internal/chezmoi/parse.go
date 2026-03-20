package chezmoi

import (
	"bytes"
	"strings"

	"lazychezmoi/internal/model"
)

// ParseStatus parses chezmoi status output
// Format: "XY /absolute/path/to/file\n..."
func ParseStatus(data []byte) []model.Entry {
	var entries []model.Entry
	for _, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		if len(line) < 3 {
			continue
		}
		sourceCode := model.StatusCode(line[0])
		targetCode := model.StatusCode(line[1])
		if line[2] != ' ' {
			continue
		}
		path := strings.TrimSpace(string(line[3:]))
		if path == "" {
			continue
		}
		entries = append(entries, model.Entry{
			SourceCode: sourceCode,
			TargetCode: targetCode,
			TargetPath: path,
		})
	}
	return entries
}
