package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorFocus    = lipgloss.Color("62")
	colorInactive = lipgloss.Color("240")
	colorGreen    = lipgloss.Color("42")
	colorRed      = lipgloss.Color("196")
	colorYellow   = lipgloss.Color("220")
	colorCyan     = lipgloss.Color("81")
	colorBlue     = lipgloss.Color("75")
	colorModalBg  = lipgloss.Color("236")

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(colorFocus).
			Padding(0, 1)

	inactiveTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("255")).
				Background(colorInactive).
				Padding(0, 1)

	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorFocus)

	unfocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorInactive)

	selectedRowStyle = lipgloss.NewStyle().
				Background(colorFocus).
				Foreground(lipgloss.Color("255")).
				Bold(true)

	inactiveSelectedRowStyle = lipgloss.NewStyle().
					Background(colorInactive).
					Foreground(lipgloss.Color("255"))

	statusAddedStyle     = lipgloss.NewStyle().Foreground(colorGreen)
	statusModStyle       = lipgloss.NewStyle().Foreground(colorYellow)
	statusDeletedStyle   = lipgloss.NewStyle().Foreground(colorRed)
	statusDirStyle       = lipgloss.NewStyle().Foreground(colorBlue).Bold(true)
	statusUnmanagedStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Bold(true)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255")).
			Background(lipgloss.Color("237")).
			Width(0).
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Foreground(colorInactive).
			Padding(0, 1)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	diffAddStyle = lipgloss.NewStyle().Foreground(colorGreen)
	diffDelStyle = lipgloss.NewStyle().Foreground(colorRed)

	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(colorFocus).
			Background(colorModalBg).
			Padding(1, 2)

	modalTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("255"))

	modalBodyStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	commandStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	commandInputBoxStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")).
				Background(lipgloss.Color("235")).
				Padding(0, 1)

	commandInputPromptStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Bold(true)

	commandInputTextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255"))

	commandInputPlaceholderStyle = lipgloss.NewStyle().
					Foreground(colorInactive)

	commandInputCursorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")).
				Background(colorCyan)

	commandHistoryBoxStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Background(lipgloss.Color("235")).
				Padding(0, 1)

	commandHistoryStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	commandHistorySelectedStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("255")).
					Background(colorFocus)

	filterStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	spinnerStyle = lipgloss.NewStyle().Foreground(colorCyan)
)
