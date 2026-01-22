package envoy

import (
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	Subprotocols: []string{"tty"},
}

func (s *Server) proxyWebSocket(w http.ResponseWriter, r *http.Request, targetAddr string) {
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}
	defer clientConn.Close()

	targetURL := url.URL{
		Scheme: "ws",
		Host:   targetAddr,
		Path:   "/ws",
	}

	dialer := websocket.Dialer{
		Subprotocols: []string{"tty"},
	}

	targetConn, _, err := dialer.Dial(targetURL.String(), nil)
	if err != nil {
		log.Printf("failed to connect to ttyd at %s: %v", targetURL.String(), err)
		return
	}
	defer targetConn.Close()

	log.Printf("proxy connected: client <-> %s", targetAddr)

	errCh := make(chan error, 2)

	go func() {
		for {
			msgType, msg, err := clientConn.ReadMessage()
			if err != nil {
				log.Printf("client read error: %v", err)
				errCh <- err
				return
			}
			log.Printf("client -> ttyd: type=%d len=%d first=%v", msgType, len(msg), msg[:min(10, len(msg))])
			if err := targetConn.WriteMessage(msgType, msg); err != nil {
				log.Printf("ttyd write error: %v", err)
				errCh <- err
				return
			}
		}
	}()

	go func() {
		for {
			msgType, msg, err := targetConn.ReadMessage()
			if err != nil {
				log.Printf("ttyd read error: %v", err)
				errCh <- err
				return
			}
			log.Printf("ttyd -> client: type=%d len=%d first=%v", msgType, len(msg), msg[:min(10, len(msg))])
			if err := clientConn.WriteMessage(msgType, msg); err != nil {
				log.Printf("client write error: %v", err)
				errCh <- err
				return
			}
		}
	}()

	err = <-errCh
	log.Printf("proxy disconnected: %v", err)
}
