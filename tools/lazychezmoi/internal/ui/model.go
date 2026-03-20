package ui

import (
	"github.com/charmbracelet/bubbles/viewport"

	"lazychezmoi/internal/chezmoi"
	"lazychezmoi/internal/model"
)

type paneKind int

const (
	paneTarget paneKind = iota
	paneSrc
)

type appState int

const (
	stateNormal appState = iota
	stateConfirming
	stateHelp
	stateApplying
)

type Model struct {
	chezmoi *chezmoi.Client

	entries []model.Entry
	cursor  int // shared cursor index into entries

	focusedPane paneKind

	diffViewport viewport.Model
	diffContent  string
	diffLoading  bool
	diffErr      error

	width  int
	height int
	ready  bool

	state     appState
	statusMsg string
	loadErr   error
}

func New(client *chezmoi.Client) Model {
	return Model{
		chezmoi:     client,
		focusedPane: paneTarget,
	}
}

func (m Model) selectedEntry() *model.Entry {
	if len(m.entries) == 0 || m.cursor < 0 || m.cursor >= len(m.entries) {
		return nil
	}
	return &m.entries[m.cursor]
}
