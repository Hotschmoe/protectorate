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

	targetConn, _, err := websocket.DefaultDialer.Dial(targetURL.String(), nil)
	if err != nil {
		log.Printf("failed to connect to ttyd at %s: %v", targetURL.String(), err)
		return
	}
	defer targetConn.Close()

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

	<-errCh
}
