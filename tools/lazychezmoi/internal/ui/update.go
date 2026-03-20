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

// Messages

type entriesLoadedMsg struct{ entries []model.Entry }
type entriesErrMsg struct{ err error }
type diffLoadedMsg struct {
	targetPath string
	content    string
}
type diffErrMsg struct {
	targetPath string
	err        error
}
type sourcePathMsg struct {
	index int
	path  string
}
type applyDoneMsg struct{ targetPath string }
type applyErrMsg struct {
	targetPath string
	err        error
}

// Commands

func (m Model) loadEntriesCmd() tea.Cmd {
	return func() tea.Msg {
		entries, err := m.chezmoi.Status()
		if err != nil {
			return entriesErrMsg{err: err}
		}
		return entriesLoadedMsg{entries: entries}
	}
}

func (m Model) loadDiffCmd(entry model.Entry) tea.Cmd {
	return func() tea.Msg {
		// Read destination file
		dst, err := os.ReadFile(entry.TargetPath)
		if err != nil && !os.IsNotExist(err) {
			return diffErrMsg{targetPath: entry.TargetPath, err: fmt.Errorf("read destination: %w", err)}
		}
		// Get rendered source
		src, err := m.chezmoi.Cat(entry.TargetPath)
		if err != nil {
			return diffErrMsg{targetPath: entry.TargetPath, err: fmt.Errorf("chezmoi cat: %w", err)}
		}

		srcName := "source (rendered)"
		if entry.SourcePath != "" {
			srcName = entry.SourcePath
		}
		content := diff.Compute(srcName, src, entry.TargetPath, dst)
		return diffLoadedMsg{targetPath: entry.TargetPath, content: content}
	}
}

func (m Model) loadSourcePathCmd(index int, targetPath string) tea.Cmd {
	return func() tea.Msg {
		p, err := m.chezmoi.SourcePath(targetPath)
		if err != nil {
			return sourcePathMsg{index: index, path: ""}
		}
		return sourcePathMsg{index: index, path: p}
	}
}

func (m Model) applyCmd(entry model.Entry) tea.Cmd {
	return func() tea.Msg {
		if err := m.chezmoi.Apply(entry.TargetPath); err != nil {
			return applyErrMsg{targetPath: entry.TargetPath, err: err}
		}
		return applyDoneMsg{targetPath: entry.TargetPath}
	}
}

// Init

func (m Model) Init() tea.Cmd {
	return m.loadEntriesCmd()
}

// Update

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
		rightW := m.width - m.width/3 - 2 // -2 for borders
		diffH := contentH - 2             // -2 for borders
		if !m.ready {
			m.diffViewport = viewport.New(rightW, diffH)
			m.ready = true
		} else {
			m.diffViewport.Width = rightW
			m.diffViewport.Height = diffH
		}
		m.diffViewport.SetContent(colorizeDiff(m.diffContent))

	case entriesLoadedMsg:
		m.entries = msg.entries
		m.loadErr = nil
		m.cursor = 0
		// Load source paths and diff for initial selection
		var batchCmds []tea.Cmd
		for i, e := range m.entries {
			batchCmds = append(batchCmds, m.loadSourcePathCmd(i, e.TargetPath))
		}
		if entry := m.selectedEntry(); entry != nil {
			m.diffLoading = true
			batchCmds = append(batchCmds, m.loadDiffCmd(*entry))
		}
		cmds = append(cmds, batchCmds...)

	case entriesErrMsg:
		m.loadErr = msg.err
		m.entries = nil

	case sourcePathMsg:
		if msg.index >= 0 && msg.index < len(m.entries) {
			m.entries[msg.index].SourcePath = msg.path
		}

	case diffLoadedMsg:
		entry := m.selectedEntry()
		if entry != nil && entry.TargetPath == msg.targetPath {
			m.diffContent = msg.content
			m.diffLoading = false
			m.diffErr = nil
			m.diffViewport.SetContent(colorizeDiff(m.diffContent))
			m.diffViewport.GotoTop()
		}

	case diffErrMsg:
		entry := m.selectedEntry()
		if entry != nil && entry.TargetPath == msg.targetPath {
			m.diffContent = ""
			m.diffLoading = false
			m.diffErr = msg.err
			m.diffViewport.SetContent(errorStyle.Render(fmt.Sprintf("Error: %v", msg.err)))
		}

	case applyDoneMsg:
		m.state = stateNormal
		m.statusMsg = fmt.Sprintf("Applied: %s", msg.targetPath)
		cmds = append(cmds, m.loadEntriesCmd())

	case applyErrMsg:
		m.state = stateNormal
		m.statusMsg = fmt.Sprintf("Error applying: %v", msg.err)

	case tea.KeyMsg:
		switch m.state {
		case stateHelp:
			if msg.String() == "?" || msg.String() == "q" || msg.String() == "esc" {
				m.state = stateNormal
			}

		case stateConfirming:
			switch msg.String() {
			case "y", "Y":
				entry := m.selectedEntry()
				if entry != nil {
					m.state = stateApplying
					m.statusMsg = fmt.Sprintf("Applying %s...", entry.TargetPath)
					cmds = append(cmds, m.applyCmd(*entry))
				} else {
					m.state = stateNormal
				}
			case "n", "N", "esc":
				m.state = stateNormal
				m.statusMsg = "Cancelled"
			}

		case stateApplying:
			// ignore keys while applying

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
				if m.focusedPane == paneTarget {
					m.focusedPane = paneSrc
				} else {
					m.focusedPane = paneTarget
				}

			case "shift+tab":
				if m.focusedPane == paneSrc {
					m.focusedPane = paneTarget
				} else {
					m.focusedPane = paneSrc
				}

			case "j", "down":
				if m.cursor < len(m.entries)-1 {
					m.cursor++
					cmds = append(cmds, m.triggerDiffLoad())
				}

			case "k", "up":
				if m.cursor > 0 {
					m.cursor--
					cmds = append(cmds, m.triggerDiffLoad())
				}

			case "a":
				if m.focusedPane == paneTarget {
					entry := m.selectedEntry()
					if entry != nil && entry.CanApply() {
						m.state = stateConfirming
						m.statusMsg = ""
					}
				}

			case "e":
				entry := m.selectedEntry()
				if entry != nil {
					path := entry.SourcePath
					if path == "" {
						path = entry.TargetPath
					}
					cmds = append(cmds, openEditorCmd(path))
				}

			default:
				// Pass scroll keys to diff viewport
				var cmd tea.Cmd
				m.diffViewport, cmd = m.diffViewport.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m Model) triggerDiffLoad() tea.Cmd {
	entry := m.selectedEntry()
	if entry == nil {
		return nil
	}
	m.diffLoading = true
	m.diffContent = ""
	m.diffErr = nil
	return m.loadDiffCmd(*entry)
}

func openEditorCmd(path string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return applyErrMsg{err: fmt.Errorf("editor: %w", err)}
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
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			result.WriteString(diffAddStyle.Render(line))
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			result.WriteString(diffDelStyle.Render(line))
		} else {
			result.WriteString(line)
		}
		result.WriteString("\n")
	}
	return result.String()
}
