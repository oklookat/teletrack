package api

import "sync"

// client represents one SSE connection.
type client struct {
	events chan []byte
	done   chan struct{}

	closeOnce sync.Once
}

func newClient(buffer int) *client {
	return &client{
		events: make(chan []byte, buffer),
		done:   make(chan struct{}),
	}
}

// enqueue is deliberately non-blocking.
//
// A renderer update must never wait for a browser to consume an event.
func (c *client) enqueue(event []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}

	select {
	case c.events <- event:
		return true
	default:
		// The client is too slow.
		return false
	}
}

func (c *client) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		close(c.events)
	})
}
