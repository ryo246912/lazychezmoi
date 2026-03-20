package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"lazychezmoi/internal/model"
)

func TestEditUsesFocusedPanePath(t *testing.T) {
	entry := model.Entry{
		SourceCode: model.StatusModified,
		TargetCode: model.StatusModified,
		SourcePath: "/src/dot_zshrc",
		TargetPath: "/dst/.zshrc",
	}

	testCases := []struct {
		name       string
		focus      paneKind
		sourcePath string
		wantPath   string
		wantStatus string
	}{
		{
			name:       "src pane opens source",
			focus:      paneSrc,
			sourcePath: "/src/dot_zshrc",
			wantPath:   "/src/dot_zshrc",
		},
		{
			name:       "target pane opens target",
			focus:      paneTarget,
			sourcePath: "/src/dot_zshrc",
			wantPath:   "/dst/.zshrc",
		},
		{
			name:       "src pane waits for source resolution",
			focus:      paneSrc,
			sourcePath: "",
			wantStatus: "Source path is still resolving",
		},
		{
			name:       "diff pane does not open an editor",
			focus:      paneDiff,
			wantStatus: "Move focus to src or target to edit a file",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel([]model.Entry{entry})
			m.focusedPane = tc.focus
			m.entries[0].SourcePath = tc.sourcePath

			var opened []string
			m.openEditor = func(path string) tea.Cmd {
				opened = append(opened, path)
				return nil
			}

			next, _ := m.Update(keyRunes("e"))
			got := next.(Model)

			if tc.wantPath == "" {
				if len(opened) != 0 {
					t.Fatalf("unexpected editor open: %v", opened)
				}
			} else {
				if len(opened) != 1 {
					t.Fatalf("expected one editor open, got %d", len(opened))
				}
				if opened[0] != tc.wantPath {
					t.Fatalf("opened path = %q, want %q", opened[0], tc.wantPath)
				}
			}

			if got.statusMsg != tc.wantStatus {
				t.Fatalf("status = %q, want %q", got.statusMsg, tc.wantStatus)
			}
		})
	}
}

func TestTargetSelectionAndBatchApplyConfirmation(t *testing.T) {
	m := newTestModel([]model.Entry{
		{SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetPath: "/dst/.zshrc"},
		{SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetPath: "/dst/.gitconfig"},
	})

	next, _ := m.Update(keySpace())
	m = next.(Model)
	if !m.isTargetSelected("/dst/.zshrc") {
		t.Fatalf("expected first target to be selected")
	}

	next, _ = m.Update(keyDown())
	m = next.(Model)
	next, _ = m.Update(keySpace())
	m = next.(Model)
	next, _ = m.Update(keyRunes("a"))
	m = next.(Model)

	if m.state != stateConfirming {
		t.Fatalf("state = %v, want confirming", m.state)
	}

	wantTargets := []string{"/dst/.zshrc", "/dst/.gitconfig"}
	if strings.Join(m.confirmTargets, ",") != strings.Join(wantTargets, ",") {
		t.Fatalf("confirm targets = %v, want %v", m.confirmTargets, wantTargets)
	}

	view := m.View()
	if !strings.Contains(view, "Apply 2 files?") {
		t.Fatalf("confirm modal missing batch summary: %q", view)
	}
}

func TestDiffFocusScrollsViewport(t *testing.T) {
	m := newTestModel([]model.Entry{
		{SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetPath: "/dst/.zshrc"},
		{SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetPath: "/dst/.gitconfig"},
	})
	m.focusedPane = paneDiff
	m.diffCache["/dst/.zshrc"] = diffState{
		content: strings.Join([]string{"1", "2", "3", "4", "5", "6"}, "\n"),
	}
	m.syncDiffPreview(true)

	next, _ := m.Update(keyRunes("j"))
	got := next.(Model)

	if got.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", got.cursor)
	}
	if got.diffViewport.YOffset == 0 {
		t.Fatalf("expected diff viewport to scroll")
	}

	got.focusedPane = paneTarget
	next, _ = got.Update(keyDown())
	got = next.(Model)
	if got.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", got.cursor)
	}
	if got.diffViewport.YOffset != 0 {
		t.Fatalf("diff viewport offset = %d, want 0", got.diffViewport.YOffset)
	}
}

func TestStaleDiffResultsAreIgnored(t *testing.T) {
	m := newTestModel([]model.Entry{
		{SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetPath: "/dst/.zshrc"},
	})
	m.entryGeneration = 1
	_ = m.requestDiffLoadCmd("/dst/.zshrc")
	request := m.diffCache["/dst/.zshrc"]

	next, _ := m.Update(diffLoadedMsg{
		targetPath: "/dst/.zshrc",
		generation: request.requestGeneration - 1,
		requestID:  request.requestID,
		content:    "stale",
	})
	m = next.(Model)
	if m.diffCache["/dst/.zshrc"].content != "" {
		t.Fatalf("stale diff unexpectedly updated cache")
	}

	next, _ = m.Update(diffLoadedMsg{
		targetPath: "/dst/.zshrc",
		generation: request.requestGeneration,
		requestID:  request.requestID,
		content:    "fresh",
	})
	m = next.(Model)
	if m.diffCache["/dst/.zshrc"].content != "fresh" {
		t.Fatalf("diff cache = %q, want fresh", m.diffCache["/dst/.zshrc"].content)
	}
}

func newTestModel(entries []model.Entry) Model {
	m := New(nil)
	m.entries = append([]model.Entry(nil), entries...)
	m.ready = true
	m.width = 120
	m.height = 24
	m.diffViewport = viewport.New(60, 4)
	return m
}

func keyRunes(value string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(value)}
}

func keySpace() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeySpace}
}

func keyDown() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyDown}
}
