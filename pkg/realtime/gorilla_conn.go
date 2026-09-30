package realtime

import (
	"net/http"
	"sync"
	"time"

	"github.com/combi/pkg/domain/realtime"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 1024
)

type gorillaConnection struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

var _ realtime.Connection = (*gorillaConnection)(nil)

func NewGorillaConnection(w http.ResponseWriter, r *http.Request, allowedOrigins []string) (realtime.Connection, error) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if len(allowedOrigins) == 0 || (len(allowedOrigins) == 1 && allowedOrigins[0] == "*") {
				return true
			}
			for _, allowed := range allowedOrigins {
				if origin == allowed {
					return true
				}
			}
			return false
		},
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, realtime.ErrUpgradeFailed
	}
	return &gorillaConnection{conn: conn}, nil
}

func (c *gorillaConnection) ReadPump(hub realtime.Hub, client *realtime.Client) {
	defer func() {
		hub.Unregister(client)
		c.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)

	// FIX: Check error on initial deadline set
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}

	// FIX: Return the error from SetReadDeadline inside the handler
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		hub.ProcessClientMessage(client, message)
	}
}

func (c *gorillaConnection) WritePump(client *realtime.Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			// FIX: Check error on deadline set
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if !ok {
				// FIX: Explicitly ignore error (best effort close) or handle it
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(message); err != nil {
				return
			}
		case <-ticker.C:
			// FIX: Check error on deadline set
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *gorillaConnection) Close() error {
	return c.conn.Close()
}

func (c *gorillaConnection) WriteJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.WriteJSON(v)
}
