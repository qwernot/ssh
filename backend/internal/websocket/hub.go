package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Message types
const (
	MsgTypeInput    = "input"    // Client -> Server: terminal input
	MsgTypeOutput   = "output"   // Server -> Client: terminal output
	MsgTypeResize   = "resize"   // Client -> Server: terminal resize
	MsgTypeConnect  = "connect"  // Server -> Client: connection established
	MsgTypeClose    = "close"    // Server -> Client: connection closed
	MsgTypeError    = "error"    // Server -> Client: error message
	MsgTypePing     = "ping"     // Keepalive
	MsgTypePong     = "pong"     // Keepalive response
)

type WSMessage struct {
	Type    string          `json:"type"`
	Data    json.RawMessage `json:"data,omitempty"`
	TermID  string          `json:"term_id,omitempty"`
}

// TerminalSession represents an active terminal connection
type TerminalSession struct {
	ID         string
	Conn       *websocket.Conn
	OutputCh   chan []byte
	CloseCh    chan struct{}
	Mu         sync.Mutex
	Closed     bool
	OnInput    func(data []byte)
	OnResize   func(cols, rows uint16)
	OnClose    func()
	CreatedAt  time.Time
}

func (ts *TerminalSession) WriteOutput(data []byte) {
	ts.Mu.Lock()
	defer ts.Mu.Unlock()
	if ts.Closed {
		return
	}
	select {
	case ts.OutputCh <- data:
	default:
		// Buffer full, drop
	}
}

func (ts *TerminalSession) Close() {
	ts.Mu.Lock()
	defer ts.Mu.Unlock()
	if ts.Closed {
		return
	}
	ts.Closed = true
	close(ts.CloseCh)
	if ts.OnClose != nil {
		ts.OnClose()
	}
	ts.Conn.Close()
}

func (ts *TerminalSession) SendJSON(msg WSMessage) error {
	ts.Mu.Lock()
	defer ts.Mu.Unlock()
	if ts.Closed {
		return nil
	}
	ts.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return ts.Conn.WriteJSON(msg)
}

// Hub manages all terminal sessions
type Hub struct {
	sessions map[string]*TerminalSession
	mu       sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		sessions: make(map[string]*TerminalSession),
	}
}

func (h *Hub) Register(session *TerminalSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[session.ID] = session
}

func (h *Hub) Unregister(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if session, ok := h.sessions[id]; ok {
		session.Close()
		delete(h.sessions, id)
	}
}

func (h *Hub) GetSession(id string) *TerminalSession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessions[id]
}

func (h *Hub) GetSessionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.sessions)
}

// HandleWebSocket upgrades HTTP to WebSocket and manages the terminal session
func (h *Hub) HandleWebSocket(c *gin.Context, termID string, setupFn func(session *TerminalSession) error) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	session := &TerminalSession{
		ID:        termID,
		Conn:      conn,
		OutputCh:  make(chan []byte, 256),
		CloseCh:   make(chan struct{}),
		CreatedAt: time.Now(),
	}

	h.Register(session)

	// Setup the connection (SSH, telnet, etc.)
	if err := setupFn(session); err != nil {
		msg := WSMessage{
			Type: MsgTypeError,
			Data: json.RawMessage(`"` + err.Error() + `"`),
		}
		session.SendJSON(msg)
		session.Close()
		h.Unregister(termID)
		return
	}

	// Notify client of connection
	session.SendJSON(WSMessage{Type: MsgTypeConnect, TermID: termID})

	// Output writer goroutine
	go func() {
		defer session.Close()
		for {
			select {
			case data, ok := <-session.OutputCh:
				if !ok {
					return
				}
				msg := WSMessage{
					Type:   MsgTypeOutput,
					TermID: termID,
					Data:   json.RawMessage(`"` + escapeJSON(string(data)) + `"`),
				}
				if err := session.SendJSON(msg); err != nil {
					return
				}
			case <-session.CloseCh:
				return
			}
		}
	}()

	// Input reader loop
	conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))
		return nil
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

		var wsMsg WSMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			continue
		}

		switch wsMsg.Type {
		case MsgTypeInput:
			var data string
			if json.Unmarshal(wsMsg.Data, &data) == nil && session.OnInput != nil {
				session.OnInput([]byte(data))
			}
		case MsgTypeResize:
			var resize struct {
				Cols uint16 `json:"cols"`
				Rows uint16 `json:"rows"`
			}
			if json.Unmarshal(wsMsg.Data, &resize) == nil && session.OnResize != nil {
				session.OnResize(resize.Cols, resize.Rows)
			}
		case MsgTypePong:
			// Keepalive response
		}
	}

	session.SendJSON(WSMessage{Type: MsgTypeClose, TermID: termID})
	session.Close()
	h.Unregister(termID)
}

func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	// Remove surrounding quotes since we add them in the template
	return string(b[1 : len(b)-1])
}
