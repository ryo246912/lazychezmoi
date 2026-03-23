package ui

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizeCommandHistoryRemovesEmptyAndDuplicates(t *testing.T) {
	history := normalizeCommandHistory([]string{
		" echo ok ",
		"",
		"pwd",
		"echo ok",
		"   ",
		"ls",
	})

	want := []string{"echo ok", "pwd", "ls"}
	if !reflect.DeepEqual(history, want) {
		t.Fatalf("history = %v, want %v", history, want)
	}
}

func TestPrependCommandHistoryKeepsNewestFirst(t *testing.T) {
	history := prependCommandHistory("pwd", []string{"echo ok", "pwd", "ls"})
	want := []string{"pwd", "echo ok", "ls"}
	if !reflect.DeepEqual(history, want) {
		t.Fatalf("history = %v, want %v", history, want)
	}
}

func TestFileCommandHistoryStoreSaveAndLoad(t *testing.T) {
	store := fileCommandHistoryStore{
		path: filepath.Join(t.TempDir(), "lazychezmoi", "command-history.json"),
	}

	if err := store.Save([]string{"pwd", "echo ok", "pwd"}); err != nil {
		t.Fatalf("save history: %v", err)
	}

	history, err := store.Load()
	if err != nil {
		t.Fatalf("load history: %v", err)
	}

	want := []string{"pwd", "echo ok"}
	if !reflect.DeepEqual(history, want) {
		t.Fatalf("history = %v, want %v", history, want)
	}
}

func TestFileCommandHistoryStoreLoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lazychezmoi", "command-history.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatalf("write invalid history: %v", err)
	}

	store := fileCommandHistoryStore{path: path}
	if _, err := store.Load(); err == nil {
		t.Fatalf("expected invalid json to fail")
	}
}
