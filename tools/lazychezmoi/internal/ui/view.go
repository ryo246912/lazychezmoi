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

	body := m.renderMain(contentH)
	if m.state == stateConfirming {
		body = lipgloss.Place(m.width, contentH, lipgloss.Center, lipgloss.Center, m.renderConfirmModal())
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m Model) renderMain(contentH int) string {
	leftW := m.width / 3
	rightW := m.width - leftW

	halfH := contentH / 2
	srcH := halfH
	targetH := contentH - halfH

	srcPane := m.renderListPane(paneSrc, leftW, srcH)
	targetPane := m.renderListPane(paneTarget, leftW, targetH)
	leftCol := lipgloss.JoinVertical(lipgloss.Left, srcPane, targetPane)

	diffPane := m.renderDiffPane(rightW, contentH)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, diffPane)
}

func (m Model) renderHeader() string {
	left := " lazychezmoi"

	var middle string
	if m.loadErr != nil {
		middle = errorStyle.Render(fmt.Sprintf(" Error: %v", m.loadErr))
	} else {
		middle = fmt.Sprintf(" %d changed files", len(m.entries))
		if queued := m.selectedTargetCount(); queued > 0 {
			middle += fmt.Sprintf(" | %d queued", queued)
		}
	}

	return headerStyle.Width(m.width).Render(left + middle)
}

func (m Model) renderFooter() string {
	var lines []string

	switch m.state {
	case stateApplying:
		lines = append(lines, m.statusMsg)
	case stateConfirming:
		lines = append(lines, "y:confirm  n/esc:cancel")
	default:
		if m.statusMsg != "" {
			lines = append(lines, m.statusMsg)
		}
		lines = append(lines, m.renderKeyHints())
	}

	return footerStyle.Width(m.width).Render(strings.Join(lines, "\n"))
}

func (m Model) renderKeyHints() string {
	switch m.focusedPane {
	case paneSrc:
		return " j/k:move  tab:switch pane  e:edit source  r:refresh  ?:help  q:quit"
	case paneDiff:
		return " j/k/pgup/pgdn/g/G:scroll diff  tab:switch pane  r:refresh  ?:help  q:quit"
	default:
		return " j/k:move  space:queue  a:apply  e:edit target  tab:switch pane  r:refresh  ?:help  q:quit"
	}
}

func (m Model) renderHelp() string {
	help := `lazychezmoi - chezmoi TUI

Keybindings:
  j / down      Move down in src/target or scroll diff
  k / up        Move up in src/target or scroll diff
  tab           Switch pane (target -> src -> diff)
  shift+tab     Switch pane (reverse)
  space         Toggle current target in the apply queue
  a             Apply queued targets (or the current target)
  e             Open the focused src/target file in $EDITOR
  r             Refresh file list and diff cache
  ?             Toggle this help
  q / ctrl+c    Quit

Diff pane scrolling:
  pgdn/pgup     Page down / page up
  g / G         Top / bottom of diff
`
	return help
}

func (m Model) renderListPane(kind paneKind, width, height int) string {
	focused := m.focusedPane == kind

	title := "src"
	if kind == paneTarget {
		title = "target (apply queue)"
	}

	titleText := fmt.Sprintf(" %s (%d)", title, len(m.entries))
	if kind == paneTarget && m.selectedTargetCount() > 0 {
		titleText = fmt.Sprintf(" %s (%d queued)", title, m.selectedTargetCount())
	}

	titleBar := inactiveTitleStyle.Render(titleText)
	if focused {
		titleBar = titleStyle.Render(titleText)
	}

	titleH := lipgloss.Height(titleBar)
	listH := height - titleH - 2
	if listH < 1 {
		listH = 1
	}
	listW := width - 2

	offset := 0
	if m.cursor >= listH {
		offset = m.cursor - listH + 1
	}

	var rows []string
	for i := offset; i < len(m.entries) && i < offset+listH; i++ {
		entry := m.entries[i]
		rows = append(rows, m.renderEntryRow(entry, kind, i == m.cursor, focused, listW))
	}

	if len(m.entries) == 0 {
		rows = append(rows, "  (no changed files)")
	}
	for len(rows) < listH {
		rows = append(rows, "")
	}

	content := strings.Join(rows, "\n")
	borderStyle := unfocusedBorderStyle.Width(listW).Height(listH)
	if focused {
		borderStyle = focusedBorderStyle.Width(listW).Height(listH)
	}

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, borderStyle.Render(content))
}

func (m Model) renderEntryRow(entry model.Entry, kind paneKind, current, focused bool, maxWidth int) string {
	path := entry.TargetPath
	switch kind {
	case paneSrc:
		if entry.SourcePath != "" {
			path = entry.SourcePath
		}
	case paneTarget:
		path = entry.TargetPath
	}

	label := m.renderStatusBadge(entry)
	if kind == paneTarget {
		selection := "[ ]"
		if m.isTargetSelected(entry.TargetPath) {
			selection = "[x]"
		}
		label = selection + " " + label + " " + truncatePath(path, max(1, maxWidth-12))
	} else {
		label = label + " " + truncatePath(path, max(1, maxWidth-8))
	}

	switch {
	case current && focused:
		return selectedRowStyle.Width(maxWidth).Render(label)
	case current:
		return inactiveSelectedRowStyle.Width(maxWidth).Render(label)
	default:
		return lipgloss.NewStyle().Width(maxWidth).Render(label)
	}
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
	switch {
	case m.diffLoading && m.diffContent != "":
		title = " diff preview (refreshing...)"
	case m.diffLoading:
		title = " diff preview (loading...)"
	}

	titleBar := inactiveTitleStyle.Render(title)
	if m.focusedPane == paneDiff {
		titleBar = titleStyle.Render(title)
	}

	titleH := lipgloss.Height(titleBar)
	innerH := height - titleH - 2
	if innerH < 1 {
		innerH = 1
	}
	innerW := width - 2
	if innerW < 1 {
		innerW = 1
	}

	m.diffViewport.Width = innerW
	m.diffViewport.Height = innerH

	var content string
	switch {
	case m.diffErr != nil:
		content = errorStyle.Render(fmt.Sprintf("Error: %v", m.diffErr))
	case m.diffLoading && m.diffContent == "":
		content = "Loading diff..."
	case m.diffContent == "" && len(m.entries) == 0:
		content = "(no files changed)"
	case m.diffContent == "":
		content = "Waiting for diff..."
	default:
		content = m.diffViewport.View()
	}

	borderStyle := unfocusedBorderStyle.Width(innerW).Height(innerH)
	if m.focusedPane == paneDiff {
		borderStyle = focusedBorderStyle.Width(innerW).Height(innerH)
	}

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, borderStyle.Render(content))
}

func (m Model) renderConfirmModal() string {
	count := len(m.confirmTargets)
	title := fmt.Sprintf("Apply %d file?", count)
	if count != 1 {
		title = fmt.Sprintf("Apply %d files?", count)
	}

	lines := []string{
		"This will run `chezmoi apply` for:",
		"",
	}
	for i, targetPath := range m.confirmTargets {
		if i >= 5 {
			lines = append(lines, fmt.Sprintf("...and %d more", len(m.confirmTargets)-i))
			break
		}
		lines = append(lines, truncatePath(targetPath, 64))
	}
	lines = append(lines, "", "y: confirm    n / esc: cancel")

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		modalTitleStyle.Render(title),
		modalBodyStyle.Render(strings.Join(lines, "\n")),
	)

	modalWidth := min(80, max(36, m.width-8))
	return modalStyle.Width(modalWidth).Render(body)
}

func truncatePath(path string, maxWidth int) string {
	if maxWidth <= 3 || len(path) <= maxWidth {
		return path
	}
	return "..." + path[len(path)-(maxWidth-3):]
}
