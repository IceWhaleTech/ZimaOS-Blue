package server

import (
	"sync"
)

// RequestPool provides object pooling for chat requests to reduce GC pressure
type RequestPool struct {
	pool sync.Pool
}

// ChatRequestPooled is a pooled chat request object
type ChatRequestPooled struct {
	UserID        string
	ConversationID string
	Messages      []interface{}
	Model         string
	Temperature   float32
	MaxTokens     int
	Stream        bool
}

// NewRequestPool creates a new request pool
func NewRequestPool() *RequestPool {
	return &RequestPool{
		pool: sync.Pool{
			New: func() interface{} {
				return &ChatRequestPooled{}
			},
		},
	}
}

// Get retrieves a request from the pool
func (rp *RequestPool) Get() *ChatRequestPooled {
	return rp.pool.Get().(*ChatRequestPooled)
}

// Put returns a request to the pool
func (rp *RequestPool) Put(req *ChatRequestPooled) {
	// Reset fields
	req.UserID = ""
	req.ConversationID = ""
	req.Messages = nil
	req.Model = ""
	req.Temperature = 0
	req.MaxTokens = 0
	req.Stream = false
	rp.pool.Put(req)
}

// ResponsePool provides object pooling for chat responses
type ResponsePool struct {
	pool sync.Pool
}

// ChatResponsePooled is a pooled chat response object
type ChatResponsePooled struct {
	ID      string
	Content string
	Tokens  int
	Error   string
}

// NewResponsePool creates a new response pool
func NewResponsePool() *ResponsePool {
	return &ResponsePool{
		pool: sync.Pool{
			New: func() interface{} {
				return &ChatResponsePooled{}
			},
		},
	}
}

// Get retrieves a response from the pool
func (rp *ResponsePool) Get() *ChatResponsePooled {
	return rp.pool.Get().(*ChatResponsePooled)
}

// Put returns a response to the pool
func (rp *ResponsePool) Put(resp *ChatResponsePooled) {
	// Reset fields
	resp.ID = ""
	resp.Content = ""
	resp.Tokens = 0
	resp.Error = ""
	rp.pool.Put(resp)
}
