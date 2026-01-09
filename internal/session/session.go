/*
Package session provides session management for the LLM CLI tool.

Manager persists conversation history to ~/.llm_session.json for up to 24 hours.
*/
package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Message is the core unit of chat data in a session.
type Message struct {
	Role    string    `json:"role"`
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

// Session represents the state of a conversation with an LLM provider.
type Session struct {
	Messages            []Message `json:"messages"`
	Provider            string    `json:"provider"`
	OpenRouterFreeModel string    `json:"openrouter_free_model,omitempty"`
}

// Manager provides methods for loading, saving, and manipulating sessions.
type Manager struct{}

// NewManager creates a new Manager instance.
func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) sessionFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".llm_session.json")
}

// Load reads the session from the JSON file.
// Returns an empty session if the file does not exist.
func (m *Manager) Load() (*Session, error) {
	file := m.sessionFile()
	data, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return &Session{}, nil
		}
		return nil, err
	}
	var sess Session
	err = json.Unmarshal(data, &sess)
	return &sess, err
}

// Save serializes the session to the JSON file with indentation.
func (m *Manager) Save(sess *Session) error {
	file := m.sessionFile()
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, data, 0644)
}

// IsExpired checks if the session has expired (24 hours since last message).
func (m *Manager) IsExpired(sess *Session) bool {
	if len(sess.Messages) == 0 {
		return false
	}
	lastMsg := sess.Messages[len(sess.Messages)-1]
	return time.Since(lastMsg.Time) > 24*time.Hour
}

// AddMessage appends a new message to the session.
func (m *Manager) AddMessage(sess *Session, role, content string) {
	sess.Messages = append(sess.Messages, Message{
		Role:    role,
		Content: content,
		Time:    time.Now(),
	})
}

// GetMessages returns the list of messages in the session.
func (m *Manager) GetMessages(sess *Session) []Message {
	return sess.Messages
}

// NewSession creates a new empty session for the given provider.
func (m *Manager) NewSession(provider string) *Session {
	return &Session{Provider: provider}
}
