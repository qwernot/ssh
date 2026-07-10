package ssh

import (
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// TelnetClient wraps a telnet connection
type TelnetClient struct {
	conn   net.Conn
	mu     sync.Mutex
	closed bool
}

// ConnectTelnet establishes a telnet connection
func ConnectTelnet(host string, port int) (*TelnetClient, error) {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("telnet dial: %w", err)
	}

	return &TelnetClient{conn: conn}, nil
}

func (t *TelnetClient) Read(p []byte) (int, error) {
	if t.closed {
		return 0, io.EOF
	}
	t.conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	return t.conn.Read(p)
}

func (t *TelnetClient) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return 0, io.EOF
	}
	return t.conn.Write(p)
}

func (t *TelnetClient) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return
	}
	t.closed = true
	t.conn.Close()
}
