package chezmoi

import (
	"bytes"
	"os"
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
			Kind:       model.EntryManaged,
			SourceCode: sourceCode,
			TargetCode: targetCode,
			TargetPath: path,
			TargetType: model.TargetFile,
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

func detectTargetType(path string) model.TargetKind {
	info, err := os.Lstat(path)
	if err != nil {
		return model.TargetUnknown
	}

	switch mode := info.Mode(); {
	case mode&os.ModeSymlink != 0:
		return model.TargetSymlink
	case mode.IsDir():
		return model.TargetDirectory
	default:
		return model.TargetFile
	}
}
