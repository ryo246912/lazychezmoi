package ui

import (
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"lazychezmoi/internal/chezmoi"
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
	stateHelp
	stateApplying
)

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

	focusedPane paneKind

	selectedTargets map[string]struct{}
	confirmTargets  []string

	sourcePathCache map[string]string
	diffCache       map[string]diffState
	entryGeneration int
	diffRequestSeq  int

	diffViewport viewport.Model
	diffContent  string
	diffLoading  bool
	diffErr      error

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
		selectedTargets: make(map[string]struct{}),
		sourcePathCache: make(map[string]string),
		diffCache:       make(map[string]diffState),
		openEditor:      openEditorCmd,
	}
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
	valid := make(map[string]struct{}, len(m.selectedTargets))
	for _, entry := range m.entries {
		if entry.CanApply() && m.isTargetSelected(entry.TargetPath) {
			valid[entry.TargetPath] = struct{}{}
		}
	}
	m.selectedTargets = valid
}

func (m Model) currentApplyTargets() []string {
	if targets := m.orderedSelectedTargets(); len(targets) > 0 {
		return targets
	}
	entry := m.selectedEntry()
	if entry == nil || !entry.CanApply() {
		return nil
	}
	return []string{entry.TargetPath}
}
