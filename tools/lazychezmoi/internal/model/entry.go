package model

import (
	"os"
	"path/filepath"
	"strings"
)

type StatusCode byte

const (
	StatusNone     StatusCode = ' '
	StatusAdded    StatusCode = 'A'
	StatusModified StatusCode = 'M'
	StatusDeleted  StatusCode = 'D'
	StatusScript   StatusCode = 'R'
)

type EntryKind int

const (
	EntryManaged EntryKind = iota
	EntryUnmanaged
)

func (k EntryKind) String() string {
	switch k {
	case EntryUnmanaged:
		return "unmanaged"
	default:
		return "managed"
	}
}

type TargetKind int

const (
	TargetUnknown TargetKind = iota
	TargetFile
	TargetDirectory
	TargetSymlink
	TargetScript
)

func (k TargetKind) String() string {
	switch k {
	case TargetFile:
		return "file"
	case TargetDirectory:
		return "directory"
	case TargetSymlink:
		return "symlink"
	case TargetScript:
		return "script"
	default:
		return "unknown"
	}
}

type Entry struct {
	Kind       EntryKind
	SourceCode StatusCode
	TargetCode StatusCode
	TargetType TargetKind
	TargetPath string
	SourcePath string // resolved lazily
}

func (e Entry) HasTargetDiff() bool {
	return e.Kind == EntryManaged && e.TargetCode != StatusNone && e.TargetCode != StatusDeleted
}

func (e Entry) CanApply() bool {
	return e.HasTargetDiff() || (e.Kind == EntryManaged && e.TargetType == TargetScript)
}

// IsTemplate returns true when the source file is a chezmoi template (.tmpl).
// SourcePath must already be resolved for this to return true.
func (e Entry) IsTemplate() bool {
	return e.Kind == EntryManaged && strings.HasSuffix(e.SourcePath, ".tmpl")
}

func (e Entry) CanAdd() bool {
	if e.Kind == EntryUnmanaged {
		return true
	}
	return e.HasTargetDiff() || e.TargetType == TargetScript
}

func (e Entry) CanDeleteTarget() bool {
	return e.Kind == EntryUnmanaged
}

func (e Entry) CanEditSource() bool {
	return e.Kind == EntryManaged && e.SourcePath != ""
}

func (e Entry) CanEditTarget() bool {
	return e.TargetType != TargetDirectory && e.TargetType != TargetScript
}

func (e Entry) StatusLabel() string {
	if e.Kind == EntryUnmanaged {
		return "unmanaged"
	}
	switch e.TargetCode {
	case StatusAdded:
		return "added"
	case StatusModified:
		return "modified"
	case StatusDeleted:
		return "deleted"
	case StatusScript:
		return "runnable"
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

func DetectTargetKind(path string) TargetKind {
	info, err := os.Lstat(path)
	if err != nil {
		return TargetUnknown
	}

	switch mode := info.Mode(); {
	case mode&os.ModeSymlink != 0:
		return TargetSymlink
	case mode.IsDir():
		return TargetDirectory
	default:
		return TargetFile
	}
}
