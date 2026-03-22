package model

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	tuiauth "github.com/Gradient-Linux/concave-tui/internal/auth"
	apiclient "github.com/Gradient-Linux/concave-tui/internal/client"
)

type loginSuccessMsg struct {
	session tuiauth.Session
}

type loginFailureMsg struct {
	err error
}

type LoginModel struct {
	width         int
	height        int
	usernameInput textinput.Model
	passwordInput textinput.Model
	focus         int
	loading       bool
	errorText     string
	attempts      int
}

func NewLoginModel() LoginModel {
	username := textinput.New()
	username.Prompt = ""
	username.Placeholder = "Username"
	username.CharLimit = 64

	password := textinput.New()
	password.Prompt = ""
	password.Placeholder = "Password"
	password.CharLimit = 128
	password.EchoMode = textinput.EchoPassword
	password.EchoCharacter = '•'

	return LoginModel{
		usernameInput: username,
		passwordInput: password,
	}
}

func (m *LoginModel) Activate() tea.Cmd {
	m.focus = 0
	m.usernameInput.Focus()
	m.passwordInput.Blur()
	return textinput.Blink
}

func (m *LoginModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m LoginModel) Update(msg tea.Msg) (LoginModel, tea.Cmd) {
	switch msg := msg.(type) {
	case loginFailureMsg:
		m.loading = false
		m.attempts++
		if err := msg.err; err != nil {
			m.errorText = err.Error()
			var apiErr *apiclient.APIError
			if errors.As(err, &apiErr) && apiErr.Status == 429 {
				return m, tea.Batch(
					tea.Printf("Too many failed attempts. Try again later."),
					tea.Quit,
				)
			}
		}
		if m.attempts >= 3 {
			return m, tea.Batch(
				tea.Printf("Too many failed attempts. Try again later."),
				tea.Quit,
			)
		}
		return m, nil
	case loginSuccessMsg:
		m.loading = false
		m.errorText = ""
		m.attempts = 0
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "shift+tab", "up", "down", "j", "k":
			if m.loading {
				return m, nil
			}
			m.toggleFocus(msg.String())
			return m, nil
		case "enter":
			if m.loading {
				return m, nil
			}
			if strings.TrimSpace(m.usernameInput.Value()) == "" || strings.TrimSpace(m.passwordInput.Value()) == "" {
				m.errorText = "Username and password are required"
				return m, nil
			}
			m.loading = true
			m.errorText = ""
			return m, loginCmd(m.usernameInput.Value(), m.passwordInput.Value())
		}
	}

	var cmd tea.Cmd
	if m.focus == 0 {
		m.usernameInput, cmd = m.usernameInput.Update(msg)
		return m, cmd
	}
	m.passwordInput, cmd = m.passwordInput.Update(msg)
	return m, cmd
}

func (m LoginModel) View() string {
	status := mutedText("[Enter] sign in · [Esc] quit")
	if m.loading {
		status = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render("⟳ Signing in...")
	} else if m.errorText != "" {
		status = errorText("✗ " + m.errorText)
	}

	panel := lipgloss.NewStyle().
		Width(max(42, min(56, m.width-8))).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorDeep)).
		Padding(1, 2).
		Render(strings.Join([]string{
			lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("concave tui"),
			"",
			"Username",
			m.usernameInput.View(),
			"",
			"Password",
			m.passwordInput.View(),
			"",
			status,
		}, "\n"))

	return lipgloss.Place(
		max(minWidth, m.width),
		max(24, m.height),
		lipgloss.Center,
		lipgloss.Center,
		panel,
	)
}

func (m *LoginModel) toggleFocus(key string) {
	switch key {
	case "shift+tab", "up", "k":
		m.focus = (m.focus + 1) % 2
		if m.focus == 0 {
			m.focus = 1
		} else {
			m.focus = 0
		}
	default:
		m.focus = (m.focus + 1) % 2
	}
	if m.focus == 0 {
		m.passwordInput.Blur()
		m.usernameInput.Focus()
		return
	}
	m.usernameInput.Blur()
	m.passwordInput.Focus()
}

func loginCmd(username, password string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		session, err := apiLoginFn(ctx, strings.TrimSpace(username), password)
		if err != nil {
			return loginFailureMsg{err: err}
		}
		return loginSuccessMsg{session: session}
	}
}
