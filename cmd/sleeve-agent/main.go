package main

import (
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
)

var (
	addr = flag.String("addr", ":7681", "http service address")
	cmd  = flag.String("cmd", "bash", "command to run")
)

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"tty"},
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ResizeMessage struct {
	Columns int `json:"columns"`
	Rows    int `json:"rows"`
}

func serveWS(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	log.Printf("Client connected from %s", r.RemoteAddr)

	// Create command
	cCmd := exec.Command("tmux", "attach-session", "-t", "main")
    // Fallback if tmux fails? Or assume entrypoint set it up?
    // Let's assume entrypoint set it up, or we can try to create if attach fails.
    // But simplistic is better for now.

	// Start PTY
	ptmx, err := pty.Start(cCmd)
	if err != nil {
		log.Printf("Failed to start pty: %s", err)
        c.WriteMessage(websocket.TextMessage, []byte("Failed to start pty: "+err.Error()))
		return
	}
	defer func() {
        log.Printf("Closing pty")
        ptmx.Close()
        cCmd.Process.Kill()
        cCmd.Wait() // Avoid zombies
    }()

	// Handle resize
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			if err := pty.InheritSize(os.Stdin, ptmx); err != nil {
				log.Printf("error resizing pty: %s", err)
			}
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize

	// Copy stdin to websocket (Output)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				if err != io.EOF {
                    // Linux returns EIO when the last file descriptor is closed
					log.Printf("pty read error: %s", err)
				}
				c.Close()
				return
			}
            // Send raw data
			if err := c.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				log.Printf("websocket write error: %s", err)
				return
			}
		}
	}()

	// Copy websocket to stdin (Input)
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Printf("websocket read error: %s", err)
			break
		}

        if len(message) == 0 {
            continue
        }

        // Protocol:
        // 0 + data = Input
        // 1 + json = Resize

        switch message[0] {
        case '0': // Input
            if len(message) > 1 {
                if _, err := ptmx.Write(message[1:]); err != nil {
                    log.Printf("pty write error: %s", err)
                    return
                }
            }
        case '1': // Resize
            var msg ResizeMessage
            if err := json.Unmarshal(message[1:], &msg); err != nil {
                log.Printf("resize json error: %s", err)
                continue
            }
            if err := pty.Setsize(ptmx, &pty.Winsize{
                Rows: uint16(msg.Rows),
                Cols: uint16(msg.Columns),
            }); err != nil {
                log.Printf("pty resize error: %s", err)
            }
        default:
            log.Printf("unknown message type: %d", message[0])
        }

        // Keep the loop happy with mt
        _ = mt
	}
}

func main() {
	flag.Parse()
	log.SetFlags(0)
	http.HandleFunc("/ws", serveWS) // Match existing path proxy logic if possible
    // Envoy proxy sends to /ws?
    // internal/envoy/ws_proxy.go:
    // Path:   "/ws",
    // Yes.

    // Also handle root for health checks if needed?
    // Ttyd was running at /, but the proxy connects to /ws.
    // Wait, ttyd usually runs on / by default.
    // ws_proxy.go explicitly sets Path: "/ws".
    // ttyd exposes /ws for websocket.
    // So if I handle /ws, I am good.

	log.Printf("listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}
