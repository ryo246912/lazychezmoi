package chezmoi

import (
	"bytes"
	"strings"

	"github.com/ryo246912/lazychezmoi/tools/lazychezmoi/internal/model"
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

		targetType := model.TargetFile
		if targetCode == model.StatusScript {
			targetType = model.TargetScript
		}

		entries = append(entries, model.Entry{
			Kind:       model.EntryManaged,
			SourceCode: sourceCode,
			TargetCode: targetCode,
			TargetPath: path,
			TargetType: targetType,
		})
	}
	return entries
}

// ParseUnmanaged parses chezmoi unmanaged output.
func ParseUnmanaged(data []byte) []model.Entry {
	if len(data) == 0 {
		return nil
	}

	var rawPaths [][]byte
	if bytes.Contains(data, []byte{0}) {
		rawPaths = bytes.Split(data, []byte{0})
	} else {
		rawPaths = bytes.Split(data, []byte("\n"))
	}

	var entries []model.Entry
	for _, rawPath := range rawPaths {
		path := strings.TrimSpace(string(bytes.TrimRight(rawPath, "\r")))
		if path == "" {
			continue
		}
		entries = append(entries, model.Entry{
			Kind:       model.EntryUnmanaged,
			TargetPath: path,
			TargetType: detectTargetType(path),
		})
	}
	return entries
}

func detectTargetType(path string) model.TargetKind { return model.DetectTargetKind(path) }
