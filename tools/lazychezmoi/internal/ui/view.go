package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"lazychezmoi/internal/model"
)

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.state == stateHelp {
		return m.renderHelp()
	}

	header := m.renderHeader()
	footer := m.renderFooter()

	headerH := lipgloss.Height(header)
	footerH := lipgloss.Height(footer)
	contentH := m.height - headerH - footerH
	if contentH < 2 {
		contentH = 2
	}

	leftW := m.width / 3
	rightW := m.width - leftW

	halfH := contentH / 2
	srcH := halfH
	targetH := contentH - halfH

	srcPane := m.renderListPane(paneSrc, leftW, srcH)
	targetPane := m.renderListPane(paneTarget, leftW, targetH)
	leftCol := lipgloss.JoinVertical(lipgloss.Left, srcPane, targetPane)

	diffPane := m.renderDiffPane(rightW, contentH)

	main := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, diffPane)

	return lipgloss.JoinVertical(lipgloss.Left, header, main, footer)
}

func (m Model) renderHeader() string {
	left := " lazychezmoi"
	var middle string
	if m.loadErr != nil {
		middle = errorStyle.Render(fmt.Sprintf(" Error: %v", m.loadErr))
	} else {
		middle = fmt.Sprintf(" %d changed files", len(m.entries))
	}
	content := left + middle
	return headerStyle.Width(m.width).Render(content)
}

func (m Model) renderFooter() string {
	var lines []string

	switch m.state {
	case stateConfirming:
		entry := m.selectedEntry()
		if entry != nil {
			lines = append(lines, fmt.Sprintf("Apply %s? [y/N]", entry.TargetPath))
		}
	case stateApplying:
		lines = append(lines, m.statusMsg)
	case stateHelp:
		// handled by renderHelp
	default:
		if m.statusMsg != "" {
			lines = append(lines, m.statusMsg)
		}
		keys := " j/k:move  tab:switch pane  a:apply  e:edit  r:refresh  ?:help  q:quit"
		if m.focusedPane == paneSrc {
			keys = " j/k:move  tab:switch pane  e:edit source  r:refresh  ?:help  q:quit"
		}
		lines = append(lines, keys)
	}

	return footerStyle.Width(m.width).Render(strings.Join(lines, "\n"))
}

func (m Model) renderHelp() string {
	help := `lazychezmoi - chezmoi TUI

Keybindings:
  j / ↓        Move down
  k / ↑        Move up
  tab          Switch pane (src ↔ target)
  shift+tab    Switch pane (reverse)
  a            Apply selected file (target pane only)
  e            Open source file in $EDITOR
  r            Refresh file list
  ?            Toggle this help
  q / ctrl+c   Quit

Diff pane scrolling:
  pgdn/pgup    Scroll diff
  g / G        Top / bottom of diff
`
	return help
}

func (m Model) renderListPane(kind paneKind, width, height int) string {
	focused := m.focusedPane == kind

	var title string
	switch kind {
	case paneSrc:
		title = "src"
	case paneTarget:
		title = "target (apply queue)"
	}

	// Title bar
	var titleBar string
	if focused {
		titleBar = titleStyle.Render(fmt.Sprintf(" %s (%d)", title, len(m.entries)))
	} else {
		titleBar = inactiveTitleStyle.Render(fmt.Sprintf(" %s (%d)", title, len(m.entries)))
	}
	titleH := lipgloss.Height(titleBar)

	// Inner area for list items (account for border: 2 lines vertical)
	listH := height - titleH - 2
	if listH < 1 {
		listH = 1
	}
	listW := width - 2 // account for border: 2 chars horizontal

	// Compute scroll offset to keep cursor visible
	offset := 0
	if m.cursor >= listH {
		offset = m.cursor - listH + 1
	}

	var rows []string
	for i := offset; i < len(m.entries) && i < offset+listH; i++ {
		entry := m.entries[i]
		row := m.renderEntryRow(entry, i, kind, i == m.cursor, focused, listW)
		rows = append(rows, row)
	}

	if len(m.entries) == 0 {
		rows = append(rows, "  (no changed files)")
	}

	// Pad to listH
	for len(rows) < listH {
		rows = append(rows, "")
	}

	content := strings.Join(rows, "\n")

	var borderStyle lipgloss.Style
	if focused {
		borderStyle = focusedBorderStyle.Width(listW).Height(listH)
	} else {
		borderStyle = unfocusedBorderStyle.Width(listW).Height(listH)
	}

	body := borderStyle.Render(content)
	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body)
}

func (m Model) renderEntryRow(entry model.Entry, index int, kind paneKind, selected, focused bool, maxWidth int) string {
	var path string
	switch kind {
	case paneSrc:
		path = entry.SourcePath
		if path == "" {
			path = entry.TargetPath
		}
	case paneTarget:
		path = entry.TargetPath
	}

	label := m.renderStatusBadge(entry) + " " + truncatePath(path, maxWidth-8)

	if selected && focused {
		return selectedRowStyle.Width(maxWidth).Render(label)
	} else if selected {
		return inactiveSelectedRowStyle.Width(maxWidth).Render(label)
	}
	return lipgloss.NewStyle().Width(maxWidth).Render(label)
}

func (m Model) renderStatusBadge(entry model.Entry) string {
	label := string([]byte{byte(entry.SourceCode), byte(entry.TargetCode)})
	switch entry.TargetCode {
	case model.StatusAdded:
		return statusAddedStyle.Render(label)
	case model.StatusModified:
		return statusModStyle.Render(label)
	case model.StatusDeleted:
		return statusDeletedStyle.Render(label)
	default:
		return statusModStyle.Render(label)
	}
}

func (m Model) renderDiffPane(width, height int) string {
	title := " diff preview"
	if m.diffLoading {
		title = " diff preview (loading...)"
	}
	titleBar := inactiveTitleStyle.Render(title)
	titleH := lipgloss.Height(titleBar)

	innerH := height - titleH - 2
	innerW := width - 2

	m.diffViewport.Width = innerW
	m.diffViewport.Height = innerH

	var content string
	if m.diffErr != nil {
		content = errorStyle.Render(fmt.Sprintf("Error: %v", m.diffErr))
	} else if m.diffLoading {
		content = "Loading diff..."
	} else if m.diffContent == "" && len(m.entries) == 0 {
		content = "(no files changed)"
	} else {
		content = m.diffViewport.View()
	}

	body := unfocusedBorderStyle.Width(innerW).Height(innerH).Render(content)
	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body)
}

func truncatePath(path string, maxWidth int) string {
	if maxWidth <= 3 || len(path) <= maxWidth {
		return path
	}
	return "..." + path[len(path)-(maxWidth-3):]
}
