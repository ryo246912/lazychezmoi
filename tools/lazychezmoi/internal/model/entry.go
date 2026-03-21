package model

import "path/filepath"

type StatusCode byte

const (
	StatusNone     StatusCode = ' '
	StatusAdded    StatusCode = 'A'
	StatusModified StatusCode = 'M'
	StatusDeleted  StatusCode = 'D'
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
)

func (k TargetKind) String() string {
	switch k {
	case TargetFile:
		return "file"
	case TargetDirectory:
		return "directory"
	case TargetSymlink:
		return "symlink"
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

func (e Entry) CanApply() bool {
	if e.Kind != EntryManaged {
		return false
	}
	return e.TargetCode != StatusNone && e.TargetCode != StatusDeleted
}

func (e Entry) CanAdd() bool {
	return e.Kind == EntryUnmanaged
}

func (e Entry) CanDeleteTarget() bool {
	return e.Kind == EntryUnmanaged
}

func (e Entry) CanEditSource() bool {
	return e.Kind == EntryManaged && e.SourcePath != ""
}

func (e Entry) CanEditTarget() bool {
	return e.TargetType != TargetDirectory
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
