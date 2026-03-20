package model

import "path/filepath"

type StatusCode byte

const (
	StatusNone     StatusCode = ' '
	StatusAdded    StatusCode = 'A'
	StatusModified StatusCode = 'M'
	StatusDeleted  StatusCode = 'D'
)

type Entry struct {
	SourceCode StatusCode
	TargetCode StatusCode
	TargetPath string
	SourcePath string // resolved lazily
}

func (e Entry) CanApply() bool {
	return e.TargetCode != StatusNone && e.TargetCode != StatusDeleted
}

func (e Entry) StatusLabel() string {
	switch e.TargetCode {
	case StatusAdded:
		return "added"
	case StatusModified:
		return "modified"
	case StatusDeleted:
		return "deleted"
	default:
		if e.SourceCode == StatusAdded {
			return "new"
		}
		return string([]byte{byte(e.SourceCode), byte(e.TargetCode)})
	}
}

func (e Entry) Name() string {
	return filepath.Base(e.TargetPath)
}
