// Made by YTSworks
// YTS工作室製作
package edgecore

import (
	"fmt"
	"io"
	"log"
	"net"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"management-server/services/sshsecurity"
)

const sshPort = 22
const sshTimeout = 20 * time.Second

// SSHClient wraps an interactive PTY session to an EdgeCore CLI switch.
//
// Confirmed compatible models: ECS2100, ECS4150, ECS2220 (and likely other EdgeCore series).
//
// Common EdgeCore SSH behaviour:
//   - SSH login drops directly into privileged EXEC mode (no "enable" needed)
//   - Prompt formats: "Vty-N#", "Vty-N(config)#", "Switch#", "Switch(config)#"
//   - reload → confirm with "y" → device reboots
//   - show running-config → works directly from privileged mode
//   - configure → interface ethernet 1/N → commands → exit → end
//   - Save: "copy running-config startup-config" ("write memory" is invalid on some models)
type SSHClient struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	outCh   chan string
}

// NewSSHClient dials and opens a PTY shell. Drains the login banner and
// waits for the initial "Vty-N#" prompt before returning.
func NewSSHClient(ip, username, password string) (*SSHClient, error) {
	cfg := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: sshsecurity.TOFUCallback(filepath.Join("data", "ssh_known_hosts")),
		Timeout:         sshTimeout,
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort(ip, fmt.Sprintf("%d", sshPort)), cfg)
	if err != nil {
		return nil, fmt.Errorf("SSH dial %s: %w", ip, err)
	}

	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("SSH session: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 38400,
		ssh.TTY_OP_OSPEED: 38400,
	}
	if err := session.RequestPty("vt100", 24, 512, modes); err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("PTY: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, err
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		client.Close()
		return nil, err
	}

	if err := session.Shell(); err != nil {
		session.Close()
		client.Close()
		return nil, fmt.Errorf("shell: %w", err)
	}

	sc := &SSHClient{
		client:  client,
		session: session,
		stdin:   stdin,
		outCh:   make(chan string, 1024),
	}

	// Background reader: stdout → line channel.
	// Reads byte-by-byte; flushes on \n OR when the partial line looks like
	// a prompt/confirmation (ends with #, >, ?, or --More--) so that
	// waitPrompt can detect lines that the device does not terminate with \n.
	go func() {
		defer close(sc.outCh)
		buf := make([]byte, 1)
		var line strings.Builder
		flushLine := func() {
			if line.Len() > 0 {
				sc.outCh <- line.String()
				line.Reset()
			}
		}
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				ch := buf[0]
				switch ch {
				case '\n':
					flushLine()
				case '\r':
					// ignore carriage return
				default:
					line.WriteByte(ch)
					// Flush immediately when the partial line matches a prompt
					// so waitPrompt doesn't have to wait for \n.
					t := strings.TrimSpace(line.String())
					if strings.HasSuffix(t, "#") ||
						strings.HasSuffix(t, ">") ||
						strings.HasSuffix(t, "?") ||
						strings.HasSuffix(t, ":") ||
						strings.Contains(t, "--More--") ||
						strings.Contains(strings.ToLower(t), "(y/n)") ||
						strings.Contains(strings.ToLower(t), "[y/n]") ||
						strings.Contains(strings.ToLower(t), "(yes/no)") {
						flushLine()
					}
				}
			}
			if err != nil {
				flushLine()
				return
			}
		}
	}()

	// Drain banner — ECS2100 shows a WARNING block before "Vty-N#"
	sc.waitPrompt(12 * time.Second)
	return sc, nil
}

// Close terminates the SSH session and connection.
func (s *SSHClient) Close() {
	if s.session != nil {
		s.session.Close()
	}
	if s.client != nil {
		s.client.Close()
	}
}

// waitPrompt reads lines from outCh until a prompt line appears or timeout.
func (s *SSHClient) waitPrompt(timeout time.Duration) string {
	var sb strings.Builder
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case line, ok := <-s.outCh:
			if !ok {
				return sb.String()
			}
			log.Printf("[ECS SSH] << %s", line)
			sb.WriteString(line + "\n")
			if isPrompt(line) {
				return sb.String()
			}
		case <-timer.C:
			log.Printf("[ECS SSH] waitPrompt timeout (%v)", timeout)
			return sb.String()
		}
	}
}

// isPrompt returns true for lines that indicate the CLI is ready for input.
//
// Confirmed EdgeCore prompt formats (ECS2100 / ECS4150 / ECS2220):
//   - "Vty-1#", "Vty-1(config)#", "Vty-1(config-if)#"   (most models)
//   - "Switch#", "Switch(config)#"                        (some models)
//   - "Vty-1>", "Switch>"                                 (user EXEC mode)
//
// Must NOT match config content ending with > (e.g. "!<stackingDB>"):
//
//	Rule for ">": line must start with "Vty" or "Switch" (case-insensitive).
//
// Must NOT match config comments ending with # (e.g. "spanning-tree mode rstp #comment"):
//
//	Rule for "#": line must be short (<= 40 chars) and contain no spaces.
func isPrompt(line string) bool {
	t := strings.TrimSpace(line)
	tl := strings.ToLower(t)
	if strings.Contains(t, "--More--") ||
		strings.Contains(tl, "(y/n)") ||
		strings.Contains(tl, "[y/n]") ||
		strings.Contains(tl, "(yes/no)") ||
		strings.HasSuffix(tl, "[startup1.cfg]:") ||
		strings.HasSuffix(tl, "]:") {
		return true
	}
	// CLI prompt: short, no spaces, ends with #
	if len(t) <= 40 && !strings.Contains(t, " ") && strings.HasSuffix(t, "#") {
		return true
	}
	// CLI prompt ending with >: must start with a known prompt prefix
	// to avoid matching config content lines like "!<stackingDB>"
	if len(t) <= 40 && !strings.Contains(t, " ") && strings.HasSuffix(t, ">") {
		if strings.HasPrefix(tl, "vty") || strings.HasPrefix(tl, "switch") {
			return true
		}
	}
	return false
}

// send writes a command line to stdin.
func (s *SSHClient) send(cmd string) {
	log.Printf("[ECS SSH] >> %s", cmd)
	fmt.Fprintf(s.stdin, "%s\n", cmd)
}

// RunShow executes a show command and collects the full output,
// automatically pressing space through any --More-- pager pages.
func (s *SSHClient) RunShow(cmd string) string {
	s.send(cmd)
	var full strings.Builder
	for {
		out := s.waitPrompt(12 * time.Second)
		full.WriteString(out)
		if strings.Contains(out, "--More--") {
			fmt.Fprint(s.stdin, " ")
			continue
		}
		break
	}
	return full.String()
}

// ShowRunningConfig retrieves the running-config.
// ECS2100: already privileged at login, issue directly.
// Send "terminal length 0" first to disable the pager so the full config
// arrives without --More-- interruptions.
func (s *SSHClient) ShowRunningConfig() (string, error) {
	s.send("terminal length 0")
	s.waitPrompt(4 * time.Second)
	raw := s.RunShow("show running-config")
	return cleanOutput(raw, "show running-config"), nil
}

// Reboot sends "reload" and confirms with "y".
func (s *SSHClient) Reboot() error {
	s.send("reload")
	out := s.waitPrompt(15 * time.Second)
	lo := strings.ToLower(out)
	if strings.Contains(lo, "y/n") ||
		strings.Contains(lo, "yes/no") ||
		strings.Contains(lo, "confirm") ||
		strings.Contains(lo, "?") {
		s.send("y")
		time.Sleep(500 * time.Millisecond)
	} else {
		// Some firmware versions accept reload without confirmation
		log.Printf("[ECS SSH] Reboot: no confirmation prompt received, assuming accepted. out=%q", out)
	}
	return nil
}

// enterConfig enters global configuration mode (configure → Vty-N(config)#).
func (s *SSHClient) enterConfig() {
	s.send("configure")
	s.waitPrompt(4 * time.Second)
}

// exitConfig exits configuration mode and saves to startup-config.
// ECS2100: "write memory" is invalid; use "copy running-config startup-config".
// Device prompts: "Startup configuration file name [startup1.cfg]:" → press Enter.
func (s *SSHClient) exitConfig() {
	s.send("end")
	s.waitPrompt(3 * time.Second)
	s.send("copy running-config startup-config")
	out := s.waitPrompt(6 * time.Second)
	// If device asks for filename, press Enter to accept default
	if strings.Contains(strings.ToLower(out), "file name") || strings.HasSuffix(strings.TrimSpace(out), "]:") {
		s.send("")
		s.waitPrompt(8 * time.Second)
	}
}

// PoEPortEnable enables PoE on a 1-based port number.
// ECS2100 command: "power inline" (not "power inline auto")
func (s *SSHClient) PoEPortEnable(portNum int) error {
	s.enterConfig()
	s.send(fmt.Sprintf("interface ethernet 1/%d", portNum))
	s.waitPrompt(3 * time.Second)
	s.send("power inline")
	s.waitPrompt(3 * time.Second)
	s.send("exit")
	s.waitPrompt(3 * time.Second)
	s.exitConfig()
	return nil
}

// PoEPortDisable disables PoE (no power inline) on a 1-based port number.
func (s *SSHClient) PoEPortDisable(portNum int) error {
	s.enterConfig()
	s.send(fmt.Sprintf("interface ethernet 1/%d", portNum))
	s.waitPrompt(3 * time.Second)
	s.send("no power inline")
	s.waitPrompt(3 * time.Second)
	s.send("exit")
	s.waitPrompt(3 * time.Second)
	s.exitConfig()
	return nil
}

// PoEPortRecycle power-cycles a PoE port within a single SSH session:
// disable → wait waitSec seconds → enable.
func (s *SSHClient) PoEPortRecycle(portNum int, waitSec int) error {
	if waitSec <= 0 {
		waitSec = 3
	}

	// --- Disable ---
	s.enterConfig()
	s.send(fmt.Sprintf("interface ethernet 1/%d", portNum))
	s.waitPrompt(3 * time.Second)
	s.send("no power inline")
	s.waitPrompt(3 * time.Second)
	s.send("exit")
	s.waitPrompt(3 * time.Second)
	s.send("end")
	s.waitPrompt(2 * time.Second)

	log.Printf("[ECS SSH] PoE recycle port %d: off, waiting %ds", portNum, waitSec)
	time.Sleep(time.Duration(waitSec) * time.Second)

	// --- Enable ---
	s.enterConfig()
	s.send(fmt.Sprintf("interface ethernet 1/%d", portNum))
	s.waitPrompt(3 * time.Second)
	s.send("power inline")
	s.waitPrompt(3 * time.Second)
	s.send("exit")
	s.waitPrompt(3 * time.Second)
	s.exitConfig()
	return nil
}

// cleanOutput strips the echoed command and trailing prompt lines from raw output.
func cleanOutput(raw, cmd string) string {
	lines := strings.Split(raw, "\n")
	var result []string
	started := false
	for _, line := range lines {
		clean := strings.TrimRight(line, "\r ")
		if !started {
			if strings.Contains(clean, cmd) {
				started = true
			}
			continue
		}
		t := strings.TrimSpace(clean)
		if strings.HasSuffix(t, "#") || strings.HasSuffix(t, ">") {
			continue
		}
		result = append(result, clean)
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}
