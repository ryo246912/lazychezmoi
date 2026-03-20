package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"lazychezmoi/internal/diff"
	"lazychezmoi/internal/model"
)

type entriesLoadedMsg struct{ entries []model.Entry }
type entriesErrMsg struct{ err error }

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

type applyDoneMsg struct{ targets []string }

type applyErrMsg struct {
	targets      []string
	appliedCount int
	failedTarget string
	err          error
}

type editorErrMsg struct{ err error }

func (m Model) loadEntriesCmd() tea.Cmd {
	return func() tea.Msg {
		entries, err := m.chezmoi.Status()
		if err != nil {
			return entriesErrMsg{err: err}
		}
		return entriesLoadedMsg{entries: entries}
	}
}

func (m Model) loadDiffCmd(entry model.Entry, generation, requestID int) tea.Cmd {
	return func() tea.Msg {
		// Read current destination file (what exists on disk now)
		dst, err := os.ReadFile(entry.TargetPath)
		if err != nil && !os.IsNotExist(err) {
			return diffErrMsg{
				targetPath: entry.TargetPath,
				generation: generation,
				requestID:  requestID,
				err:        fmt.Errorf("read destination: %w", err),
			}
		}

		// Get rendered target (what apply would write)
		rendered, err := m.chezmoi.Cat(entry.TargetPath)
		if err != nil {
			return diffErrMsg{
				targetPath: entry.TargetPath,
				generation: generation,
				requestID:  requestID,
				err:        fmt.Errorf("chezmoi cat: %w", err),
			}
		}

		// Sanity check: if rendered is empty but destination has content,
		// it likely means the template failed to render silently.
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
			renderedName = entry.SourcePath + " (rendered)"
		}

		// Diff direction: destination (now) -> rendered target (after apply).
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

func (m Model) applyTargetsCmd(targets []string) tea.Cmd {
	targets = append([]string(nil), targets...)
	return func() tea.Msg {
		for i, targetPath := range targets {
			if err := m.chezmoi.Apply(targetPath); err != nil {
				return applyErrMsg{
					targets:      targets,
					appliedCount: i,
					failedTarget: targetPath,
					err:          err,
				}
			}
		}
		return applyDoneMsg{targets: targets}
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

		headerH := 2
		footerH := 3
		contentH := m.height - headerH - footerH
		if contentH < 2 {
			contentH = 2
		}

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
		m.entryGeneration++
		m.loadErr = nil
		m.entries = m.hydrateEntries(msg.entries)
		m.clampCursor()
		m.reconcileSelections()
		m.pruneCaches()
		cmds = append(cmds, m.loadSourcePathsCmd())
		cmds = append(cmds, m.queueInitialDiffLoadsCmd())
		m.syncDiffPreview(true)

	case entriesErrMsg:
		m.loadErr = msg.err
		m.entries = nil
		m.confirmTargets = nil
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

	case applyDoneMsg:
		m.state = stateNormal
		m.confirmTargets = nil
		m.clearTargetSelections(msg.targets...)
		m.statusMsg = fmt.Sprintf("Applied %d file(s)", len(msg.targets))
		cmds = append(cmds, m.loadEntriesCmd())

	case applyErrMsg:
		m.state = stateNormal
		m.confirmTargets = nil
		switch {
		case msg.appliedCount > 0:
			m.statusMsg = fmt.Sprintf(
				"Applied %d file(s) before failing on %s: %v",
				msg.appliedCount,
				msg.failedTarget,
				msg.err,
			)
		case msg.failedTarget != "":
			m.statusMsg = fmt.Sprintf("Failed to apply %s: %v", msg.failedTarget, msg.err)
		default:
			m.statusMsg = fmt.Sprintf("Error applying: %v", msg.err)
		}
		cmds = append(cmds, m.loadEntriesCmd())

	case editorErrMsg:
		m.statusMsg = fmt.Sprintf("Editor error: %v", msg.err)

	case tea.KeyMsg:
		switch m.state {
		case stateHelp:
			if msg.String() == "?" || msg.String() == "q" || msg.String() == "esc" {
				m.state = stateNormal
			}

		case stateConfirming:
			switch msg.String() {
			case "y", "Y":
				if len(m.confirmTargets) == 0 {
					m.state = stateNormal
					break
				}
				m.state = stateApplying
				m.statusMsg = fmt.Sprintf("Applying %d file(s)...", len(m.confirmTargets))
				cmds = append(cmds, m.applyTargetsCmd(m.confirmTargets))
			case "n", "N", "esc":
				m.state = stateNormal
				m.confirmTargets = nil
				m.statusMsg = "Cancelled"
			}

		case stateApplying:
			// Ignore keys while applying.

		case stateNormal:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit

			case "?":
				m.state = stateHelp

			case "r":
				m.statusMsg = "Refreshing..."
				cmds = append(cmds, m.loadEntriesCmd())

			case "tab":
				m.focusedPane = nextPane(m.focusedPane)

			case "shift+tab":
				m.focusedPane = prevPane(m.focusedPane)

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
				m.confirmTargets = targets
				m.state = stateConfirming
				m.statusMsg = ""

			case "e":
				entry := m.selectedEntry()
				if entry == nil {
					break
				}
				switch m.focusedPane {
				case paneSrc:
					if entry.SourcePath == "" {
						m.statusMsg = "Source path is still resolving"
						break
					}
					cmds = append(cmds, m.openEditor(entry.SourcePath))
				case paneTarget:
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
		if entry.SourcePath != "" {
			continue
		}
		cmds = append(cmds, m.loadSourcePathCmd(entry.TargetPath, m.entryGeneration))
	}
	return tea.Batch(cmds...)
}

func (m *Model) requestDiffLoadCmd(targetPath string) tea.Cmd {
	entry := m.entryByTarget(targetPath)
	if entry == nil {
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

	return m.loadDiffCmd(*entry, state.requestGeneration, state.requestID)
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

	cmd := m.requestDiffLoadCmd(entry.TargetPath)
	m.syncDiffPreview(resetScroll)
	return cmd
}

func (m *Model) queueInitialDiffLoadsCmd() tea.Cmd {
	entry := m.selectedEntry()
	if entry == nil {
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

func nextPane(current paneKind) paneKind {
	switch current {
	case paneTarget:
		return paneSrc
	case paneSrc:
		return paneDiff
	default:
		return paneTarget
	}
}

func prevPane(current paneKind) paneKind {
	switch current {
	case paneTarget:
		return paneDiff
	case paneSrc:
		return paneTarget
	default:
		return paneSrc
	}
}

func openEditorCmd(path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return editorErrMsg{err: fmt.Errorf("editor: %w", err)}
		}
		return nil
	})
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
