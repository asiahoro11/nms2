package license

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

var machineIDCache struct {
	sync.Once
	value string
}

func SystemMachineID() string {
	machineIDCache.Do(func() { machineIDCache.value = detectMachineID() })
	return machineIDCache.value
}

func machineCommand(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

func detectMachineID() string {
	if runtime.GOOS == "windows" {
		if output, err := machineCommand(2*time.Second, "wmic", "csproduct", "get", "uuid"); err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				value := strings.TrimSpace(line)
				if value != "" && !strings.Contains(strings.ToLower(value), "uuid") {
					return strings.ToLower(value)
				}
			}
		}
		if output, err := machineCommand(2*time.Second, "reg", "query", `HKLM\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid"); err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				if strings.Contains(line, "MachineGuid") {
					parts := strings.Fields(line)
					if len(parts) >= 3 {
						return strings.ToLower(parts[len(parts)-1])
					}
				}
			}
		}
		return "windows-unknown-uuid"
	}
	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id", "/sys/class/dmi/id/product_uuid"} {
		if data, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(data)) != "" {
			return strings.ToLower(strings.TrimSpace(string(data)))
		}
	}
	return "unknown-system-uuid"
}
