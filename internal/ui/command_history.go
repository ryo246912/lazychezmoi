package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

const maxCommandHistoryEntries = 100

type commandHistoryStore interface {
	Load() ([]string, error)
	Save([]string) error
}

type shellCommandRunner func(command string, env []string, action pendingAction) tea.Cmd

type fileCommandHistoryStore struct {
	path string
}

type failingCommandHistoryStore struct {
	err error
}

func newDefaultCommandHistoryStore() commandHistoryStore {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return failingCommandHistoryStore{err: fmt.Errorf("resolve config dir: %w", err)}
	}

	return fileCommandHistoryStore{
		path: filepath.Join(configDir, "lazychezmoi", "command-history.json"),
	}
}

func (s fileCommandHistoryStore) Load() ([]string, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read command history: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	var history []string
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, fmt.Errorf("parse command history: %w", err)
	}

	return normalizeCommandHistory(history), nil
}

func (s fileCommandHistoryStore) Save(history []string) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("create command history dir: %w", err)
	}

	payload, err := json.MarshalIndent(normalizeCommandHistory(history), "", "  ")
	if err != nil {
		return fmt.Errorf("encode command history: %w", err)
	}
	payload = append(payload, '\n')

	if err := os.WriteFile(s.path, payload, 0o644); err != nil {
		return fmt.Errorf("write command history: %w", err)
	}

	return nil
}

func (s failingCommandHistoryStore) Load() ([]string, error) {
	return nil, s.err
}

func (s failingCommandHistoryStore) Save(_ []string) error {
	return s.err
}

func normalizeCommandHistory(history []string) []string {
	seen := make(map[string]struct{}, len(history))
	cleaned := make([]string, 0, min(len(history), maxCommandHistoryEntries))

	for _, command := range history {
		command = trimCommandHistoryValue(command)
		if command == "" {
			continue
		}
		if _, ok := seen[command]; ok {
			continue
		}

		seen[command] = struct{}{}
		cleaned = append(cleaned, command)
		if len(cleaned) >= maxCommandHistoryEntries {
			break
		}
	}

	return cleaned
}

func prependCommandHistory(command string, history []string) []string {
	return normalizeCommandHistory(append([]string{command}, history...))
}

func trimCommandHistoryValue(command string) string {
	return string(bytes.TrimSpace([]byte(command)))
}
