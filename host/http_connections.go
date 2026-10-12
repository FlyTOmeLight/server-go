package host

import (
	"net"
	"sync"
)

// httpListener retains ownership when net/http hands a connection to H2C or a
// hijacking handler. Closed connections remove themselves from the live set.
type httpListener struct {
	net.Listener
	mu          sync.Mutex
	connections map[*httpConnection]struct{}
	active      sync.WaitGroup
	closed      bool
	closeOnce   sync.Once
	closeErr    error
}

func newHTTPListener(listener net.Listener) *httpListener {
	return &httpListener{Listener: listener, connections: make(map[*httpConnection]struct{})}
}

func (l *httpListener) Accept() (net.Conn, error) {
	connection, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		_ = connection.Close()
		return nil, net.ErrClosed
	}
	owned := &httpConnection{Conn: connection, listener: l}
	l.connections[owned] = struct{}{}
	l.active.Add(1)
	return owned, nil
}

func (l *httpListener) Close() error {
	l.closeOnce.Do(func() {
		l.mu.Lock()
		l.closed = true
		l.mu.Unlock()
		l.closeErr = l.Listener.Close()
	})
	return l.closeErr
}

func (l *httpListener) closeConnections() {
	l.mu.Lock()
	connections := make([]*httpConnection, 0, len(l.connections))
	for connection := range l.connections {
		connections = append(connections, connection)
	}
	l.mu.Unlock()
	for _, connection := range connections {
		_ = connection.Close()
	}
}

// Call only after Close has ended connection admission.
func (l *httpListener) wait() { l.active.Wait() }

type httpConnection struct {
	net.Conn
	listener  *httpListener
	closeOnce sync.Once
	closeErr  error
}

func (c *httpConnection) Close() error {
	c.closeOnce.Do(func() {
		c.closeErr = c.Conn.Close()
		c.listener.mu.Lock()
		delete(c.listener.connections, c)
		c.listener.mu.Unlock()
		c.listener.active.Done()
	})
	return c.closeErr
}
