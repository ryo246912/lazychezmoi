package chezmoi

import (
	"os"
	"path/filepath"
	"testing"

	"lazychezmoi/internal/model"
)

func TestParseStatus(t *testing.T) {
	input := []byte("MM /home/user/.bashrc\nA  /home/user/.vimrc\n D /home/user/.zshrc\n")
	entries := ParseStatus(input)

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].SourceCode != model.StatusModified || entries[0].TargetCode != model.StatusModified {
		t.Errorf("entry 0: expected MM, got %c%c", entries[0].SourceCode, entries[0].TargetCode)
	}
	if entries[0].Kind != model.EntryManaged {
		t.Errorf("entry 0 kind = %v, want managed", entries[0].Kind)
	}
	if entries[0].TargetPath != "/home/user/.bashrc" {
		t.Errorf("entry 0 path: got %q", entries[0].TargetPath)
	}

	if entries[1].SourceCode != model.StatusAdded {
		t.Errorf("entry 1: expected A, got %c", entries[1].SourceCode)
	}
	if entries[1].TargetPath != "/home/user/.vimrc" {
		t.Errorf("entry 1 path: got %q", entries[1].TargetPath)
	}
}

func TestParseStatusEmpty(t *testing.T) {
	entries := ParseStatus([]byte(""))
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestParseStatusFile(t *testing.T) {
	// Load testdata fixture
	input := []byte("MM /Users/user/.claude/settings.json\nA  /Users/user/plan/03-result.md\n")
	entries := ParseStatus(input)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].TargetPath != "/Users/user/.claude/settings.json" {
		t.Errorf("path mismatch: %q", entries[0].TargetPath)
	}
}

func TestParseUnmanaged(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, ".zshrc")
	dirPath := filepath.Join(root, ".config")
	linkPath := filepath.Join(root, ".gitconfig")

	if err := os.WriteFile(filePath, []byte("export FOO=bar\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.Mkdir(dirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(filePath, linkPath); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	input := []byte(filePath + "\x00" + dirPath + "\x00" + linkPath + "\x00")
	entries := ParseUnmanaged(input)

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].Kind != model.EntryUnmanaged || entries[0].TargetType != model.TargetFile {
		t.Fatalf("file entry = %#v", entries[0])
	}
	if entries[1].TargetType != model.TargetDirectory {
		t.Fatalf("dir entry type = %v, want directory", entries[1].TargetType)
	}
	if entries[2].TargetType != model.TargetSymlink {
		t.Fatalf("symlink entry type = %v, want symlink", entries[2].TargetType)
	}
}
