package session

import (
	"testing"
	"time"
)

func TestSession(t *testing.T) {
	// Test session creation and message adding
	manager := NewManager()
	sess := manager.NewSession("grok")
	manager.AddMessage(sess, "user", "Hello")
	manager.AddMessage(sess, "assistant", "Hi there")

	if len(sess.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(sess.Messages))
	}

	if sess.Messages[0].Role != "user" || sess.Messages[0].Content != "Hello" {
		t.Errorf("First message incorrect")
	}

	if sess.Messages[1].Role != "assistant" || sess.Messages[1].Content != "Hi there" {
		t.Errorf("Second message incorrect")
	}
}

func TestSessionExpiry(t *testing.T) {
	manager := NewManager()
	sess := manager.NewSession("grok")
	manager.AddMessage(sess, "user", "Hello")

	// Fresh session should not be expired
	if manager.IsExpired(sess) {
		t.Error("Fresh session should not be expired")
	}

	// Manually set old time
	sess.Messages[0].Time = time.Now().Add(-25 * time.Hour)

	if !manager.IsExpired(sess) {
		t.Error("Old session should be expired")
	}
}

func TestSessionSaveLoad(t *testing.T) {
	manager := NewManager()
	sess := manager.NewSession("grok")
	manager.AddMessage(sess, "user", "Test")

	err := manager.Save(sess)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := manager.Load()
	if err != nil {
		t.Fatal(err)
	}

	if loaded.Provider != "grok" {
		t.Errorf("Expected provider 'grok', got '%s'", loaded.Provider)
	}

	if len(loaded.Messages) != 1 || loaded.Messages[0].Content != "Test" {
		t.Error("Message not loaded correctly")
	}
}
