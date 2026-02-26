// MilliBee - Ultra-lightweight personal AI agent
// License: MIT
//
// Copyright (c) 2026 MilliBee contributors

package channels

import (
	"context"
	"fmt"
	"sync"

	"github.com/develder/millibee/pkg/bus"
	"github.com/develder/millibee/pkg/config"
	"github.com/develder/millibee/pkg/constants"
	"github.com/develder/millibee/pkg/logger"
)

type Manager struct {
	channels     map[string]Channel
	bus          *bus.MessageBus
	config       *config.Config
	dispatchTask *asyncTask
	mu           sync.RWMutex
}

type asyncTask struct {
	cancel context.CancelFunc
}

func NewManager(cfg *config.Config, messageBus *bus.MessageBus) (*Manager, error) {
	m := &Manager{
		channels: make(map[string]Channel),
		bus:      messageBus,
		config:   cfg,
	}

	if err := m.initChannels(); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manager) initChannels() error {
	logger.InfoC("channels", "Initializing channel manager")

	if m.config.Channels.Telegram.Enabled && m.config.Channels.Telegram.Token != "" {
		logger.DebugC("channels", "Attempting to initialize Telegram channel")
		telegram, err := NewTelegramChannel(m.config, m.bus)
		if err != nil {
			logger.ErrorCF("channels", "Failed to initialize Telegram channel", map[string]any{
				"error": err.Error(),
			})
		} else {
			m.channels["telegram"] = telegram
			logger.InfoC("channels", "Telegram channel enabled successfully")
		}
	}

	if m.config.Channels.SSH.Enabled {
		logger.DebugC("channels", "Attempting to initialize SSH channel")
		sshCh, err := NewSSHChannel(m.config.Channels.SSH, m.bus)
		if err != nil {
			logger.ErrorCF("channels", "Failed to initialize SSH channel", map[string]any{
				"error": err.Error(),
			})
		} else {
			m.channels["ssh"] = sshCh
			logger.InfoC("channels", "SSH channel enabled successfully")
		}
	}

	if m.config.Channels.Console.Enabled {
		logger.DebugC("channels", "Attempting to initialize Console channel")
		console, err := NewConsoleChannel(m.config.Channels.Console, m.bus)
		if err != nil {
			logger.ErrorCF("channels", "Failed to initialize Console channel", map[string]any{
				"error": err.Error(),
			})
		} else {
			m.channels["console"] = console
			logger.InfoC("channels", "Console channel enabled successfully")
		}
	}

	logger.InfoCF("channels", "Channel initialization completed", map[string]any{
		"enabled_channels": len(m.channels),
	})

	return nil
}

func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.channels) == 0 {
		logger.WarnC("channels", "No channels enabled")
		return nil
	}

	logger.InfoC("channels", "Starting all channels")

	dispatchCtx, cancel := context.WithCancel(ctx)
	m.dispatchTask = &asyncTask{cancel: cancel}

	go m.dispatchOutbound(dispatchCtx)

	for name, channel := range m.channels {
		logger.InfoCF("channels", "Starting channel", map[string]any{
			"channel": name,
		})
		if err := channel.Start(ctx); err != nil {
			logger.ErrorCF("channels", "Failed to start channel", map[string]any{
				"channel": name,
				"error":   err.Error(),
			})
		}
	}

	logger.InfoC("channels", "All channels started")
	return nil
}

func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	logger.InfoC("channels", "Stopping all channels")

	if m.dispatchTask != nil {
		m.dispatchTask.cancel()
		m.dispatchTask = nil
	}

	for name, channel := range m.channels {
		logger.InfoCF("channels", "Stopping channel", map[string]any{
			"channel": name,
		})
		if err := channel.Stop(ctx); err != nil {
			logger.ErrorCF("channels", "Error stopping channel", map[string]any{
				"channel": name,
				"error":   err.Error(),
			})
		}
	}

	logger.InfoC("channels", "All channels stopped")
	return nil
}

func (m *Manager) dispatchOutbound(ctx context.Context) {
	logger.InfoC("channels", "Outbound dispatcher started")

	for {
		select {
		case <-ctx.Done():
			logger.InfoC("channels", "Outbound dispatcher stopped")
			return
		default:
			msg, ok := m.bus.SubscribeOutbound(ctx)
			if !ok {
				continue
			}

			// Silently skip internal channels
			if constants.IsInternalChannel(msg.Channel) {
				continue
			}

			m.mu.RLock()
			channel, exists := m.channels[msg.Channel]
			m.mu.RUnlock()

			if !exists {
				logger.WarnCF("channels", "Unknown channel for outbound message", map[string]any{
					"channel": msg.Channel,
				})
				continue
			}

			if err := channel.Send(ctx, msg); err != nil {
				logger.ErrorCF("channels", "Error sending message to channel", map[string]any{
					"channel": msg.Channel,
					"error":   err.Error(),
				})
			}
		}
	}
}

func (m *Manager) GetChannel(name string) (Channel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	channel, ok := m.channels[name]
	return channel, ok
}

func (m *Manager) GetStatus() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]any)
	for name, channel := range m.channels {
		status[name] = map[string]any{
			"enabled": true,
			"running": channel.IsRunning(),
		}
	}
	return status
}

func (m *Manager) GetEnabledChannels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.channels))
	for name := range m.channels {
		names = append(names, name)
	}
	return names
}

func (m *Manager) RegisterChannel(name string, channel Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[name] = channel
}

func (m *Manager) UnregisterChannel(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.channels, name)
}

// StreamCallbackFor returns a streaming callback that routes chunks to the
// named channel's SendChunk method. Returns nil if the channel does not
// support streaming. The returned flush function should be called when
// streaming is complete.
func (m *Manager) StreamCallbackFor(channelName, chatID string) (func(string), func()) {
	m.mu.RLock()
	ch, exists := m.channels[channelName]
	m.mu.RUnlock()

	if !exists {
		return nil, nil
	}

	streamer, ok := ch.(StreamingChannel)
	if !ok {
		return nil, nil
	}

	ctx := context.Background()
	callback := func(chunk string) {
		_ = streamer.SendChunk(ctx, chatID, chunk)
	}
	flush := func() {
		_ = streamer.FlushStream(ctx, chatID)
	}
	return callback, flush
}

func (m *Manager) SendToChannel(ctx context.Context, channelName, chatID, content string) error {
	m.mu.RLock()
	channel, exists := m.channels[channelName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("channel %s not found", channelName)
	}

	msg := bus.OutboundMessage{
		Channel: channelName,
		ChatID:  chatID,
		Content: content,
	}

	return channel.Send(ctx, msg)
}
