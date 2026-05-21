package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/ryo246912/lazychezmoi/tools/lazychezmoi/internal/model"
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
		body = centeredModalBody(m.renderConfirmModal(), m.width, contentH)
	case stateCommandInput:
		body = centeredModalBody(m.renderCommandInputModal(), m.width, contentH)
	case stateError:
		body = centeredModalBody(m.renderErrorModal(), m.width, contentH)
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
	parts = append(parts, fmt.Sprintf("apply source: %s", m.applySourceMode))
	if m.filterQuery != "" || m.state == stateFilterInput {
		parts = append(parts, filterStyle.Render("filter: /"+m.filterInput))
	}

	if m.loadErr != nil {
		parts = append(parts, errorStyle.Render(fmt.Sprintf("Error: %v", m.loadErr)))
	} else {
		parts = append(parts, fmt.Sprintf("%d entries", len(m.entries)))
		parts = append(parts, fmt.Sprintf("%d rows", len(m.rows)))
		if queued := m.selectedTargetCount(); queued > 0 {
			parts = append(parts, fmt.Sprintf("%d queued", queued))
		}
	}
	if indicator := m.loadingIndicator(); indicator != "" {
		parts = append(parts, indicator)
	}

	return headerStyle.Width(m.width).Render(left + " | " + strings.Join(parts, " | "))
}

func (m Model) renderFooter() string {
	var lines []string

	switch m.state {
	case stateRunningAction:
		lines = append(lines, m.loadingIndicator())
	case stateConfirming:
		lines = append(lines, "y:confirm  n/esc:cancel")
	case stateCommandInput:
		lines = append(lines, "enter:run  down/up:history  esc:cancel")
	case stateError:
		lines = append(lines, "enter/esc/q:close")
	case stateFilterInput:
		lines = append(lines, filterStyle.Render("/"+m.filterInput))
		lines = append(lines, "enter:apply  esc:clear")
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
		return " j/k:move  enter:toggle dir  /:filter  h/l:focus src/target  tab:focus diff  e:edit source  ::command  m:mode  1/2/3:apply src  r:refresh  ?:help  q:quit"
	case paneDiff:
		return " j/k/pgup/pgdn/g/G:scroll diff  /:filter  tab:return to list  ::command  m:mode  1/2/3:apply src  r:refresh  ?:help  q:quit"
	default:
		if m.listMode == listModeManaged {
			return " j/k:move  enter:toggle dir  /:filter  h/l:focus src/target  tab:focus diff  space:queue  a:apply  i:add->src  e:edit target  ::command  m:mode  1/2/3:apply src  r:refresh  ?:help  q:quit"
		}
		return " j/k:move  enter:toggle dir  /:filter  h/l:focus src/target  tab:focus diff  space:queue  a:apply  i:add->src/track  d:delete unmanaged  e:edit  ::command  m:mode  1/2/3:apply src  r:refresh  ?:help  q:quit"
	}
}

func (m Model) renderModeHint() string {
	hint := " mode: managed = tracked entries with target-side diff; i copies target into source state"
	if m.listMode == listModeAll {
		hint = " mode: all = managed diffs + unmanaged paths; space/a:apply managed  i:add->src or track  d:delete unmanaged"
	}
	return truncateText(hint, max(1, m.width-2))
}

func (m Model) renderHelp() string {
	help := `lazychezmoi - chezmoi TUI

Modes:
  managed       Entries already tracked by chezmoi with target-side diffs
  all           Managed entries with diffs plus unmanaged (target-only) paths
  m             Toggle managed / all list mode
  1 / 2 / 3     Select apply source: working tree / staged / HEAD
  :             Open the custom shell command prompt for the selected entry

Keybindings:
  j / down      Move down in src/target or scroll diff
  k / up        Move up in src/target or scroll diff
  enter         Expand or collapse the selected directory row
  /             Start filter input for the file tree
  h / l         Focus src / target pane
  tab           Toggle diff focus
  space         Toggle current target in the apply queue (managed mode)
  a             Apply queued targets (or the current target) from the selected source mode
  i             In target pane, patch source from target (managed template),
                run chezmoi add to update source (managed non-template),
                or start tracking the selected path (unmanaged / all mode)
  d             Delete the current unmanaged target after confirmation
  e             Open the focused src/target file in $EDITOR
  click         Focus the clicked pane; row clicks select and focus that file
  wheel         Scroll the hovered src/target tree
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

Shell command prompt:
  enter         Run the current command immediately
  down / up     Select older / newer command history
  esc           Close the prompt

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
	for i := metrics.offset; i < len(m.rows) && i < metrics.offset+metrics.listHeight; i++ {
		row := m.rows[i]
		rows = append(rows, m.renderEntryRow(row, kind, i == m.cursor, focused, metrics.listWidth))
	}

	if len(m.rows) == 0 {
		if m.filterQuery != "" {
			rows = append(rows, "  (no matching rows)")
		} else {
			rows = append(rows, "  (no entries)")
		}
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

func (m Model) renderEntryRow(row listRow, kind paneKind, current, focused bool, maxWidth int) string {
	path := m.rowPathForPane(row, kind)
	prefix := ""

	if kind == paneTarget && row.hasEntry && row.entry.Kind == model.EntryManaged && row.entry.CanApply() {
		prefix = "[ ]"
		if m.isTargetSelected(row.entry.TargetPath) {
			prefix = "[x]"
		}
	}

	fixedWidth := lipgloss.Width(m.statusLabelForRow(row)) + 1
	if prefix != "" {
		fixedWidth += lipgloss.Width(prefix) + 1
	}
	path = truncateText(path, max(1, maxWidth-fixedWidth))

	badge := m.renderStatusBadge(row, current, focused)
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

func (m Model) rowPathForPane(row listRow, kind paneKind) string {
	indent := strings.Repeat("  ", row.depth)
	marker := "- "
	if row.directory {
		marker = "> "
		if row.expanded || m.filterQuery != "" {
			marker = "v "
		}
	}

	label := row.name
	switch kind {
	case paneSrc:
		label = m.rowSourceLabel(row)
	case paneTarget:
		if row.directory {
			label += string(filepath.Separator)
		}
	}
	if kind == paneSrc && row.directory {
		label += string(filepath.Separator)
	}

	return indent + marker + label
}

func (m Model) rowSourceLabel(row listRow) string {
	if row.directory {
		return row.name
	}
	if !row.hasEntry {
		return row.name
	}
	if row.entry.Kind == model.EntryUnmanaged {
		return "(missing) " + row.name
	}
	if row.entry.SourcePath == "" {
		return "(resolving) " + row.name
	}
	return filepath.Base(row.entry.SourcePath)
}

func (m Model) renderStatusBadge(row listRow, current, focused bool) string {
	style := statusDirStyle
	label := m.statusLabelForRow(row)

	if row.hasEntry {
		style = statusModStyle
		if row.entry.Kind == model.EntryUnmanaged {
			style = statusUnmanagedStyle
		} else {
			switch row.entry.TargetCode {
			case model.StatusAdded:
				style = statusAddedStyle
			case model.StatusModified:
				style = statusModStyle
			case model.StatusDeleted:
				style = statusDeletedStyle
			}
		}
	}

	switch {
	case current && focused:
		style = style.Background(colorFocus).Foreground(lipgloss.Color("255")).Bold(true)
	case current:
		style = style.Background(colorInactive).Foreground(lipgloss.Color("255"))
	}

	return style.Render(label)
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

func (m Model) statusLabelForRow(row listRow) string {
	if !row.hasEntry {
		return "DR"
	}
	return m.statusLabel(row.entry)
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
	case m.listMode == listModeAll:
		title = fmt.Sprintf(" diff preview (%s / unmanaged)", m.applySourceMode)
	}
	selected := m.selectedEntry()
	if selected != nil && m.pendingDiffPath == selected.TargetPath {
		title += " (refresh pending...)"
	} else if m.diffLoading && m.diffContent != "" {
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
	case m.applySourceMode.RequiresSnapshot() && m.snapshotLoading:
		content = fmt.Sprintf("%s Preparing %s snapshot...", m.spinner.View(), m.applySourceMode)
	case m.applySourceMode.RequiresSnapshot() && m.snapshotErr != nil:
		content = errorStyle.Render(fmt.Sprintf("Snapshot error: %v", m.snapshotErr))
	case m.diffLoading && m.diffContent == "":
		content = fmt.Sprintf("%s Loading diff...", m.spinner.View())
	case m.diffContent == "" && len(m.rows) == 0:
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

func overlayCenteredBody(body, modal string, width, height int) string {
	baseLines := strings.Split(body, "\n")
	if len(baseLines) < height {
		padding := make([]string, height-len(baseLines))
		baseLines = append(baseLines, padding...)
	}
	if len(baseLines) > height {
		baseLines = baseLines[:height]
	}

	modalLines := strings.Split(modal, "\n")
	modalWidth := 0
	for _, line := range modalLines {
		modalWidth = max(modalWidth, lipgloss.Width(line))
	}
	if modalWidth == 0 {
		return strings.Join(baseLines, "\n")
	}

	left := max(0, (width-modalWidth)/2)
	top := max(0, (height-len(modalLines))/2)
	for i, line := range modalLines {
		if top+i >= len(baseLines) {
			break
		}
		baseLines[top+i] = overlayLine(baseLines[top+i], line, width, left, modalWidth)
	}

	return strings.Join(baseLines, "\n")
}

func centeredModalBody(modal string, width, height int) string {
	if height <= 0 {
		return ""
	}

	blankLine := strings.Repeat(" ", max(0, width))
	lines := make([]string, height)
	for i := range lines {
		lines[i] = blankLine
	}

	return overlayCenteredBody(strings.Join(lines, "\n"), modal, width, height)
}

func overlayLine(base, overlay string, width, left, overlayWidth int) string {
	plain := padPlainLine(xansi.Strip(base), width)
	rightStart := min(width, left+overlayWidth)
	prefix := slicePlainLine(plain, 0, left)
	suffix := slicePlainLine(plain, rightStart, width)
	return prefix + overlay + suffix
}

func padPlainLine(line string, width int) string {
	if lipgloss.Width(line) >= width {
		return line
	}
	return line + strings.Repeat(" ", width-lipgloss.Width(line))
}

func slicePlainLine(line string, start, end int) string {
	if start < 0 {
		start = 0
	}
	if end < start {
		end = start
	}

	runes := []rune(line)
	if start > len(runes) {
		start = len(runes)
	}
	if end > len(runes) {
		end = len(runes)
	}
	return string(runes[start:end])
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
	case actionPatchSource:
		title = "Patch Template Source?"
		lines = append(lines,
			"Apply target diff as a patch to the template source file:",
			"",
			truncatePath(m.confirmAction.entry.SourcePath, 64),
		)
	case actionPatchSourceConfirm:
		title = "Conflicts Found - Apply Anyway?"
		lines = append(lines,
			"The patch could not be applied cleanly. Conflict markers will",
			"be written to the source file for manual resolution:",
			"",
			truncatePath(m.confirmAction.entry.SourcePath, 64),
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
	inputWidth := min(76, max(34, m.width-18))
	input := m.commandInput
	input.Width = max(16, inputWidth-4)

	historyLines := m.renderCommandHistoryLines(5)
	historyBody := "No command history yet."
	if len(historyLines) > 0 {
		historyBody = strings.Join(historyLines, "\n")
	}

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		modalTitleStyle.Render("Run Shell Command"),
		commandInputBoxStyle.Width(inputWidth).Render(input.View()),
		"",
		modalBodyStyle.Render("History"),
		commandHistoryBoxStyle.Width(inputWidth).Render(historyBody),
	)

	modalWidth := min(88, max(44, m.width-8))
	return modalStyle.Width(modalWidth).Render(body)
}

func (m Model) renderCommandHistoryLines(limit int) []string {
	if len(m.commandHistory) == 0 || limit <= 0 {
		return nil
	}

	start := 0
	if m.commandHistoryIndex >= limit {
		start = m.commandHistoryIndex - limit + 1
	}

	end := min(len(m.commandHistory), start+limit)
	lines := make([]string, 0, end-start)
	for idx := start; idx < end; idx++ {
		prefix := "  "
		style := commandHistoryStyle
		if idx == m.commandHistoryIndex {
			prefix = "> "
			style = commandHistorySelectedStyle
		}
		lines = append(lines, style.Render(prefix+truncateText(m.commandHistory[idx], 72)))
	}

	return lines
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

func (m Model) renderErrorModal() string {
	title := "Error"
	var lines []string

	msg := m.lastActionErr
	switch msg.action.kind {
	case actionApply:
		title = "Apply Failed"
	case actionAdd:
		title = "Add Failed"
	case actionDelete:
		title = "Delete Failed"
	case actionShell:
		title = "Shell Command Failed"
	case actionPatchSource, actionPatchSourceConfirm:
		title = "Patch Failed"
	}

	if msg.failedTarget != "" {
		lines = append(lines, "Target:", truncatePath(msg.failedTarget, 64), "")
	}

	errMsg := msg.err.Error()
	// Split long error messages into multiple lines
	maxLineLen := 64
	for len(errMsg) > maxLineLen {
		lines = append(lines, errMsg[:maxLineLen])
		errMsg = errMsg[maxLineLen:]
	}
	lines = append(lines, errMsg)

	lines = append(lines, "", "enter / esc / q: close")

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		modalTitleStyle.Foreground(colorRed).Render(title),
		modalBodyStyle.Render(strings.Join(lines, "\n")),
	)

	modalWidth := min(80, max(36, m.width-8))
	return modalStyle.BorderForeground(colorRed).Width(modalWidth).Render(body)
}
