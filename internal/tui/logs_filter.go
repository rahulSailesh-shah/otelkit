package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type logsFilterField int

const (
	logsFilterFieldSeverity logsFilterField = iota
	logsFilterFieldService
	logsFilterFieldBody
)

var logsSeverityOptions = []string{"all", "fatal", "error", "warn", "info", "debug", "trace"}

type logsFilterState struct {
	Severity string
	Service  string
	Body     string
}

type logsFilter struct {
	active       bool
	field        logsFilterField
	state        logsFilterState
	serviceInput textinput.Model
	bodyInput    textinput.Model
}

func newLogsFilter() logsFilter {
	service := textinput.New()
	service.Prompt = ""
	service.Placeholder = "service"
	service.Blur()

	body := textinput.New()
	body.Prompt = ""
	body.Placeholder = "body"
	body.Blur()

	return logsFilter{
		state:        logsFilterState{Severity: "all"},
		serviceInput: service,
		bodyInput:    body,
	}
}

func (f logsFilter) Active() bool { return f.active }

func (f logsFilter) HasCriteria() bool {
	sev := strings.ToLower(strings.TrimSpace(f.state.Severity))
	if sev != "" && sev != "all" {
		return true
	}
	if strings.TrimSpace(f.state.Service) != "" {
		return true
	}
	if strings.TrimSpace(f.state.Body) != "" {
		return true
	}
	return false
}

func (f logsFilter) State() logsFilterState { return f.state }

func (f logsFilter) Field() logsFilterField { return f.field }

func (f *logsFilter) Enter() tea.Cmd {
	f.active = true
	f.field = logsFilterFieldSeverity
	f.serviceInput.SetValue(f.state.Service)
	f.bodyInput.SetValue(f.state.Body)
	return f.focus()
}

func (f *logsFilter) Exit() {
	f.active = false
	f.serviceInput.Blur()
	f.bodyInput.Blur()
}

func (f *logsFilter) Clear() tea.Cmd {
	f.state = logsFilterState{Severity: "all"}
	f.serviceInput.SetValue("")
	f.bodyInput.SetValue("")
	return f.focus()
}

func (f *logsFilter) NextField() tea.Cmd {
	f.field = (f.field + 1) % 3
	return f.focus()
}

func (f *logsFilter) PrevField() tea.Cmd {
	f.field = (f.field + 2) % 3
	return f.focus()
}

func (f *logsFilter) CycleSeverity(delta int) {
	cur := 0
	for i, opt := range logsSeverityOptions {
		if opt == f.state.Severity {
			cur = i
			break
		}
	}
	next := (cur + delta + len(logsSeverityOptions)) % len(logsSeverityOptions)
	f.state.Severity = logsSeverityOptions[next]
}

func (f *logsFilter) UpdateInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch f.field {
	case logsFilterFieldService:
		f.serviceInput, cmd = f.serviceInput.Update(msg)
		f.state.Service = f.serviceInput.Value()
	case logsFilterFieldBody:
		f.bodyInput, cmd = f.bodyInput.Update(msg)
		f.state.Body = f.bodyInput.Value()
	default:
		return nil
	}
	return cmd
}

func (f logsFilter) Matches(r LogRow) bool {
	sev := strings.ToLower(strings.TrimSpace(f.state.Severity))
	if sev != "" && sev != "all" {
		label := strings.ToLower(severityLabel(r.Severity, strOrNil(r.SeverityText)))
		if label != sev {
			return false
		}
	}
	service := strings.ToLower(strings.TrimSpace(f.state.Service))
	if service != "" && !strings.Contains(strings.ToLower(r.Service), service) {
		return false
	}
	body := strings.ToLower(strings.TrimSpace(f.state.Body))
	if body != "" && !strings.Contains(strings.ToLower(r.Body), body) {
		return false
	}
	return true
}

func (f *logsFilter) SetInputWidth(w int) {
	f.serviceInput.SetWidth(w)
	f.bodyInput.SetWidth(w * 2)
}

func (f *logsFilter) focus() tea.Cmd {
	f.serviceInput.Blur()
	f.bodyInput.Blur()
	switch f.field {
	case logsFilterFieldService:
		return f.serviceInput.Focus()
	case logsFilterFieldBody:
		return f.bodyInput.Focus()
	default:
		return nil
	}
}

func (f logsFilter) serviceView() string { return f.serviceInput.View() }
func (f logsFilter) bodyView() string    { return f.bodyInput.View() }
