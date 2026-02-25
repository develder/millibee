package channels

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/develder/millibee/pkg/bus"
	"github.com/develder/millibee/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsoleChannel_Name(t *testing.T) {
	ch := newTestConsole(t)
	assert.Equal(t, "console", ch.Name())
}

func TestConsoleChannel_StartStop(t *testing.T) {
	ch := newTestConsole(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, ch.Start(ctx))
	assert.True(t, ch.IsRunning())

	require.NoError(t, ch.Stop(ctx))
	assert.False(t, ch.IsRunning())
}

func TestConsoleChannel_Send(t *testing.T) {
	var buf bytes.Buffer
	msgBus := bus.NewMessageBus()
	defer msgBus.Close()

	ch := &ConsoleChannel{
		BaseChannel: NewBaseChannel("console", config.ConsoleConfig{Enabled: true}, msgBus, nil),
		reader:      strings.NewReader(""),
		writer:      &buf,
	}
	ch.setRunning(true)

	err := ch.Send(context.Background(), bus.OutboundMessage{
		Channel: "console",
		ChatID:  "console",
		Content: "Hello from agent",
	})
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Hello from agent")
}

func TestConsoleChannel_Send_NotRunning(t *testing.T) {
	ch := newTestConsole(t)
	// Not started, so not running

	err := ch.Send(context.Background(), bus.OutboundMessage{
		Content: "test",
	})
	assert.Error(t, err)
}

func TestConsoleChannel_ReadInput(t *testing.T) {
	msgBus := bus.NewMessageBus()
	defer msgBus.Close()

	input := "Hello agent\n"
	var buf bytes.Buffer

	ch := &ConsoleChannel{
		BaseChannel: NewBaseChannel("console", config.ConsoleConfig{Enabled: true}, msgBus, nil),
		reader:      strings.NewReader(input),
		writer:      &buf,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, ch.Start(ctx))

	// Wait for the message to be published
	msg, ok := msgBus.ConsumeInbound(ctx)
	require.True(t, ok)
	assert.Equal(t, "console", msg.Channel)
	assert.Equal(t, "console", msg.SenderID)
	assert.Equal(t, "Hello agent", msg.Content)
}

func TestConsoleChannel_SkipsEmptyLines(t *testing.T) {
	msgBus := bus.NewMessageBus()
	defer msgBus.Close()

	input := "\n\n  \nactual message\n"
	var buf bytes.Buffer

	ch := &ConsoleChannel{
		BaseChannel: NewBaseChannel("console", config.ConsoleConfig{Enabled: true}, msgBus, nil),
		reader:      strings.NewReader(input),
		writer:      &buf,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	require.NoError(t, ch.Start(ctx))

	msg, ok := msgBus.ConsumeInbound(ctx)
	require.True(t, ok)
	assert.Equal(t, "actual message", msg.Content)
}

func TestConsoleChannel_IsAllowed(t *testing.T) {
	ch := newTestConsole(t)
	// Empty allowList = allow all
	assert.True(t, ch.IsAllowed("console"))
	assert.True(t, ch.IsAllowed("anyone"))
}

func newTestConsole(t *testing.T) *ConsoleChannel {
	t.Helper()
	msgBus := bus.NewMessageBus()
	t.Cleanup(func() { msgBus.Close() })
	cfg := config.ConsoleConfig{Enabled: true}
	ch, err := NewConsoleChannel(cfg, msgBus)
	require.NoError(t, err)
	return ch
}
