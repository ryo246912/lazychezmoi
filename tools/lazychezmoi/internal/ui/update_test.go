package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	xansi "github.com/charmbracelet/x/ansi"

	gitmode "lazychezmoi/internal/git"
	"lazychezmoi/internal/model"
)

func TestEditUsesFocusedPanePath(t *testing.T) {
	entry := model.Entry{
		Kind:       model.EntryManaged,
		SourceCode: model.StatusModified,
		TargetCode: model.StatusModified,
		TargetType: model.TargetFile,
		SourcePath: "/src/dot_zshrc",
		TargetPath: "/dst/.zshrc",
	}

	testCases := []struct {
		name       string
		focus      paneKind
		entry      model.Entry
		wantPath   string
		wantStatus string
	}{
		{
			name:     "src pane opens source",
			focus:    paneSrc,
			entry:    entry,
			wantPath: "/src/dot_zshrc",
		},
		{
			name:     "target pane opens target",
			focus:    paneTarget,
			entry:    entry,
			wantPath: "/dst/.zshrc",
		},
		{
			name:  "src pane waits for source resolution",
			focus: paneSrc,
			entry: model.Entry{
				Kind:       model.EntryManaged,
				SourceCode: model.StatusModified,
				TargetCode: model.StatusModified,
				TargetType: model.TargetFile,
				TargetPath: "/dst/.zshrc",
			},
			wantStatus: "Source path is still resolving",
		},
		{
			name:  "unmanaged src pane has no source file",
			focus: paneSrc,
			entry: model.Entry{
				Kind:       model.EntryUnmanaged,
				TargetType: model.TargetFile,
				TargetPath: "/dst/.zshrc",
			},
			wantStatus: "Unmanaged entries do not have a source file yet",
		},
		{
			name:  "directory target cannot be edited",
			focus: paneTarget,
			entry: model.Entry{
				Kind:       model.EntryUnmanaged,
				TargetType: model.TargetDirectory,
				TargetPath: "/dst/.config",
			},
			wantStatus: "Directories cannot be opened in $EDITOR",
		},
		{
			name:       "diff pane does not open an editor",
			focus:      paneDiff,
			entry:      entry,
			wantStatus: "Move focus to src or target to edit a file",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel([]model.Entry{tc.entry})
			m.focusedPane = tc.focus

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
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.gitconfig"},
	})

	firstTarget := m.selectedEntry().TargetPath
	next, _ := m.Update(keySpace())
	m = next.(Model)
	if !m.isTargetSelected(firstTarget) {
		t.Fatalf("expected first visible target to be selected")
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

	if len(m.confirmAction.targets) != 2 {
		t.Fatalf("confirm targets = %v, want 2 targets", m.confirmAction.targets)
	}

	view := m.View()
	if !strings.Contains(view, "Apply 2 files?") {
		t.Fatalf("confirm modal missing batch summary: %q", view)
	}
}

func TestUnmanagedActionsEnterConfirmation(t *testing.T) {
	entry := model.Entry{
		Kind:       model.EntryUnmanaged,
		TargetType: model.TargetFile,
		TargetPath: "/dst/.zshrc",
	}

	m := newTestModel([]model.Entry{entry})
	m.listMode = listModeAll

	next, _ := m.Update(keyRunes("i"))
	got := next.(Model)
	if got.state != stateConfirming || got.confirmAction.kind != actionAdd {
		t.Fatalf("unexpected add confirmation state: %#v", got.confirmAction)
	}

	next, _ = got.Update(keyRunes("d"))
	got = next.(Model)
	if got.state != stateConfirming || got.confirmAction.kind != actionAdd {
		t.Fatalf("delete should not replace existing confirmation without reset")
	}

	got.state = stateNormal
	got.confirmAction = pendingAction{}
	next, _ = got.Update(keyRunes("d"))
	got = next.(Model)
	if got.state != stateConfirming || got.confirmAction.kind != actionDelete {
		t.Fatalf("unexpected delete confirmation state: %#v", got.confirmAction)
	}
}

func TestManagedAddEntersConfirmation(t *testing.T) {
	m := newTestModel([]model.Entry{
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
	})

	next, _ := m.Update(keyRunes("i"))
	got := next.(Model)
	if got.state != stateConfirming || got.confirmAction.kind != actionAdd {
		t.Fatalf("unexpected add confirmation state: %#v", got.confirmAction)
	}

	view := got.View()
	if !strings.Contains(view, "Copy Current Target Into Source?") {
		t.Fatalf("managed add confirmation missing copy wording: %q", view)
	}
}

func TestFooterExplainsModeAndAddHint(t *testing.T) {
	testCases := []struct {
		name      string
		listMode  listMode
		entry     model.Entry
		wantHints []string
	}{
		{
			name:     "managed footer explains tracked mode",
			listMode: listModeManaged,
			entry: model.Entry{
				Kind:       model.EntryManaged,
				SourceCode: model.StatusModified,
				TargetCode: model.StatusModified,
				TargetType: model.TargetFile,
				TargetPath: "/dst/.zshrc",
			},
			wantHints: []string{"i:add->src", "tracked entries with target-side diff"},
		},
		{
			name:     "unmanaged footer explains target only mode",
			listMode: listModeAll,
			entry: model.Entry{
				Kind:       model.EntryUnmanaged,
				TargetType: model.TargetFile,
				TargetPath: "/dst/.gitconfig",
			},
			wantHints: []string{"i:add->src/track", "managed diffs + unmanaged paths"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel([]model.Entry{tc.entry})
			m.listMode = tc.listMode

			footer := m.renderFooter()
			for _, hint := range tc.wantHints {
				if !strings.Contains(footer, hint) {
					t.Fatalf("footer missing %q: %q", hint, footer)
				}
			}
		})
	}
}

func TestSuccessfulActionsUpdateEntriesImmediately(t *testing.T) {
	testCases := []struct {
		name        string
		listMode    listMode
		action      pendingAction
		entries     []model.Entry
		wantTargets []string
		wantStatus  string
	}{
		{
			name:     "apply removes applied target immediately",
			listMode: listModeManaged,
			action: pendingAction{
				kind:    actionApply,
				targets: []string{"/dst/.zshrc"},
			},
			entries: []model.Entry{
				{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
				{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.gitconfig"},
			},
			wantTargets: []string{"/dst/.gitconfig"},
			wantStatus:  "Applied 1 file(s) from working tree",
		},
		{
			name:     "managed add removes target immediately",
			listMode: listModeManaged,
			action: pendingAction{
				kind:  actionAdd,
				entry: model.Entry{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
			},
			entries: []model.Entry{
				{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
				{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.gitconfig"},
			},
			wantTargets: []string{"/dst/.gitconfig"},
			wantStatus:  "Updated source state from /dst/.zshrc",
		},
		{
			name:     "unmanaged add removes target immediately",
			listMode: listModeAll,
			action: pendingAction{
				kind:  actionAdd,
				entry: model.Entry{Kind: model.EntryUnmanaged, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
			},
			entries: []model.Entry{
				{Kind: model.EntryUnmanaged, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
				{Kind: model.EntryUnmanaged, TargetType: model.TargetFile, TargetPath: "/dst/.gitconfig"},
			},
			wantTargets: []string{"/dst/.gitconfig"},
			wantStatus:  "Added /dst/.zshrc to source state",
		},
		{
			name:     "delete removes target immediately",
			listMode: listModeAll,
			action: pendingAction{
				kind:  actionDelete,
				entry: model.Entry{Kind: model.EntryUnmanaged, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
			},
			entries: []model.Entry{
				{Kind: model.EntryUnmanaged, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
				{Kind: model.EntryUnmanaged, TargetType: model.TargetFile, TargetPath: "/dst/.gitconfig"},
			},
			wantTargets: []string{"/dst/.gitconfig"},
			wantStatus:  "Deleted /dst/.zshrc",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(tc.entries)
			m.listMode = tc.listMode

			next, _ := m.Update(actionDoneMsg{action: tc.action})
			got := next.(Model)

			var targets []string
			for _, entry := range got.entries {
				targets = append(targets, entry.TargetPath)
			}
			if strings.Join(targets, ",") != strings.Join(tc.wantTargets, ",") {
				t.Fatalf("targets = %v, want %v", targets, tc.wantTargets)
			}
			if got.statusMsg != tc.wantStatus {
				t.Fatalf("status = %q, want %q", got.statusMsg, tc.wantStatus)
			}
		})
	}
}

func TestPartialApplyFailureRemovesCompletedTargetsImmediately(t *testing.T) {
	m := newTestModel([]model.Entry{
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.gitconfig"},
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.tmux.conf"},
	})

	next, _ := m.Update(actionErrMsg{
		action: pendingAction{
			kind:    actionApply,
			targets: []string{"/dst/.zshrc", "/dst/.gitconfig", "/dst/.tmux.conf"},
		},
		completed:    2,
		failedTarget: "/dst/.tmux.conf",
		err:          fmt.Errorf("apply failed"),
	})
	got := next.(Model)

	var targets []string
	for _, entry := range got.entries {
		targets = append(targets, entry.TargetPath)
	}
	wantTargets := []string{"/dst/.tmux.conf"}
	if strings.Join(targets, ",") != strings.Join(wantTargets, ",") {
		t.Fatalf("targets = %v, want %v", targets, wantTargets)
	}
}

func TestCommandPromptCreatesShellConfirmation(t *testing.T) {
	m := newTestModel([]model.Entry{
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
	})

	next, _ := m.Update(keyRunes("!"))
	m = next.(Model)
	if m.state != stateCommandInput {
		t.Fatalf("state = %v, want command input", m.state)
	}

	next, _ = m.Update(keyRunes("echo ok"))
	m = next.(Model)
	next, _ = m.Update(keyEnter())
	m = next.(Model)

	if m.state != stateConfirming {
		t.Fatalf("state = %v, want confirming", m.state)
	}
	if m.confirmAction.kind != actionShell {
		t.Fatalf("action kind = %v, want shell", m.confirmAction.kind)
	}
	if m.confirmAction.command != "echo ok" {
		t.Fatalf("command = %q, want echo ok", m.confirmAction.command)
	}
}

func TestSourceModeSwitchStartsSnapshotPreparation(t *testing.T) {
	m := newTestModel([]model.Entry{
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
	})

	next, _ := m.Update(keyRunes("2"))
	got := next.(Model)

	if got.applySourceMode != gitmode.SourceModeStaged {
		t.Fatalf("apply source = %v, want staged", got.applySourceMode)
	}
	if !got.snapshotLoading {
		t.Fatalf("expected snapshot preparation to start")
	}
}

func TestDiffFocusScrollsViewport(t *testing.T) {
	m := newTestModel([]model.Entry{
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.gitconfig"},
	})
	m.focusedPane = paneDiff
	selectedTarget := m.selectedEntry().TargetPath
	m.diffCache[selectedTarget] = diffState{
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

	next, _ = got.Update(diffRefreshDueMsg{
		targetPath:  got.pendingDiffPath,
		token:       got.pendingDiffSeq,
		resetScroll: true,
	})
	got = next.(Model)
	if got.diffViewport.YOffset != 0 {
		t.Fatalf("diff viewport offset = %d, want 0 after debounce", got.diffViewport.YOffset)
	}
}

func TestTabTogglesDiffFocusAndRestoresLastListPane(t *testing.T) {
	m := newTestModel([]model.Entry{
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
	})

	next, _ := m.Update(keyRunes("h"))
	m = next.(Model)
	if m.focusedPane != paneSrc {
		t.Fatalf("focused pane = %v, want src", m.focusedPane)
	}

	next, _ = m.Update(keyTab())
	m = next.(Model)
	if m.focusedPane != paneDiff {
		t.Fatalf("focused pane = %v, want diff", m.focusedPane)
	}

	next, _ = m.Update(keyTab())
	m = next.(Model)
	if m.focusedPane != paneSrc {
		t.Fatalf("focused pane = %v, want src after returning from diff", m.focusedPane)
	}

	next, _ = m.Update(keyRunes("l"))
	m = next.(Model)
	next, _ = m.Update(keyTab())
	m = next.(Model)
	next, _ = m.Update(keyTab())
	m = next.(Model)
	if m.focusedPane != paneTarget {
		t.Fatalf("focused pane = %v, want target after returning from diff", m.focusedPane)
	}
}

func TestMouseClickSelectsRowAndFocusesPane(t *testing.T) {
	m := newTestModel([]model.Entry{
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.gitconfig"},
	})

	targetMetrics := m.listPaneMetrics(paneTarget, m.layout().rect(paneTarget))
	next, _ := m.Update(mouseLeftPress(targetMetrics.pane.X+2, targetMetrics.pane.Y+targetMetrics.titleHeight+2))
	got := next.(Model)

	if got.focusedPane != paneTarget {
		t.Fatalf("focused pane = %v, want target", got.focusedPane)
	}
	if got.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", got.cursor)
	}

	entry := got.selectedEntry()
	if entry == nil {
		t.Fatalf("expected clicked row to have an entry")
	}
	state, ok := got.diffCache[entry.TargetPath]
	if !ok || !state.loading {
		t.Fatalf("expected clicked row diff to start loading, cache = %#v", state)
	}
}

func TestMouseClickFocusesDiffPane(t *testing.T) {
	m := newTestModel([]model.Entry{
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
	})

	layout := m.layout()
	next, _ := m.Update(mouseLeftPress(layout.diff.X+1, layout.diff.Y+1))
	got := next.(Model)

	if got.focusedPane != paneDiff {
		t.Fatalf("focused pane = %v, want diff", got.focusedPane)
	}
	if got.lastListPane != paneTarget {
		t.Fatalf("last list pane = %v, want target", got.lastListPane)
	}
}

func TestEnterExpandsUnmanagedDirectoryAndAllowsAddingChild(t *testing.T) {
	dir := t.TempDir()
	childPath := filepath.Join(dir, "child.txt")
	if err := os.WriteFile(childPath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write child file: %v", err)
	}

	m := newTestModel([]model.Entry{
		{Kind: model.EntryUnmanaged, TargetType: model.TargetDirectory, TargetPath: dir},
	})
	m.listMode = listModeAll

	next, _ := m.Update(keyEnter())
	m = next.(Model)
	if len(m.rows) != 2 {
		t.Fatalf("row count = %d, want 2", len(m.rows))
	}

	next, _ = m.Update(keyDown())
	m = next.(Model)
	entry := m.selectedEntry()
	if entry == nil || entry.TargetPath != childPath {
		t.Fatalf("selected entry = %#v, want child %q", entry, childPath)
	}

	next, _ = m.Update(keyRunes("i"))
	m = next.(Model)
	if m.state != stateConfirming || m.confirmAction.kind != actionAdd || m.confirmAction.entry.TargetPath != childPath {
		t.Fatalf("unexpected add confirmation after expanding directory: %#v", m.confirmAction)
	}
}

func TestFilterInputNarrowsTreeRows(t *testing.T) {
	m := newTestModel([]model.Entry{
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.config/nvim/init.lua"},
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.gitconfig"},
	})

	next, _ := m.Update(keyRunes("/"))
	m = next.(Model)
	next, _ = m.Update(keyRunes("nvim"))
	m = next.(Model)

	if m.state != stateFilterInput {
		t.Fatalf("state = %v, want filter input", m.state)
	}
	if m.filterQuery != "nvim" {
		t.Fatalf("filter query = %q, want nvim", m.filterQuery)
	}
	if len(m.rows) != 3 {
		t.Fatalf("row count = %d, want 3 filtered rows", len(m.rows))
	}
	entry := m.selectedEntry()
	if entry == nil || entry.TargetPath != "/dst/.config/nvim/init.lua" {
		t.Fatalf("selected entry = %#v, want filtered leaf", entry)
	}

	next, _ = m.Update(keyEnter())
	m = next.(Model)
	if m.state != stateNormal {
		t.Fatalf("state = %v, want normal", m.state)
	}
}

func TestSelectedRowStylesPath(t *testing.T) {
	m := newTestModel([]model.Entry{
		{
			Kind:       model.EntryManaged,
			SourceCode: model.StatusModified,
			TargetCode: model.StatusModified,
			SourcePath: "/src/dot_zshrc",
			TargetPath: "/dst/.zshrc",
		},
	})

	row := m.renderEntryRow(m.rows[0], paneSrc, true, true, 64)
	if !strings.Contains(row, selectedRowStyle.Render("- dot_zshrc")) {
		t.Fatalf("selected row should style the path, row = %q", row)
	}
}

func TestConfirmModalOverlaysMainView(t *testing.T) {
	m := newTestModel([]model.Entry{
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
	})
	m.state = stateConfirming
	m.confirmAction = pendingAction{kind: actionAdd, entry: m.entries[0]}

	view := m.View()
	if !strings.Contains(view, "Copy Current Target Into Source?") {
		t.Fatalf("confirm modal missing: %q", view)
	}
	if !strings.Contains(view, "src (1 rows)") {
		t.Fatalf("main view should remain visible behind modal: %q", view)
	}
}

func TestConfirmModalOverlayKeepsUnderlyingColumns(t *testing.T) {
	m := newTestModel([]model.Entry{
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
	})
	m.state = stateConfirming
	m.confirmAction = pendingAction{kind: actionAdd, entry: m.entries[0]}

	view := xansi.Strip(m.View())
	for _, line := range strings.Split(view, "\n") {
		if !strings.Contains(line, "Copy Current Target Into Source?") {
			continue
		}

		leftBorder := strings.IndexRune(line, '║')
		rightBorder := strings.LastIndex(line, "║")
		if leftBorder <= 0 || rightBorder <= leftBorder {
			t.Fatalf("unexpected modal line: %q", line)
		}
		if strings.TrimSpace(line[:leftBorder]) == "" {
			t.Fatalf("left side of overlay row was cleared: %q", line)
		}
		if strings.TrimSpace(line[rightBorder+1:]) == "" {
			t.Fatalf("right side of overlay row was cleared: %q", line)
		}
		return
	}

	t.Fatalf("overlay row with modal title not found: %q", view)
}

func TestHeaderShowsLoadingIndicator(t *testing.T) {
	m := newTestModel([]model.Entry{
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
	})
	m.entriesLoading = true

	header := m.renderHeader()
	if !strings.Contains(header, "Loading entries...") {
		t.Fatalf("header missing loading indicator: %q", header)
	}
}

func TestStaleDiffResultsAreIgnored(t *testing.T) {
	m := newTestModel([]model.Entry{
		{Kind: model.EntryManaged, SourceCode: model.StatusModified, TargetCode: model.StatusModified, TargetType: model.TargetFile, TargetPath: "/dst/.zshrc"},
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
	m.entriesLoading = false
	m.statusMsg = ""
	m.ready = true
	m.width = 120
	m.height = 24
	m.diffViewport = viewport.New(60, 4)
	m.rebuildRows("")
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

func keyTab() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyTab}
}

func keyEnter() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyEnter}
}

func mouseLeftPress(x, y int) tea.MouseMsg {
	return tea.MouseMsg{
		X:      x,
		Y:      y,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		Type:   tea.MouseLeft,
	}
}
