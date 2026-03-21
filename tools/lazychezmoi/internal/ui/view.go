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
	switch m.state {
	case stateConfirming:
		body = lipgloss.Place(m.width, contentH, lipgloss.Center, lipgloss.Center, m.renderConfirmModal())
	case stateCommandInput:
		body = lipgloss.Place(m.width, contentH, lipgloss.Center, lipgloss.Center, m.renderCommandInputModal())
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

	parts := []string{m.listMode.HeaderLabel()}
	if m.listMode == listModeManaged {
		parts = append(parts, fmt.Sprintf("apply source: %s", m.applySourceMode))
	}

	if m.loadErr != nil {
		parts = append(parts, errorStyle.Render(fmt.Sprintf("Error: %v", m.loadErr)))
	} else {
		parts = append(parts, fmt.Sprintf("%d entries", len(m.entries)))
		if queued := m.selectedTargetCount(); queued > 0 {
			parts = append(parts, fmt.Sprintf("%d queued", queued))
		}
	}

	return headerStyle.Width(m.width).Render(left + " | " + strings.Join(parts, " | "))
}

func (m Model) renderFooter() string {
	var lines []string

	switch m.state {
	case stateRunningAction:
		lines = append(lines, m.statusMsg)
	case stateConfirming:
		lines = append(lines, "y:confirm  n/esc:cancel")
	case stateCommandInput:
		lines = append(lines, "enter:confirm  esc:cancel")
	default:
		if m.statusMsg != "" {
			lines = append(lines, m.statusMsg)
		}
		lines = append(lines, m.renderKeyHints())
		lines = append(lines, m.renderModeHint())
	}

	return footerStyle.Width(m.width).Render(strings.Join(lines, "\n"))
}

func (m Model) renderKeyHints() string {
	switch m.focusedPane {
	case paneSrc:
		if m.listMode == listModeManaged {
			return " j/k:move  h/l:focus src/target  tab:focus diff  e:edit source  !:command  m:mode  1/2/3:apply src  r:refresh  ?:help  q:quit"
		}
		return " j/k:move  h/l:focus src/target  tab:focus diff  !:command  m:mode  r:refresh  ?:help  q:quit"
	case paneDiff:
		if m.listMode == listModeManaged {
			return " j/k/pgup/pgdn/g/G:scroll diff  tab:return to list  !:command  m:mode  1/2/3:apply src  r:refresh  ?:help  q:quit"
		}
		return " j/k/pgup/pgdn/g/G:scroll diff  tab:return to list  !:command  m:mode  r:refresh  ?:help  q:quit"
	default:
		if m.listMode == listModeManaged {
			return " j/k:move  h/l:focus src/target  tab:focus diff  space:queue  a:apply  i:add->src  e:edit target  !:command  m:mode  1/2/3:apply src  r:refresh  ?:help  q:quit"
		}
		return " j/k:move  h/l:focus src/target  tab:focus diff  i:add  d:delete  e:edit target  !:command  m:mode  r:refresh  ?:help  q:quit"
	}
}

func (m Model) renderModeHint() string {
	hint := " mode: managed = tracked entries with target-side diff; target pane i copies the current target into source state"
	if m.listMode == listModeUnmanaged {
		hint = " mode: unmanaged = target-only paths not yet tracked by chezmoi; target pane i runs chezmoi add"
	}
	return truncateText(hint, max(1, m.width-2))
}

func (m Model) renderHelp() string {
	help := `lazychezmoi - chezmoi TUI

Modes:
  managed       Entries already tracked by chezmoi with target-side diffs
  unmanaged     Target-only paths that are not yet tracked by chezmoi
  m             Toggle managed / unmanaged list mode
  1 / 2 / 3     Select apply source: working tree / staged / HEAD
  !             Enter a custom shell command for the selected entry

Keybindings:
  j / down      Move down in src/target or scroll diff
  k / up        Move up in src/target or scroll diff
  h / l         Focus src / target pane
  tab           Toggle diff focus
  space         Toggle current target in the apply queue (managed mode)
  a             Apply queued targets (or the current target) from the selected source mode
  i             In target pane, run chezmoi add to update source from target (managed)
                or start tracking the selected target path (unmanaged)
  d             Delete the current unmanaged target after confirmation
  e             Open the focused src/target file in $EDITOR
  click         Focus the clicked pane; src/target row clicks also select it
  r             Refresh file list, snapshots, and diff cache
  ?             Toggle this help
  q / ctrl+c    Quit

Shell command context:
  LAZYCHEZMOI_TARGET_PATH
  LAZYCHEZMOI_SOURCE_PATH
  LAZYCHEZMOI_ENTRY_MODE
  LAZYCHEZMOI_TARGET_KIND
  LAZYCHEZMOI_APPLY_SOURCE
  LAZYCHEZMOI_LIST_MODE

Diff pane scrolling:
  pgdn/pgup     Page down / page up
  g / G         Top / bottom of diff
`
	return help
}

func (m Model) renderListPane(kind paneKind, width, height int) string {
	focused := m.focusedPane == kind
	metrics := m.listPaneMetrics(kind, paneRect{Width: width, Height: height})

	titleBar := inactiveTitleStyle.Render(metrics.title)
	if focused {
		titleBar = titleStyle.Render(metrics.title)
	}

	var rows []string
	for i := metrics.offset; i < len(m.entries) && i < metrics.offset+metrics.listHeight; i++ {
		entry := m.entries[i]
		rows = append(rows, m.renderEntryRow(entry, kind, i == m.cursor, focused, metrics.listWidth))
	}

	if len(m.entries) == 0 {
		rows = append(rows, "  (no entries)")
	}
	for len(rows) < metrics.listHeight {
		rows = append(rows, "")
	}

	content := strings.Join(rows, "\n")
	borderStyle := unfocusedBorderStyle.Width(metrics.listWidth).Height(metrics.listHeight)
	if focused {
		borderStyle = focusedBorderStyle.Width(metrics.listWidth).Height(metrics.listHeight)
	}

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, borderStyle.Render(content))
}

func (m Model) renderEntryRow(entry model.Entry, kind paneKind, current, focused bool, maxWidth int) string {
	path := m.entryPathForPane(entry, kind)
	prefix := ""

	if kind == paneTarget && m.listMode == listModeManaged {
		prefix = "[ ]"
		if m.isTargetSelected(entry.TargetPath) {
			prefix = "[x]"
		}
	}

	fixedWidth := lipgloss.Width(m.statusLabel(entry)) + 1
	if prefix != "" {
		fixedWidth += lipgloss.Width(prefix) + 1
	}
	path = truncatePath(path, max(1, maxWidth-fixedWidth))

	badge := m.renderStatusBadge(entry, current, focused)
	switch {
	case current && focused:
		return renderSelectedRow(prefix, badge, path, selectedRowStyle, maxWidth)
	case current:
		return renderSelectedRow(prefix, badge, path, inactiveSelectedRowStyle, maxWidth)
	default:
		label := badge + " " + path
		if prefix != "" {
			label = prefix + " " + label
		}
		return lipgloss.NewStyle().Width(maxWidth).Render(label)
	}
}

func (m Model) entryPathForPane(entry model.Entry, kind paneKind) string {
	switch kind {
	case paneSrc:
		if entry.Kind == model.EntryUnmanaged {
			return "(missing) " + entry.TargetPath
		}
		if entry.SourcePath != "" {
			return entry.SourcePath
		}
		return "(resolving) " + entry.TargetPath
	default:
		return entry.TargetPath
	}
}

func (m Model) renderStatusBadge(entry model.Entry, current, focused bool) string {
	style := statusModStyle
	if entry.Kind == model.EntryUnmanaged {
		style = statusUnmanagedStyle
	} else {
		switch entry.TargetCode {
		case model.StatusAdded:
			style = statusAddedStyle
		case model.StatusModified:
			style = statusModStyle
		case model.StatusDeleted:
			style = statusDeletedStyle
		}
	}

	switch {
	case current && focused:
		style = style.Background(colorFocus).Foreground(lipgloss.Color("255")).Bold(true)
	case current:
		style = style.Background(colorInactive).Foreground(lipgloss.Color("255"))
	}

	return style.Render(m.statusLabel(entry))
}

func (m Model) statusLabel(entry model.Entry) string {
	if entry.Kind == model.EntryUnmanaged {
		switch entry.TargetType {
		case model.TargetDirectory:
			return "UD"
		case model.TargetSymlink:
			return "UL"
		default:
			return "UM"
		}
	}
	return string([]byte{byte(entry.SourceCode), byte(entry.TargetCode)})
}

func renderSelectedRow(prefix, badge, path string, rowStyle lipgloss.Style, maxWidth int) string {
	var rendered strings.Builder

	if prefix != "" {
		rendered.WriteString(rowStyle.Render(prefix))
		rendered.WriteString(rowStyle.Render(" "))
	}
	rendered.WriteString(badge)
	rendered.WriteString(rowStyle.Render(" "))
	rendered.WriteString(rowStyle.Render(path))

	content := rendered.String()
	if padding := maxWidth - lipgloss.Width(content); padding > 0 {
		content += rowStyle.Render(strings.Repeat(" ", padding))
	}

	return content
}

func (m Model) renderDiffPane(width, height int) string {
	title := " diff preview"
	switch {
	case m.listMode == listModeManaged:
		title = fmt.Sprintf(" diff preview (%s)", m.applySourceMode)
	case m.listMode == listModeUnmanaged:
		title = " diff preview (unmanaged target)"
	}
	if m.diffLoading && m.diffContent != "" {
		title += " (refreshing...)"
	} else if m.diffLoading {
		title += " (loading...)"
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
	case m.listMode == listModeManaged && m.applySourceMode.RequiresSnapshot() && m.snapshotLoading:
		content = fmt.Sprintf("Preparing %s snapshot...", m.applySourceMode)
	case m.listMode == listModeManaged && m.applySourceMode.RequiresSnapshot() && m.snapshotErr != nil:
		content = errorStyle.Render(fmt.Sprintf("Snapshot error: %v", m.snapshotErr))
	case m.diffLoading && m.diffContent == "":
		content = "Loading diff..."
	case m.diffContent == "" && len(m.entries) == 0:
		content = "(no entries)"
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
	var (
		title string
		lines []string
	)

	switch m.confirmAction.kind {
	case actionApply:
		count := len(m.confirmAction.targets)
		title = fmt.Sprintf("Apply %d file?", count)
		if count != 1 {
			title = fmt.Sprintf("Apply %d files?", count)
		}
		lines = append(lines, fmt.Sprintf("This will run `chezmoi apply` from %s for:", m.applySourceMode), "")
		for i, targetPath := range m.confirmAction.targets {
			if i >= 5 {
				lines = append(lines, fmt.Sprintf("...and %d more", len(m.confirmAction.targets)-i))
				break
			}
			lines = append(lines, truncatePath(targetPath, 64))
		}
	case actionAdd:
		if m.confirmAction.entry.Kind == model.EntryManaged {
			title = "Copy Current Target Into Source?"
			lines = append(lines,
				"This will run `chezmoi add` and copy the current target-side",
				"content back into chezmoi source state for:",
				"",
				truncatePath(m.confirmAction.entry.TargetPath, 64),
			)
		} else {
			title = "Track Target In Source?"
			lines = append(lines,
				"This will run `chezmoi add` and start tracking:",
				"",
				truncatePath(m.confirmAction.entry.TargetPath, 64),
			)
		}
	case actionDelete:
		title = "Delete Target?"
		lines = append(lines,
			"This will delete the unmanaged target:",
			"",
			truncatePath(m.confirmAction.entry.TargetPath, 64),
		)
	case actionShell:
		title = "Run Shell Command?"
		lines = append(lines,
			"The selected entry context will be exported as environment variables.",
			"",
			truncatePath(m.confirmAction.entry.TargetPath, 64),
			"",
			commandStyle.Render(m.confirmAction.command),
		)
	default:
		title = "Confirm?"
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

func (m Model) renderCommandInputModal() string {
	entry := m.selectedEntry()
	targetPath := "(no entry selected)"
	if entry != nil {
		targetPath = entry.TargetPath
	}

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		modalTitleStyle.Render("Run Shell Command"),
		modalBodyStyle.Render("Selected target: "+truncatePath(targetPath, 64)),
		"",
		commandInputStyle.Width(min(80, max(36, m.width-12))).Render("! "+m.commandInput),
		"",
		modalBodyStyle.Render("The command runs in your shell and receives LAZYCHEZMOI_* environment variables."),
		modalBodyStyle.Render("enter: confirm    esc: cancel"),
	)

	modalWidth := min(88, max(44, m.width-8))
	return modalStyle.Width(modalWidth).Render(body)
}

func truncatePath(path string, maxWidth int) string {
	if maxWidth <= 3 || len(path) <= maxWidth {
		return path
	}
	return "..." + path[len(path)-(maxWidth-3):]
}

func truncateText(text string, maxWidth int) string {
	if maxWidth <= 3 || len(text) <= maxWidth {
		return text
	}
	return text[:maxWidth-3] + "..."
}
