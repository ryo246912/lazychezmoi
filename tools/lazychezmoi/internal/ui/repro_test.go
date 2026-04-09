package ui

import (
	"testing"
	"github.com/ryo246912/lazychezmoi/tools/lazychezmoi/internal/model"
)

func TestCursorPositionAfterApply(t *testing.T) {
	m := newTestModel([]model.Entry{
		{Kind: model.EntryManaged, TargetPath: "/dst/.1"},
		{Kind: model.EntryManaged, TargetPath: "/dst/.2"},
		{Kind: model.EntryManaged, TargetPath: "/dst/.3"},
	})

	// Select the second item
	m.cursor = 1
	if m.selectedEntry().TargetPath != "/dst/.2" {
		t.Fatalf("expected /dst/.2, got %s", m.selectedEntry().TargetPath)
	}

	// Simulate apply of /dst/.2
	// In the real app, actionDoneMsg -> applySuccessfulAction -> removeTargets
	m.removeTargets("/dst/.2")

	if m.cursor != 1 {
		t.Errorf("cursor after removeTargets = %d, want 1", m.cursor)
	}
	if m.selectedEntry().TargetPath != "/dst/.3" {
		t.Errorf("selected entry after removeTargets = %s, want /dst/.3", m.selectedEntry().TargetPath)
	}

	// Simulate reload
	m.entriesLoading = true
	m.Update(entriesLoadedMsg{
		mode: m.listMode,
		entries: []model.Entry{
			{Kind: model.EntryManaged, TargetPath: "/dst/.1"},
			{Kind: model.EntryManaged, TargetPath: "/dst/.3"},
		},
	})

	// Check cursor after reload (via entriesLoadedMsg handle)
	// We need to re-run the logic in Update for entriesLoadedMsg
	m2, _ := m.Update(entriesLoadedMsg{
		mode: m.listMode,
		entries: []model.Entry{
			{Kind: model.EntryManaged, TargetPath: "/dst/.1"},
			{Kind: model.EntryManaged, TargetPath: "/dst/.3"},
		},
	})
	m = m2.(Model)

	if m.cursor != 1 {
		t.Errorf("cursor after entriesLoadedMsg = %d, want 1", m.cursor)
	}
	if m.selectedEntry().TargetPath != "/dst/.3" {
		t.Errorf("selected entry after entriesLoadedMsg = %s, want /dst/.3", m.selectedEntry().TargetPath)
	}
}
