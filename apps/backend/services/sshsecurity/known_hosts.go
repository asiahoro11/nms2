package sshsecurity

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var knownHostsMu sync.Mutex

// TOFUCallback pins the first observed host key in an owner-only known_hosts
// file. A later key mismatch is rejected. Provision the file out-of-band when
// first-connection MITM protection is required.
func TOFUCallback(path string) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		knownHostsMu.Lock()
		defer knownHostsMu.Unlock()

		if callback, err := knownhosts.New(path); err == nil {
			err = callback(hostname, remote, key)
			if err == nil {
				return nil
			}
			var keyErr *knownhosts.KeyError
			if !errors.As(err, &keyErr) || len(keyErr.Want) != 0 {
				return fmt.Errorf("SSH host key verification failed: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("load SSH known_hosts: %w", err)
		}

		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("create SSH known_hosts directory: %w", err)
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("open SSH known_hosts: %w", err)
		}
		defer file.Close()
		line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
		if _, err := fmt.Fprintln(file, line); err != nil {
			return fmt.Errorf("pin SSH host key: %w", err)
		}
		return nil
	}
}
