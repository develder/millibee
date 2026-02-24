package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNewModel(t *testing.T) {
	m := NewModel(nil, "test:default")
	assert.Equal(t, "test:default", m.sessionKey)
	assert.Empty(t, m.messages)
	assert.False(t, m.processing)
}

func TestModel_Init(t *testing.T) {
	m := NewModel(nil, "test:default")
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestModel_WindowResize(t *testing.T) {
	m := NewModel(nil, "test:default")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := updated.(Model)
	assert.True(t, model.ready)
	assert.Equal(t, 120, model.width)
	assert.Equal(t, 40, model.height)
}

func TestModel_CtrlCQuits(t *testing.T) {
	m := NewModel(nil, "test:default")
	// Init viewport first
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	// tea.Quit returns a special function
	assert.NotNil(t, cmd)
}

func TestRenderMessages_Empty(t *testing.T) {
	m := NewModel(nil, "test:default")
	result := m.renderMessages()
	assert.Equal(t, "", result)
}

func TestRenderMessages_UserAndAssistant(t *testing.T) {
	m := NewModel(nil, "test:default")
	m.messages = []ChatMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}
	result := m.renderMessages()
	assert.Contains(t, result, "Hello")
	assert.Contains(t, result, "Hi there")
}

func TestRenderMessages_Error(t *testing.T) {
	m := NewModel(nil, "test:default")
	m.messages = []ChatMessage{
		{Role: "error", Content: "something went wrong"},
	}
	result := m.renderMessages()
	assert.Contains(t, result, "something went wrong")
}

func TestResponseMsg_Success(t *testing.T) {
	m := NewModel(nil, "test:default")
	// Init viewport
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	m.processing = true

	updated, _ = m.Update(responseMsg{content: "Agent response"})
	model := updated.(Model)
	assert.False(t, model.processing)
	assert.Len(t, model.messages, 1)
	assert.Equal(t, "assistant", model.messages[0].Role)
	assert.Equal(t, "Agent response", model.messages[0].Content)
}

func TestResponseMsg_Error(t *testing.T) {
	m := NewModel(nil, "test:default")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updated.(Model)
	m.processing = true

	updated, _ = m.Update(responseMsg{err: fmt.Errorf("test error")})
	model := updated.(Model)
	assert.False(t, model.processing)
	assert.Len(t, model.messages, 1)
	assert.Equal(t, "error", model.messages[0].Role)
	assert.Contains(t, model.messages[0].Content, "test error")
}

func TestView_NotReady(t *testing.T) {
	m := NewModel(nil, "test:default")
	view := m.View()
	assert.Contains(t, view, "Initializing")
}
