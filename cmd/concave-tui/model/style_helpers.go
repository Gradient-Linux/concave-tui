package model

import (
	"github.com/charmbracelet/lipgloss"
)

func successText(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(text)
}

func warnText(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarn)).Render(text)
}

func errorText(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Render(text)
}

func mutedText(text string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorMuted)).Render(text)
}
