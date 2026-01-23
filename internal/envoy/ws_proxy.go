package envoy

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	Subprotocols: []string{"tty"},
}

const (
	pongWait     = 60 * time.Second
	pingInterval = 30 * time.Second
	pingTimeout  = 5 * time.Second
)

func (s *Server) proxyWebSocket(w http.ResponseWriter, r *http.Request, targetAddr string) {
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	defer clientConn.Close()

	clientConn.SetReadDeadline(time.Now().Add(pongWait))
	clientConn.SetPongHandler(func(string) error {
		clientConn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	targetURL := url.URL{
		Scheme: "ws",
		Host:   targetAddr,
		Path:   "/ws",
	}

	dialer := websocket.Dialer{
		Subprotocols:     []string{"tty"},
		HandshakeTimeout: 10 * time.Second,
	}

	targetConn, _, err := dialer.Dial(targetURL.String(), nil)
	if err != nil {
		log.Printf("failed to connect to ttyd at %s: %v", targetURL.String(), err)
		return
	}
	defer targetConn.Close()

	log.Printf("proxy connected: client <-> %s", targetAddr)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		ticker := time.NewTicker(pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := clientConn.WriteControl(websocket.PingMessage, nil, time.Now().Add(pingTimeout)); err != nil {
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	errCh := make(chan error, 2)

	go func() {
		for {
			msgType, msg, err := clientConn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			if err := targetConn.WriteMessage(msgType, msg); err != nil {
				errCh <- err
				return
			}
		}
	}()

	go func() {
		for {
			msgType, msg, err := targetConn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			if err := clientConn.WriteMessage(msgType, msg); err != nil {
				errCh <- err
				return
			}
		}
	}()

	err = <-errCh
	log.Printf("proxy disconnected: %v", err)
}
