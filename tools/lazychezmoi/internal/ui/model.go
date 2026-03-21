package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazychezmoi/internal/chezmoi"
	gitmode "lazychezmoi/internal/git"
	"lazychezmoi/internal/model"
)

type paneKind int

const (
	paneTarget paneKind = iota
	paneSrc
	paneDiff
)

type appState int

const (
	stateNormal appState = iota
	stateConfirming
	stateCommandInput
	stateHelp
	stateRunningAction
)

type listMode int

const (
	listModeManaged listMode = iota
	listModeUnmanaged
)

func (m listMode) String() string {
	switch m {
	case listModeUnmanaged:
		return "unmanaged"
	default:
		return "managed"
	}
}

type pendingActionKind int

const (
	actionNone pendingActionKind = iota
	actionApply
	actionAdd
	actionDelete
	actionShell
)

func (k pendingActionKind) String() string {
	switch k {
	case actionApply:
		return "apply"
	case actionAdd:
		return "add"
	case actionDelete:
		return "delete"
	case actionShell:
		return "shell"
	default:
		return ""
	}
}

type pendingAction struct {
	kind    pendingActionKind
	targets []string
	entry   model.Entry
	command string
}

func (a pendingAction) valid() bool {
	return a.kind != actionNone
}

type diffState struct {
	content           string
	err               error
	loading           bool
	requestGeneration int
	requestID         int
}

type Model struct {
	chezmoi *chezmoi.Client

	entries []model.Entry
	cursor  int

	focusedPane     paneKind
	lastListPane    paneKind
	listMode        listMode
	applySourceMode gitmode.SourceMode

	selectedTargets map[string]struct{}
	confirmAction   pendingAction

	sourcePathCache map[string]string
	diffCache       map[string]diffState
	entryGeneration int
	diffRequestSeq  int

	sourceRoot        string
	snapshotRoot      string
	snapshotSource    string
	snapshotLoading   bool
	snapshotErr       error
	snapshotRequestID int

	diffViewport viewport.Model
	diffContent  string
	diffLoading  bool
	diffErr      error

	commandInput string

	width  int
	height int
	ready  bool

	state      appState
	statusMsg  string
	loadErr    error
	openEditor func(string) tea.Cmd
}

func New(client *chezmoi.Client) Model {
	return Model{
		chezmoi:         client,
		focusedPane:     paneTarget,
		lastListPane:    paneTarget,
		listMode:        listModeManaged,
		applySourceMode: gitmode.SourceModeWorkingTree,
		selectedTargets: make(map[string]struct{}),
		sourcePathCache: make(map[string]string),
		diffCache:       make(map[string]diffState),
		openEditor:      openEditorCmd,
	}
}

type paneRect struct {
	X      int
	Y      int
	Width  int
	Height int
}

func (r paneRect) contains(x, y int) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}

type uiLayout struct {
	src    paneRect
	target paneRect
	diff   paneRect
}

func (l uiLayout) rect(kind paneKind) paneRect {
	switch kind {
	case paneSrc:
		return l.src
	case paneDiff:
		return l.diff
	default:
		return l.target
	}
}

type listPaneMetrics struct {
	pane        paneRect
	title       string
	titleHeight int
	listWidth   int
	listHeight  int
	offset      int
}

func (m Model) layout() uiLayout {
	headerH := lipgloss.Height(m.renderHeader())
	footerH := lipgloss.Height(m.renderFooter())
	contentH := m.height - headerH - footerH
	if contentH < 2 {
		contentH = 2
	}

	leftW := m.width / 3
	rightW := m.width - leftW
	halfH := contentH / 2
	srcH := halfH
	targetH := contentH - halfH

	return uiLayout{
		src: paneRect{
			X:      0,
			Y:      headerH,
			Width:  leftW,
			Height: srcH,
		},
		target: paneRect{
			X:      0,
			Y:      headerH + srcH,
			Width:  leftW,
			Height: targetH,
		},
		diff: paneRect{
			X:      leftW,
			Y:      headerH,
			Width:  rightW,
			Height: contentH,
		},
	}
}

func (m Model) listPaneTitle(kind paneKind) string {
	title := "src"
	switch kind {
	case paneTarget:
		if m.listMode == listModeManaged {
			title = "target (apply queue)"
		} else {
			title = "target (unmanaged)"
		}
	case paneSrc:
		if m.listMode == listModeUnmanaged {
			title = "src (missing)"
		}
	}

	titleText := fmt.Sprintf(" %s (%d)", title, len(m.entries))
	if kind == paneTarget && m.selectedTargetCount() > 0 {
		titleText = fmt.Sprintf(" %s (%d queued)", title, m.selectedTargetCount())
	}

	return titleText
}

func (m Model) listPaneMetrics(kind paneKind, rect paneRect) listPaneMetrics {
	title := m.listPaneTitle(kind)
	titleHeight := lipgloss.Height(inactiveTitleStyle.Render(title))
	listHeight := rect.Height - titleHeight - 2
	if listHeight < 1 {
		listHeight = 1
	}
	listWidth := rect.Width - 2
	if listWidth < 1 {
		listWidth = 1
	}

	offset := 0
	if m.cursor >= listHeight {
		offset = m.cursor - listHeight + 1
	}

	return listPaneMetrics{
		pane:        rect,
		title:       title,
		titleHeight: titleHeight,
		listWidth:   listWidth,
		listHeight:  listHeight,
		offset:      offset,
	}
}

func (m listPaneMetrics) rowIndex(x, y, entryCount int) (int, bool) {
	contentX := m.pane.X + 1
	contentY := m.pane.Y + m.titleHeight + 1
	if x < contentX || x >= contentX+m.listWidth || y < contentY || y >= contentY+m.listHeight {
		return 0, false
	}

	row := m.offset + (y - contentY)
	if row < 0 || row >= entryCount {
		return 0, false
	}

	return row, true
}

func normalizeListPane(kind paneKind) paneKind {
	if kind == paneSrc {
		return paneSrc
	}
	return paneTarget
}

func (m *Model) setFocusedPane(kind paneKind) {
	m.focusedPane = kind
	if kind != paneDiff {
		m.lastListPane = normalizeListPane(kind)
	}
}

func (m Model) restoreListPane() paneKind {
	return normalizeListPane(m.lastListPane)
}

func (m *Model) toggleDiffPaneFocus() {
	if m.focusedPane == paneDiff {
		m.setFocusedPane(m.restoreListPane())
		return
	}
	m.setFocusedPane(paneDiff)
}

func (m Model) Cleanup() error {
	if m.snapshotRoot == "" {
		return nil
	}
	return os.RemoveAll(m.snapshotRoot)
}

func (m Model) selectedEntry() *model.Entry {
	if len(m.entries) == 0 || m.cursor < 0 || m.cursor >= len(m.entries) {
		return nil
	}
	return &m.entries[m.cursor]
}

func (m Model) entryIndex(targetPath string) int {
	for i := range m.entries {
		if m.entries[i].TargetPath == targetPath {
			return i
		}
	}
	return -1
}

func (m Model) entryByTarget(targetPath string) *model.Entry {
	index := m.entryIndex(targetPath)
	if index < 0 {
		return nil
	}
	return &m.entries[index]
}

func (m Model) isTargetSelected(targetPath string) bool {
	_, ok := m.selectedTargets[targetPath]
	return ok
}

func (m Model) selectedTargetCount() int {
	if m.listMode != listModeManaged {
		return 0
	}
	return len(m.orderedSelectedTargets())
}

func (m Model) orderedSelectedTargets() []string {
	targets := make([]string, 0, len(m.selectedTargets))
	for _, entry := range m.entries {
		if entry.CanApply() && m.isTargetSelected(entry.TargetPath) {
			targets = append(targets, entry.TargetPath)
		}
	}
	return targets
}

func (m *Model) toggleTargetSelection(targetPath string) bool {
	entry := m.entryByTarget(targetPath)
	if entry == nil || !entry.CanApply() {
		return false
	}
	if m.isTargetSelected(targetPath) {
		delete(m.selectedTargets, targetPath)
		return false
	}
	m.selectedTargets[targetPath] = struct{}{}
	return true
}

func (m *Model) clearTargetSelections(targetPaths ...string) {
	if len(targetPaths) == 0 {
		clear(m.selectedTargets)
		return
	}
	for _, targetPath := range targetPaths {
		delete(m.selectedTargets, targetPath)
	}
}

func (m *Model) reconcileSelections() {
	if m.listMode != listModeManaged {
		return
	}

	valid := make(map[string]struct{}, len(m.selectedTargets))
	for _, entry := range m.entries {
		if entry.CanApply() && m.isTargetSelected(entry.TargetPath) {
			valid[entry.TargetPath] = struct{}{}
		}
	}
	m.selectedTargets = valid
}

func (m Model) currentApplyTargets() []string {
	if m.listMode != listModeManaged {
		return nil
	}
	if targets := m.orderedSelectedTargets(); len(targets) > 0 {
		return targets
	}
	entry := m.selectedEntry()
	if entry == nil || !entry.CanApply() {
		return nil
	}
	return []string{entry.TargetPath}
}

func (m *Model) resetCommandInput() {
	m.commandInput = ""
}

func (m *Model) invalidateDiffs() {
	m.diffCache = make(map[string]diffState)
	m.diffContent = ""
	m.diffErr = nil
	m.diffLoading = false
	m.diffViewport.SetContent("")
}

func (m *Model) invalidateSnapshot() {
	if m.snapshotRoot != "" {
		_ = os.RemoveAll(m.snapshotRoot)
	}
	m.snapshotRoot = ""
	m.snapshotSource = ""
	m.snapshotLoading = false
	m.snapshotErr = nil
}

func (m Model) diffSourceUnavailable() error {
	if !m.applySourceMode.RequiresSnapshot() {
		return nil
	}
	if m.snapshotLoading {
		return fmt.Errorf("%s snapshot is still preparing", m.applySourceMode)
	}
	if m.snapshotErr != nil {
		return fmt.Errorf("%s snapshot failed: %w", m.applySourceMode, m.snapshotErr)
	}
	if m.snapshotSource == "" {
		return fmt.Errorf("%s snapshot is unavailable", m.applySourceMode)
	}
	return nil
}

func (m Model) shellEnv(entry model.Entry) []string {
	return []string{
		fmt.Sprintf("LAZYCHEZMOI_TARGET_PATH=%s", entry.TargetPath),
		fmt.Sprintf("LAZYCHEZMOI_SOURCE_PATH=%s", entry.SourcePath),
		fmt.Sprintf("LAZYCHEZMOI_ENTRY_MODE=%s", entry.Kind.String()),
		fmt.Sprintf("LAZYCHEZMOI_TARGET_KIND=%s", entry.TargetType.String()),
		fmt.Sprintf("LAZYCHEZMOI_APPLY_SOURCE=%s", m.applySourceMode.String()),
		fmt.Sprintf("LAZYCHEZMOI_LIST_MODE=%s", m.listMode.String()),
	}
}

func applySourceModeFromKey(key string) (gitmode.SourceMode, bool) {
	switch key {
	case "1":
		return gitmode.SourceModeWorkingTree, true
	case "2":
		return gitmode.SourceModeStaged, true
	case "3":
		return gitmode.SourceModeHead, true
	default:
		return gitmode.SourceModeWorkingTree, false
	}
}
