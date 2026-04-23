package tools

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

var validHostnameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-\.]{0,253}[a-zA-Z0-9])?$`)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Ping(req PingRequest) (Response, error) {
	var outputBuilder strings.Builder

	for _, target := range req.Targets {
		if target == "" {
			continue
		}
		if !isValidTarget(target) {
			outputBuilder.WriteString(fmt.Sprintf("--- Skipped invalid target: %s ---\n\n", target))
			continue
		}
		outputBuilder.WriteString(fmt.Sprintf("--- Pinging %s ---\n", target))
		outputBuilder.WriteString(runPing(target))
		outputBuilder.WriteString("\n\n")
	}

	return Response{Success: true, Output: outputBuilder.String()}, nil
}

func (s *Service) Traceroute(req TracerouteRequest) (Response, error) {
	return Response{Success: true, Output: runTraceroute(req.Target)}, nil
}

func isValidTarget(target string) bool {
	if net.ParseIP(target) != nil {
		return true
	}
	if len(target) > 255 {
		return false
	}
	return validHostnameRegex.MatchString(target)
}

func runPing(target string) string {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("ping", "-n", "4", target)
	} else {
		cmd = exec.Command("ping", "-c", "4", target)
	}

	out, err := cmd.CombinedOutput()
	outputStr := convertOutput(out)
	if err != nil {
		return fmt.Sprintf("Error pinging %s: %v\nOutput: %s", target, err, outputStr)
	}
	return outputStr
}

func runTraceroute(target string) string {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("tracert", "-d", target)
	} else {
		path, err := exec.LookPath("traceroute")
		if err != nil {
			return "Error: 'traceroute' command not found. Please install it on the server."
		}
		cmd = exec.Command(path, "-n", target)
	}

	out, err := cmd.CombinedOutput()
	outputStr := convertOutput(out)
	if err != nil {
		return fmt.Sprintf("Error running traceroute to %s: %v\nOutput: %s", target, err, outputStr)
	}
	return outputStr
}

func convertOutput(data []byte) string {
	if runtime.GOOS == "windows" {
		reader := transform.NewReader(bytes.NewReader(data), traditionalchinese.Big5.NewDecoder())
		d, err := io.ReadAll(reader)
		if err == nil {
			return string(d)
		}
	}
	return string(data)
}
