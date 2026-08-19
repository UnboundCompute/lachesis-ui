package ui

import "github.com/charmbracelet/lipgloss"

// The palette mirrors the design canvas (One-Dark family). Colors are the same
// hexes used in the .dc.html mockups so the shipped UI matches the design.
var (
	colDim     = lipgloss.Color("#6b7488")
	colFg      = lipgloss.Color("#c4cadb")
	colBright  = lipgloss.Color("#eef1f7")
	colCyan    = lipgloss.Color("#56b6c2")
	colAmber   = lipgloss.Color("#d19a66")
	colRed     = lipgloss.Color("#e06c75")
	colGreen   = lipgloss.Color("#98c379")
	colMag     = lipgloss.Color("#c678dd")
	colBlue    = lipgloss.Color("#61afef")
	colFainter = lipgloss.Color("#4f5768")

	colBorder = lipgloss.Color("#232838")
	colRule   = lipgloss.Color("#1b2030")
	colApp    = lipgloss.Color("#0b0d13")
	colPanel  = lipgloss.Color("#0e1119")
	colSelBg  = lipgloss.Color("#141a26")
	colFocus  = lipgloss.Color("#0a0c11")
)

var (
	stDim     = lipgloss.NewStyle().Foreground(colDim)
	stFg      = lipgloss.NewStyle().Foreground(colFg)
	stBright  = lipgloss.NewStyle().Foreground(colBright).Bold(true)
	stCyan    = lipgloss.NewStyle().Foreground(colCyan)
	stCyanB   = lipgloss.NewStyle().Foreground(colCyan).Bold(true)
	stAmber   = lipgloss.NewStyle().Foreground(colAmber)
	stRed     = lipgloss.NewStyle().Foreground(colRed)
	stGreen   = lipgloss.NewStyle().Foreground(colGreen)
	stGreenB  = lipgloss.NewStyle().Foreground(colGreen).Bold(true)
	stMag     = lipgloss.NewStyle().Foreground(colMag)
	stBlue    = lipgloss.NewStyle().Foreground(colBlue)
	stFainter = lipgloss.NewStyle().Foreground(colFainter)

	// Mode chip: green NAVIGATE badge as in the mockups.
	stModeChip = lipgloss.NewStyle().
			Foreground(colGreen).Bold(true).
			Background(lipgloss.Color("#12211a")).
			Padding(0, 1)

	// Column headers and section labels.
	stColHead = lipgloss.NewStyle().Foreground(colFainter)

	// Selected row: left cyan rule + subtle fill.
	stSelected = lipgloss.NewStyle().
			Foreground(colBright).
			Background(colSelBg).
			Bold(true)

	// The bordered focus/peek panel.
	stPanel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colRule).
		Padding(0, 1)

	stApp = lipgloss.NewStyle().
		Foreground(colFg).
		Background(colApp)

	// Status bar.
	stStatusBar = lipgloss.NewStyle().
			Background(colSelBg).
			Foreground(colDim)
	stStatusKey  = lipgloss.NewStyle().Foreground(colCyan)
	stStatusMode = lipgloss.NewStyle().Foreground(colGreen).Bold(true)
	stErr        = lipgloss.NewStyle().Foreground(colRed)
)

// selRule prefixes a selected row with a cyan bar, unselected with spaces, so
// columns stay aligned regardless of selection.
func selRule(selected bool) string {
	if selected {
		return stCyanB.Render("▌ ")
	}
	return "  "
}
