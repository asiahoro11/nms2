// Made by YTSworks
// YTS工作室製作
package tools

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

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
	if !isValidTarget(strings.TrimSpace(req.Target)) {
		return Response{}, errors.New("invalid traceroute target")
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "ping", "-n", "4", target) // #nosec G204 -- target is strictly validated
	} else {
		cmd = exec.CommandContext(ctx, "ping", "-c", "4", target) // #nosec G204 -- target is strictly validated
	}

	out, err := cmd.CombinedOutput()
	outputStr := convertOutput(out)
	if err != nil {
		return fmt.Sprintf("Error pinging %s: %v\nOutput: %s", target, err, outputStr)
	}
	return outputStr
}

func runTraceroute(target string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "tracert", "-d", target) // #nosec G204 -- target is strictly validated
	} else {
		path, err := exec.LookPath("traceroute")
		if err != nil {
			return "Error: 'traceroute' command not found. Please install it on the server."
		}
		cmd = exec.CommandContext(ctx, path, "-n", target) // #nosec G204 -- path comes from LookPath and target is validated
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
