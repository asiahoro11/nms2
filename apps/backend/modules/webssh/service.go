// Made by YTSworks
// YTS工作室製作
package webssh

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	"management-server/services/sshsecurity"
)

var (
	ErrDeviceNotFound = errors.New("device_not_found")
	wsUpgrader        = websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			parsed, err := url.Parse(origin)
			return err == nil && strings.EqualFold(parsed.Host, r.Host)
		},
	}
)

type Service struct {
	queryDeviceIP func(id string) (string, error)
	ticketsMu     sync.Mutex
	tickets       map[string]terminalTicket
}

type terminalTicket struct {
	deviceID  string
	expiresAt time.Time
}

func NewService(queryDeviceIP func(id string) (string, error)) *Service {
	return &Service{queryDeviceIP: queryDeviceIP, tickets: make(map[string]terminalTicket)}
}

func (s *Service) IssueTicket(deviceID string) (string, time.Time, error) {
	if _, err := s.queryDeviceIP(deviceID); err != nil {
		return "", time.Time{}, ErrDeviceNotFound
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", time.Time{}, err
	}
	value := base64.RawURLEncoding.EncodeToString(random)
	expires := time.Now().Add(30 * time.Second)
	s.ticketsMu.Lock()
	for key, ticket := range s.tickets {
		if time.Now().After(ticket.expiresAt) {
			delete(s.tickets, key)
		}
	}
	s.tickets[value] = terminalTicket{deviceID: deviceID, expiresAt: expires}
	s.ticketsMu.Unlock()
	return value, expires, nil
}

func (s *Service) consumeTicket(value, deviceID string) bool {
	s.ticketsMu.Lock()
	defer s.ticketsMu.Unlock()
	ticket, ok := s.tickets[value]
	delete(s.tickets, value)
	return ok && ticket.deviceID == deviceID && time.Now().Before(ticket.expiresAt)
}

type resizeMsg struct {
	Type string `json:"type"`
	Cols uint32 `json:"cols"`
	Rows uint32 `json:"rows"`
}

func (s *Service) Terminal(c *gin.Context) error {
	deviceID := c.Param("id")
	if !s.consumeTicket(c.Query("ticket"), deviceID) {
		return errors.New("invalid or expired terminal ticket")
	}
	ip, err := s.queryDeviceIP(deviceID)
	if err != nil {
		return ErrDeviceNotFound
	}

	ws, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WSSH] upgrade failed: %v", err)
		return err
	}
	defer ws.Close()
	_ = ws.SetReadDeadline(time.Now().Add(20 * time.Second))
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Port     int    `json:"port"`
	}
	if err := ws.ReadJSON(&credentials); err != nil {
		return errors.New("terminal credentials were not received")
	}
	_ = ws.SetReadDeadline(time.Time{})
	credentials.Username = strings.TrimSpace(credentials.Username)
	if credentials.Username == "" || len(credentials.Username) > 128 || len(credentials.Password) > 1024 {
		return errors.New("invalid terminal credentials")
	}
	port := credentials.Port
	if port <= 0 || port > 65535 {
		port = 22
	}

	sshCfg := &ssh.ClientConfig{
		User:            credentials.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(credentials.Password)},
		HostKeyCallback: sshsecurity.TOFUCallback(filepath.Join("data", "ssh_known_hosts")),
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

	log.Printf("[WSSH] Connected device %s as %s@%s", deviceID, credentials.Username, addr)

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
