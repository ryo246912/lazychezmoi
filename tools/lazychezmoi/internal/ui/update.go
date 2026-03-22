package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazychezmoi/internal/diff"
	gitmode "lazychezmoi/internal/git"
	"lazychezmoi/internal/model"
)

type entriesLoadedMsg struct {
	mode    listMode
	entries []model.Entry
}

type entriesErrMsg struct {
	mode listMode
	err  error
}

type diffLoadedMsg struct {
	targetPath string
	generation int
	requestID  int
	content    string
}

type diffErrMsg struct {
	targetPath string
	generation int
	requestID  int
	err        error
}

type sourcePathMsg struct {
	targetPath string
	generation int
	path       string
}

type snapshotReadyMsg struct {
	mode       gitmode.SourceMode
	requestID  int
	rootDir    string
	sourceDir  string
	sourceRoot string
}

type snapshotErrMsg struct {
	mode      gitmode.SourceMode
	requestID int
	err       error
}

type actionDoneMsg struct {
	action       pendingAction
	hasConflicts bool
}

type patchConflictMsg struct {
	entry   model.Entry
	applied []byte
}

type actionErrMsg struct {
	action       pendingAction
	completed    int
	failedTarget string
	err          error
}

type editorErrMsg struct{ err error }

func (m Model) loadEntriesCmd() tea.Cmd {
	currentMode := m.listMode
	chezmoi := m.chezmoi
	return func() tea.Msg {
		switch currentMode {
		case listModeAll:
			managed, err := chezmoi.Status()
			if err != nil {
				return entriesErrMsg{mode: currentMode, err: err}
			}
			unmanaged, err := chezmoi.Unmanaged()
			if err != nil {
				return entriesErrMsg{mode: currentMode, err: err}
			}
			entries := make([]model.Entry, 0, len(managed)+len(unmanaged))
			entries = append(entries, managed...)
			entries = append(entries, unmanaged...)
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].TargetPath < entries[j].TargetPath
			})
			return entriesLoadedMsg{mode: currentMode, entries: entries}
		default:
			entries, err := chezmoi.Status()
			if err != nil {
				return entriesErrMsg{mode: currentMode, err: err}
			}
			return entriesLoadedMsg{mode: currentMode, entries: entries}
		}
	}
}

func (m Model) loadDiffCmd(
	entry model.Entry,
	generation, requestID int,
	applySourceMode gitmode.SourceMode,
	snapshotSource string,
) tea.Cmd {
	return func() tea.Msg {
		if entry.Kind == model.EntryUnmanaged {
			content, err := buildUnmanagedDiff(entry)
			if err != nil {
				return diffErrMsg{
					targetPath: entry.TargetPath,
					generation: generation,
					requestID:  requestID,
					err:        err,
				}
			}
			return diffLoadedMsg{
				targetPath: entry.TargetPath,
				generation: generation,
				requestID:  requestID,
				content:    content,
			}
		}

		client := m.chezmoi
		if applySourceMode.RequiresSnapshot() {
			if snapshotSource == "" {
				return diffErrMsg{
					targetPath: entry.TargetPath,
					generation: generation,
					requestID:  requestID,
					err:        fmt.Errorf("%s snapshot is unavailable", applySourceMode),
				}
			}
			client = client.WithSource(snapshotSource)
		}

		dst, err := os.ReadFile(entry.TargetPath)
		if err != nil && !os.IsNotExist(err) {
			return diffErrMsg{
				targetPath: entry.TargetPath,
				generation: generation,
				requestID:  requestID,
				err:        fmt.Errorf("read destination: %w", err),
			}
		}

		rendered, err := client.Cat(entry.TargetPath)
		if err != nil {
			return diffErrMsg{
				targetPath: entry.TargetPath,
				generation: generation,
				requestID:  requestID,
				err:        fmt.Errorf("chezmoi cat: %w", err),
			}
		}

		if len(rendered) == 0 && len(dst) > 0 && entry.TargetCode != model.StatusDeleted {
			return diffErrMsg{
				targetPath: entry.TargetPath,
				generation: generation,
				requestID:  requestID,
				err:        fmt.Errorf("rendered template is empty (external command may be unavailable)"),
			}
		}

		renderedName := "rendered target (after apply)"
		if entry.SourcePath != "" {
			renderedName = fmt.Sprintf("%s (%s)", entry.SourcePath, applySourceMode)
		}

		content := diff.Compute(entry.TargetPath, dst, renderedName, rendered)
		return diffLoadedMsg{
			targetPath: entry.TargetPath,
			generation: generation,
			requestID:  requestID,
			content:    content,
		}
	}
}

func (m Model) loadSourcePathCmd(targetPath string, generation int) tea.Cmd {
	return func() tea.Msg {
		path, err := m.chezmoi.SourcePath(targetPath)
		if err != nil {
			return sourcePathMsg{targetPath: targetPath, generation: generation}
		}
		return sourcePathMsg{targetPath: targetPath, generation: generation, path: path}
	}
}

func (m Model) prepareSnapshotCmd(mode gitmode.SourceMode, requestID int) tea.Cmd {
	client := m.chezmoi
	sourceRoot := m.sourceRoot
	if sourceRoot == "" && client != nil && client.Source != "" {
		sourceRoot = client.Source
	}

	return func() tea.Msg {
		if sourceRoot == "" {
			resolved, err := client.SourceDir()
			if err != nil {
				return snapshotErrMsg{mode: mode, requestID: requestID, err: err}
			}
			sourceRoot = resolved
		}

		snapshot, err := gitmode.New("", sourceRoot).Materialize(mode)
		if err != nil {
			return snapshotErrMsg{mode: mode, requestID: requestID, err: err}
		}

		return snapshotReadyMsg{
			mode:       mode,
			requestID:  requestID,
			rootDir:    snapshot.RootDir,
			sourceDir:  snapshot.SourceDir,
			sourceRoot: sourceRoot,
		}
	}
}

func (m Model) runActionCmd(action pendingAction) tea.Cmd {
	if action.kind == actionShell {
		return runShellCommandCmd(action.command, m.shellEnv(action.entry), action)
	}

	mode := m.applySourceMode
	snapshotSource := m.snapshotSource
	client := m.chezmoi

	return func() tea.Msg {
		switch action.kind {
		case actionApply:
			if mode.RequiresSnapshot() {
				if snapshotSource == "" {
					return actionErrMsg{action: action, err: fmt.Errorf("%s snapshot is unavailable", mode)}
				}
				client = client.WithSource(snapshotSource)
			}
			for i, targetPath := range action.targets {
				if err := client.Apply(targetPath); err != nil {
					return actionErrMsg{
						action:       action,
						completed:    i,
						failedTarget: targetPath,
						err:          err,
					}
				}
			}
		case actionAdd:
			if err := client.Add(action.entry.TargetPath); err != nil {
				return actionErrMsg{action: action, failedTarget: action.entry.TargetPath, err: err}
			}
		case actionDelete:
			if err := removeTargetPath(action.entry.TargetPath); err != nil {
				return actionErrMsg{action: action, failedTarget: action.entry.TargetPath, err: err}
			}
		case actionPatchSource:
			sourcePath := action.entry.SourcePath
			targetPath := action.entry.TargetPath

			dst, err := os.ReadFile(targetPath)
			if err != nil {
				return actionErrMsg{action: action, failedTarget: sourcePath, err: fmt.Errorf("read target: %w", err)}
			}
			rendered, err := client.Cat(targetPath)
			if err != nil {
				return actionErrMsg{action: action, failedTarget: sourcePath, err: fmt.Errorf("chezmoi cat: %w", err)}
			}
			srcContent, err := os.ReadFile(sourcePath)
			if err != nil {
				return actionErrMsg{action: action, failedTarget: sourcePath, err: fmt.Errorf("read source: %w", err)}
			}

			patchContent := diff.Compute("rendered", rendered, "target", dst)
			applied, hasConflicts := diff.ApplyWithConflicts(srcContent, patchContent)

			if hasConflicts {
				// Return for secondary confirmation before writing.
				return patchConflictMsg{entry: action.entry, applied: applied}
			}

			info, err := os.Stat(sourcePath)
			perm := os.FileMode(0o644)
			if err == nil {
				perm = info.Mode()
			}
			if err := os.WriteFile(sourcePath, applied, perm); err != nil {
				return actionErrMsg{action: action, failedTarget: sourcePath, err: fmt.Errorf("write source: %w", err)}
			}
			return actionDoneMsg{action: action}

		case actionPatchSourceConfirm:
			sourcePath := action.entry.SourcePath
			info, err := os.Stat(sourcePath)
			perm := os.FileMode(0o644)
			if err == nil {
				perm = info.Mode()
			}
			if err := os.WriteFile(sourcePath, action.patchResult, perm); err != nil {
				return actionErrMsg{action: action, failedTarget: sourcePath, err: fmt.Errorf("write source: %w", err)}
			}
			return actionDoneMsg{action: action, hasConflicts: true}

		default:
			return actionErrMsg{action: action, err: fmt.Errorf("unsupported action")}
		}

		return actionDoneMsg{action: action}
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadEntriesCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerH := lipgloss.Height(m.renderHeader())
		footerH := lipgloss.Height(m.renderFooter())
		contentH := max(2, m.height-headerH-footerH)

		rightW := m.width - m.width/3 - 2
		diffH := contentH - 2
		if !m.ready {
			m.diffViewport = viewport.New(rightW, diffH)
			m.ready = true
		} else {
			m.diffViewport.Width = rightW
			m.diffViewport.Height = diffH
		}
		m.syncDiffPreview(false)

	case entriesLoadedMsg:
		if msg.mode != m.listMode {
			break
		}

		m.entryGeneration++
		m.loadErr = nil
		m.entries = m.hydrateEntries(msg.entries)
		m.clampCursor()
		m.reconcileSelections()
		m.pruneCaches()
		m.invalidateDiffs()
		m.syncDiffPreview(true)

		cmds = append(cmds, m.loadSourcePathsCmd())
		if m.applySourceMode.RequiresSnapshot() {
			if m.snapshotSource == "" && !m.snapshotLoading {
				cmds = append(cmds, m.startSnapshotPreparation())
			} else if m.snapshotSource != "" {
				cmds = append(cmds, m.queueInitialDiffLoadsCmd())
			}
		} else {
			cmds = append(cmds, m.queueInitialDiffLoadsCmd())
		}

	case entriesErrMsg:
		if msg.mode != m.listMode {
			break
		}
		m.loadErr = msg.err
		m.entries = nil
		m.confirmAction = pendingAction{}
		m.invalidateDiffs()
		m.syncDiffPreview(true)

	case sourcePathMsg:
		if msg.generation != m.entryGeneration {
			break
		}
		if msg.path != "" {
			m.sourcePathCache[msg.targetPath] = msg.path
		}
		if index := m.entryIndex(msg.targetPath); index >= 0 {
			m.entries[index].SourcePath = msg.path
		}

	case snapshotReadyMsg:
		if msg.mode != m.applySourceMode || msg.requestID != m.snapshotRequestID {
			if msg.rootDir != "" {
				_ = os.RemoveAll(msg.rootDir)
			}
			break
		}
		if m.snapshotRoot != "" && m.snapshotRoot != msg.rootDir {
			_ = os.RemoveAll(m.snapshotRoot)
		}
		m.snapshotLoading = false
		m.snapshotErr = nil
		m.snapshotRoot = msg.rootDir
		m.snapshotSource = msg.sourceDir
		m.sourceRoot = msg.sourceRoot
		cmds = append(cmds, m.queueInitialDiffLoadsCmd())

	case snapshotErrMsg:
		if msg.mode != m.applySourceMode || msg.requestID != m.snapshotRequestID {
			break
		}
		m.snapshotLoading = false
		m.snapshotErr = msg.err
		m.statusMsg = fmt.Sprintf("Failed to prepare %s snapshot: %v", msg.mode, msg.err)
		m.syncDiffPreview(true)

	case diffLoadedMsg:
		if !m.acceptDiffResult(msg.targetPath, msg.generation, msg.requestID) {
			break
		}
		state := m.diffCache[msg.targetPath]
		state.content = msg.content
		state.err = nil
		state.loading = false
		m.diffCache[msg.targetPath] = state
		if entry := m.selectedEntry(); entry != nil && entry.TargetPath == msg.targetPath {
			m.syncDiffPreview(false)
		}

	case diffErrMsg:
		if !m.acceptDiffResult(msg.targetPath, msg.generation, msg.requestID) {
			break
		}
		state := m.diffCache[msg.targetPath]
		state.loading = false
		if state.content == "" {
			state.err = msg.err
		} else {
			m.statusMsg = fmt.Sprintf("Diff refresh failed for %s: %v", msg.targetPath, msg.err)
		}
		m.diffCache[msg.targetPath] = state
		if entry := m.selectedEntry(); entry != nil && entry.TargetPath == msg.targetPath {
			m.syncDiffPreview(false)
		}

	case patchConflictMsg:
		m.state = stateConfirming
		m.confirmAction = pendingAction{
			kind:        actionPatchSourceConfirm,
			entry:       msg.entry,
			patchResult: msg.applied,
		}
		m.statusMsg = fmt.Sprintf("Conflicts found in %s. Apply with conflict markers?", msg.entry.SourcePath)

	case actionDoneMsg:
		m.state = stateNormal
		if msg.action.kind == actionApply {
			m.clearTargetSelections(msg.action.targets...)
		}
		m.confirmAction = pendingAction{}
		if m.applySourceMode.RequiresSnapshot() {
			m.invalidateSnapshot()
		}
		m.applySuccessfulAction(msg.action)
		if msg.hasConflicts {
			m.statusMsg = fmt.Sprintf("Patched %s with conflicts - resolve manually ('e' to edit)", msg.action.entry.SourcePath)
		} else {
			m.statusMsg = actionSuccessMessage(msg.action, m.applySourceMode)
		}
		cmds = append(cmds, m.loadEntriesCmd())

	case actionErrMsg:
		m.state = stateNormal
		m.confirmAction = pendingAction{}
		if msg.action.kind == actionApply && msg.completed > 0 {
			m.applySuccessfulAction(pendingAction{
				kind:    actionApply,
				targets: append([]string(nil), msg.action.targets[:msg.completed]...),
			})
		}
		if m.applySourceMode.RequiresSnapshot() {
			m.invalidateSnapshot()
		}
		m.statusMsg = actionFailureMessage(msg, m.applySourceMode)
		cmds = append(cmds, m.loadEntriesCmd())

	case editorErrMsg:
		m.statusMsg = fmt.Sprintf("Editor error: %v", msg.err)

	case tea.MouseMsg:
		switch m.state {
		case stateNormal:
			if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
				cmds = append(cmds, m.handleMouseClick(msg))
				break
			}
			if m.focusedPane == paneDiff {
				var cmd tea.Cmd
				m.diffViewport, cmd = m.diffViewport.Update(msg)
				cmds = append(cmds, cmd)
			}
		case stateConfirming, stateCommandInput, stateHelp, stateRunningAction:
			// Ignore mouse input outside the main browsing state.
		}

	case tea.KeyMsg:
		switch m.state {
		case stateHelp:
			if msg.String() == "?" || msg.String() == "q" || msg.String() == "esc" {
				m.state = stateNormal
			}

		case stateCommandInput:
			switch msg.String() {
			case "esc":
				m.state = stateNormal
				m.resetCommandInput()
				m.statusMsg = "Cancelled"
			case "enter":
				command := strings.TrimSpace(m.commandInput)
				if command == "" {
					m.state = stateNormal
					m.statusMsg = "Cancelled"
					break
				}
				entry := m.selectedEntry()
				if entry == nil {
					m.state = stateNormal
					m.statusMsg = "No entry selected"
					break
				}
				m.confirmAction = pendingAction{
					kind:    actionShell,
					entry:   *entry,
					command: command,
				}
				m.state = stateConfirming
				m.statusMsg = ""
			case "backspace", "ctrl+h":
				if len(m.commandInput) > 0 {
					m.commandInput = m.commandInput[:len(m.commandInput)-1]
				}
			default:
				if msg.Type == tea.KeyRunes {
					m.commandInput += string(msg.Runes)
				}
			}

		case stateConfirming:
			switch msg.String() {
			case "y", "Y":
				if !m.confirmAction.valid() {
					m.state = stateNormal
					break
				}
				if m.confirmAction.kind == actionApply {
					if err := m.diffSourceUnavailable(); err != nil {
						m.state = stateNormal
						m.confirmAction = pendingAction{}
						m.statusMsg = err.Error()
						break
					}
				}
				m.state = stateRunningAction
				m.statusMsg = runningActionMessage(m.confirmAction, m.applySourceMode)
				cmds = append(cmds, m.runActionCmd(m.confirmAction))
			case "n", "N", "esc":
				m.state = stateNormal
				m.confirmAction = pendingAction{}
				m.statusMsg = "Cancelled"
			}

		case stateRunningAction:
			// Ignore keys while commands are running.

		case stateNormal:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit

			case "?":
				m.state = stateHelp

			case "r":
				m.statusMsg = "Refreshing..."
				m.invalidateDiffs()
				if m.applySourceMode.RequiresSnapshot() {
					m.invalidateSnapshot()
				}
				cmds = append(cmds, m.loadEntriesCmd())

			case "m":
				if m.listMode == listModeManaged {
					m.listMode = listModeAll
				} else {
					m.listMode = listModeManaged
				}
				m.statusMsg = fmt.Sprintf("Switched to %s mode", m.listMode)
				m.invalidateDiffs()
				cmds = append(cmds, m.loadEntriesCmd())

			case "1", "2", "3":
				mode, ok := applySourceModeFromKey(msg.String())
				if !ok || mode == m.applySourceMode {
					break
				}
				m.applySourceMode = mode
				m.statusMsg = fmt.Sprintf("Apply source set to %s", mode)
				m.invalidateDiffs()
				m.invalidateSnapshot()
				if mode.RequiresSnapshot() {
					cmds = append(cmds, m.startSnapshotPreparation())
				} else {
					cmds = append(cmds, m.queueInitialDiffLoadsCmd())
				}

			case "tab":
				m.toggleDiffPaneFocus()

			case "shift+tab":
				m.toggleDiffPaneFocus()

			case "h":
				if m.focusedPane != paneDiff {
					m.setFocusedPane(paneSrc)
				}

			case "l":
				if m.focusedPane != paneDiff {
					m.setFocusedPane(paneTarget)
				}

			case "j", "down":
				if m.focusedPane == paneDiff {
					var cmd tea.Cmd
					m.diffViewport, cmd = m.diffViewport.Update(msg)
					cmds = append(cmds, cmd)
					break
				}
				if m.cursor < len(m.entries)-1 {
					m.cursor++
					cmds = append(cmds, m.refreshSelectedDiffCmd(true))
				}

			case "k", "up":
				if m.focusedPane == paneDiff {
					var cmd tea.Cmd
					m.diffViewport, cmd = m.diffViewport.Update(msg)
					cmds = append(cmds, cmd)
					break
				}
				if m.cursor > 0 {
					m.cursor--
					cmds = append(cmds, m.refreshSelectedDiffCmd(true))
				}

			case " ", "space":
				if m.focusedPane != paneTarget {
					break
				}
				entry := m.selectedEntry()
				if entry == nil || !entry.CanApply() {
					break
				}
				m.toggleTargetSelection(entry.TargetPath)
				if count := m.selectedTargetCount(); count > 0 {
					m.statusMsg = fmt.Sprintf("%d file(s) queued", count)
				} else {
					m.statusMsg = "Apply queue cleared"
				}

			case "a":
				if m.focusedPane != paneTarget {
					break
				}
				targets := m.currentApplyTargets()
				if len(targets) == 0 {
					break
				}
				if err := m.diffSourceUnavailable(); err != nil {
					m.statusMsg = err.Error()
					break
				}
				entry := m.selectedEntry()
				if entry == nil {
					break
				}
				m.confirmAction = pendingAction{
					kind:    actionApply,
					targets: targets,
					entry:   *entry,
				}
				m.state = stateConfirming
				m.statusMsg = ""

			case "i":
				if m.focusedPane != paneTarget {
					break
				}
				entry := m.selectedEntry()
				if entry == nil || !entry.CanAdd() {
					break
				}
				if entry.IsTemplate() {
					m.confirmAction = pendingAction{kind: actionPatchSource, entry: *entry}
				} else {
					m.confirmAction = pendingAction{kind: actionAdd, entry: *entry}
				}
				m.state = stateConfirming
				m.statusMsg = ""

			case "d":
				if m.focusedPane != paneTarget {
					break
				}
				entry := m.selectedEntry()
				if entry == nil || !entry.CanDeleteTarget() {
					break
				}
				m.confirmAction = pendingAction{kind: actionDelete, entry: *entry}
				m.state = stateConfirming
				m.statusMsg = ""

			case "!":
				entry := m.selectedEntry()
				if entry == nil {
					break
				}
				m.state = stateCommandInput
				m.resetCommandInput()
				m.statusMsg = ""

			case "e":
				entry := m.selectedEntry()
				if entry == nil {
					break
				}
				switch m.focusedPane {
				case paneSrc:
					switch {
					case entry.Kind == model.EntryUnmanaged:
						m.statusMsg = "Unmanaged entries do not have a source file yet"
					case !entry.CanEditSource():
						m.statusMsg = "Source path is still resolving"
					default:
						cmds = append(cmds, m.openEditor(entry.SourcePath))
					}
				case paneTarget:
					if !entry.CanEditTarget() {
						m.statusMsg = "Directories cannot be opened in $EDITOR"
						break
					}
					cmds = append(cmds, m.openEditor(entry.TargetPath))
				case paneDiff:
					m.statusMsg = "Move focus to src or target to edit a file"
				}

			default:
				if m.focusedPane == paneDiff {
					var cmd tea.Cmd
					m.diffViewport, cmd = m.diffViewport.Update(msg)
					cmds = append(cmds, cmd)
				}
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) clampCursor() {
	switch {
	case len(m.entries) == 0:
		m.cursor = 0
	case m.cursor < 0:
		m.cursor = 0
	case m.cursor >= len(m.entries):
		m.cursor = len(m.entries) - 1
	}
}

func (m Model) hydrateEntries(entries []model.Entry) []model.Entry {
	hydrated := make([]model.Entry, len(entries))
	copy(hydrated, entries)
	for i := range hydrated {
		if hydrated[i].Kind != model.EntryManaged {
			continue
		}
		if path, ok := m.sourcePathCache[hydrated[i].TargetPath]; ok {
			hydrated[i].SourcePath = path
		}
	}
	return hydrated
}

func (m *Model) pruneCaches() {
	currentTargets := make(map[string]struct{}, len(m.entries))
	for _, entry := range m.entries {
		currentTargets[entry.TargetPath] = struct{}{}
	}
	for targetPath := range m.sourcePathCache {
		if _, ok := currentTargets[targetPath]; !ok {
			delete(m.sourcePathCache, targetPath)
		}
	}
	for targetPath := range m.diffCache {
		if _, ok := currentTargets[targetPath]; !ok {
			delete(m.diffCache, targetPath)
		}
	}
}

func (m Model) loadSourcePathsCmd() tea.Cmd {
	var cmds []tea.Cmd
	for _, entry := range m.entries {
		if entry.Kind != model.EntryManaged || entry.SourcePath != "" {
			continue
		}
		cmds = append(cmds, m.loadSourcePathCmd(entry.TargetPath, m.entryGeneration))
	}
	return tea.Batch(cmds...)
}

func (m *Model) startSnapshotPreparation() tea.Cmd {
	m.snapshotLoading = true
	m.snapshotErr = nil
	m.snapshotRequestID++
	return m.prepareSnapshotCmd(m.applySourceMode, m.snapshotRequestID)
}

func (m *Model) requestDiffLoadCmd(targetPath string) tea.Cmd {
	entry := m.entryByTarget(targetPath)
	if entry == nil {
		return nil
	}
	if entry.Kind == model.EntryManaged && m.applySourceMode.RequiresSnapshot() && m.snapshotSource == "" {
		return nil
	}

	m.diffRequestSeq++
	state := m.diffCache[targetPath]
	state.loading = true
	state.requestGeneration = m.entryGeneration
	state.requestID = m.diffRequestSeq
	if state.content == "" {
		state.err = nil
	}
	m.diffCache[targetPath] = state

	return m.loadDiffCmd(*entry, state.requestGeneration, state.requestID, m.applySourceMode, m.snapshotSource)
}

func (m *Model) refreshSelectedDiffCmd(resetScroll bool) tea.Cmd {
	entry := m.selectedEntry()
	if entry == nil {
		m.diffContent = ""
		m.diffErr = nil
		m.diffLoading = false
		m.diffViewport.SetContent("")
		if resetScroll {
			m.diffViewport.GotoTop()
		}
		return nil
	}

	if entry.Kind == model.EntryManaged && m.applySourceMode.RequiresSnapshot() && m.snapshotSource == "" {
		m.syncDiffPreview(resetScroll)
		return nil
	}

	cmd := m.requestDiffLoadCmd(entry.TargetPath)
	m.syncDiffPreview(resetScroll)
	return cmd
}

func (m *Model) queueInitialDiffLoadsCmd() tea.Cmd {
	entry := m.selectedEntry()
	if entry == nil {
		return nil
	}
	if entry.Kind == model.EntryManaged && m.applySourceMode.RequiresSnapshot() && m.snapshotSource == "" {
		return nil
	}

	priority := m.requestDiffLoadCmd(entry.TargetPath)
	var background []tea.Cmd
	for _, current := range m.entries {
		if current.TargetPath == entry.TargetPath {
			continue
		}
		background = append(background, m.requestDiffLoadCmd(current.TargetPath))
	}
	if len(background) == 0 {
		return priority
	}
	return tea.Sequence(priority, tea.Batch(background...))
}

func (m *Model) syncDiffPreview(resetScroll bool) {
	m.diffContent = ""
	m.diffErr = nil
	m.diffLoading = false

	if entry := m.selectedEntry(); entry != nil {
		if state, ok := m.diffCache[entry.TargetPath]; ok {
			m.diffContent = state.content
			m.diffLoading = state.loading
			if state.content == "" {
				m.diffErr = state.err
			}
		}
	}

	m.diffViewport.SetContent(colorizeDiff(m.diffContent))
	if resetScroll {
		m.diffViewport.GotoTop()
	}
}

func (m Model) acceptDiffResult(targetPath string, generation, requestID int) bool {
	state, ok := m.diffCache[targetPath]
	if !ok {
		return false
	}
	return state.requestGeneration == generation && state.requestID == requestID
}

func (m *Model) handleMouseClick(msg tea.MouseMsg) tea.Cmd {
	if !m.ready {
		return nil
	}

	layout := m.layout()
	switch {
	case layout.diff.contains(msg.X, msg.Y):
		m.setFocusedPane(paneDiff)
	case layout.src.contains(msg.X, msg.Y):
		m.setFocusedPane(paneSrc)
		return m.selectListRow(layout.rect(paneSrc), paneSrc, msg.X, msg.Y)
	case layout.target.contains(msg.X, msg.Y):
		m.setFocusedPane(paneTarget)
		return m.selectListRow(layout.rect(paneTarget), paneTarget, msg.X, msg.Y)
	}

	return nil
}

func (m *Model) selectListRow(rect paneRect, kind paneKind, x, y int) tea.Cmd {
	row, ok := m.listPaneMetrics(kind, rect).rowIndex(x, y, len(m.entries))
	if !ok || row == m.cursor {
		return nil
	}

	m.cursor = row
	return m.refreshSelectedDiffCmd(true)
}

func openEditorCmd(path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.CommandContext(context.Background(), editor, path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return editorErrMsg{err: fmt.Errorf("editor: %w", err)}
		}
		return nil
	})
}

func runShellCommandCmd(command string, env []string, action pendingAction) tea.Cmd {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}

	cmd := exec.CommandContext(context.Background(), shell, "-lc", command)
	cmd.Env = append(os.Environ(), env...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return actionErrMsg{action: action, err: fmt.Errorf("shell command: %w", err)}
		}
		return actionDoneMsg{action: action}
	})
}

func buildUnmanagedDiff(entry model.Entry) (string, error) {
	content, label, err := unmanagedPreview(entry)
	if err != nil {
		return "", err
	}
	return diff.Compute("source (missing)", nil, label, content), nil
}

func unmanagedPreview(entry model.Entry) ([]byte, string, error) {
	switch entry.TargetType {
	case model.TargetDirectory:
		entries, err := os.ReadDir(entry.TargetPath)
		if err != nil {
			return nil, "", fmt.Errorf("read directory: %w", err)
		}
		var lines []string
		for _, current := range entries {
			name := current.Name()
			if current.IsDir() {
				name += "/"
			}
			lines = append(lines, name)
		}
		if len(lines) == 0 {
			lines = append(lines, "(empty directory)")
		}
		return []byte(strings.Join(lines, "\n") + "\n"), entry.TargetPath + " (directory)", nil
	case model.TargetSymlink:
		target, err := os.Readlink(entry.TargetPath)
		if err != nil {
			return nil, "", fmt.Errorf("read symlink: %w", err)
		}
		return []byte("symlink -> " + target + "\n"), entry.TargetPath + " (symlink)", nil
	default:
		content, err := os.ReadFile(entry.TargetPath)
		if err != nil && !os.IsNotExist(err) {
			return nil, "", fmt.Errorf("read target: %w", err)
		}
		return content, entry.TargetPath, nil
	}
}

func removeTargetPath(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

func runningActionMessage(action pendingAction, mode gitmode.SourceMode) string {
	switch action.kind {
	case actionApply:
		return fmt.Sprintf("Applying %d file(s) from %s...", len(action.targets), mode)
	case actionAdd:
		if action.entry.Kind == model.EntryManaged {
			return fmt.Sprintf("Updating source state from %s...", action.entry.TargetPath)
		}
		return fmt.Sprintf("Adding %s to source state...", action.entry.TargetPath)
	case actionDelete:
		return fmt.Sprintf("Deleting %s...", action.entry.TargetPath)
	case actionShell:
		return fmt.Sprintf("Running shell command for %s...", action.entry.TargetPath)
	case actionPatchSource:
		return fmt.Sprintf("Patching template source %s...", action.entry.SourcePath)
	case actionPatchSourceConfirm:
		return fmt.Sprintf("Writing conflict markers to %s...", action.entry.SourcePath)
	default:
		return "Running..."
	}
}

func actionSuccessMessage(action pendingAction, mode gitmode.SourceMode) string {
	switch action.kind {
	case actionApply:
		return fmt.Sprintf("Applied %d file(s) from %s", len(action.targets), mode)
	case actionAdd:
		if action.entry.Kind == model.EntryManaged {
			return fmt.Sprintf("Updated source state from %s", action.entry.TargetPath)
		}
		return fmt.Sprintf("Added %s to source state", action.entry.TargetPath)
	case actionDelete:
		return fmt.Sprintf("Deleted %s", action.entry.TargetPath)
	case actionShell:
		return fmt.Sprintf("Command finished: %s", action.command)
	case actionPatchSource:
		return fmt.Sprintf("Patched template source %s", action.entry.SourcePath)
	case actionPatchSourceConfirm:
		return fmt.Sprintf("Patched template source %s", action.entry.SourcePath)
	default:
		return "Done"
	}
}

func actionFailureMessage(msg actionErrMsg, mode gitmode.SourceMode) string {
	switch msg.action.kind {
	case actionApply:
		switch {
		case msg.completed > 0:
			return fmt.Sprintf(
				"Applied %d file(s) from %s before failing on %s: %v",
				msg.completed,
				mode,
				msg.failedTarget,
				msg.err,
			)
		case msg.failedTarget != "":
			return fmt.Sprintf("Failed to apply %s from %s: %v", msg.failedTarget, mode, msg.err)
		default:
			return fmt.Sprintf("Failed to apply from %s: %v", mode, msg.err)
		}
	case actionAdd:
		if msg.action.entry.Kind == model.EntryManaged {
			return fmt.Sprintf("Failed to update source state from %s: %v", msg.failedTarget, msg.err)
		}
		return fmt.Sprintf("Failed to add %s: %v", msg.failedTarget, msg.err)
	case actionDelete:
		return fmt.Sprintf("Failed to delete %s: %v", msg.failedTarget, msg.err)
	case actionShell:
		return fmt.Sprintf("Shell command failed: %v", msg.err)
	case actionPatchSource, actionPatchSourceConfirm:
		return fmt.Sprintf("Failed to patch template source %s: %v", msg.failedTarget, msg.err)
	default:
		return fmt.Sprintf("Command failed: %v", msg.err)
	}
}

func colorizeDiff(content string) string {
	if content == "" {
		return ""
	}

	var result strings.Builder
	for _, line := range strings.Split(content, "\n") {
		switch {
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			result.WriteString(diffAddStyle.Render(line))
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			result.WriteString(diffDelStyle.Render(line))
		default:
			result.WriteString(line)
		}
		result.WriteByte('\n')
	}

	return result.String()
}
