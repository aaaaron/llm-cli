package response

import (
	"llm/internal/session"
)

// Processor defines the interface for processing LLM responses.
type Processor interface {
	Process(sessionManager *session.Manager, sess *session.Session, query, response, format string) error
}

// DefaultProcessor is the standard implementation that calls Handle.
type DefaultProcessor struct{}

// NewDefaultProcessor creates a new DefaultProcessor.
func NewDefaultProcessor() Processor {
	return &DefaultProcessor{}
}

// Process delegates to Handle for response handling.
func (p *DefaultProcessor) Process(sessionManager *session.Manager, sess *session.Session, query, response, format string) error {
	Handle(sessionManager, sess, query, response, format)
	return nil
}
