package webssh

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

var (
	ErrDeviceNotFound = errors.New("device_not_found")
	wsUpgrader        = websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		CheckOrigin:      func(r *http.Request) bool { return true },
	}
)

type Service struct {
	queryDeviceIP func(id string) (string, error)
}

func NewService(queryDeviceIP func(id string) (string, error)) *Service {
	return &Service{queryDeviceIP: queryDeviceIP}
}

type resizeMsg struct {
	Type string `json:"type"`
	Cols uint32 `json:"cols"`
	Rows uint32 `json:"rows"`
}

func (s *Service) Terminal(c *gin.Context) error {
	deviceID := c.Param("id")
	ip, err := s.queryDeviceIP(deviceID)
	if err != nil {
		return ErrDeviceNotFound
	}

	username := c.DefaultQuery("username", "admin")
	password := c.DefaultQuery("password", "")
	portStr := c.DefaultQuery("port", "22")
	port, _ := strconv.Atoi(portStr)
	if port <= 0 {
		port = 22
	}

	ws, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WSSH] upgrade failed: %v", err)
		return err
	}
	defer ws.Close()

	sshCfg := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}
	addr := net.JoinHostPort(ip, fmt.Sprintf("%d", port))
	sshClient, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m[WSSH] SSH connection failed: "+err.Error()+"\x1b[0m\r\n"))
		return nil
	}
	defer sshClient.Close()

	session, err := sshClient.NewSession()
	if err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m[WSSH] Session creation failed: "+err.Error()+"\x1b[0m\r\n"))
		return nil
	}
	defer session.Close()

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 38400,
		ssh.TTY_OP_OSPEED: 38400,
	}
	if err := session.RequestPty("xterm-256color", 24, 80, modes); err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m[WSSH] PTY request failed: "+err.Error()+"\x1b[0m\r\n"))
		return nil
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m[WSSH] Stdin pipe failed: "+err.Error()+"\x1b[0m\r\n"))
		return nil
	}

	session.Stdout = &wsWriter{ws: ws}
	session.Stderr = &wsWriter{ws: ws}

	if err := session.Shell(); err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31m[WSSH] Shell startup failed: "+err.Error()+"\x1b[0m\r\n"))
		return nil
	}

	log.Printf("[WSSH] Connected device %s as %s@%s", deviceID, username, addr)

	sshDone := make(chan struct{})
	go func() {
		_ = session.Wait()
		close(sshDone)
	}()

	for {
		mt, msg, err := ws.ReadMessage()
		if err != nil {
			break
		}
		select {
		case <-sshDone:
			_ = ws.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[33m[WSSH] SSH session closed\x1b[0m\r\n"))
			return nil
		default:
		}

		if mt == websocket.TextMessage {
			var ctrl resizeMsg
			if json.Unmarshal(msg, &ctrl) == nil && ctrl.Type == "resize" {
				cols, rows := ctrl.Cols, ctrl.Rows
				if cols == 0 {
					cols = 80
				}
				if rows == 0 {
					rows = 24
				}
				_ = session.WindowChange(int(rows), int(cols))
				continue
			}
			_, _ = stdinPipe.Write(msg)
		} else if mt == websocket.BinaryMessage {
			_, _ = stdinPipe.Write(msg)
		}
	}

	_ = session.Close()
	<-sshDone
	log.Printf("[WSSH] Disconnected device %s", deviceID)
	return nil
}

type wsWriter struct {
	ws *websocket.Conn
}

func (w *wsWriter) Write(p []byte) (int, error) {
	if err := w.ws.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}
