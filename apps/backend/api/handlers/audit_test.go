// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactAuditDetailsCoversCredentialsAndRTSPAuth(t *testing.T) {
	details := map[string]interface{}{
		"token":          "plain-token",
		"snmp_community": "private",
		"old_values": map[string]interface{}{
			"rtsp_url": "rtsp://admin:Admin123@192.0.2.10/live?token=abc&password=secret&safe=1",
		},
		"new_values": []interface{}{
			map[string]interface{}{"community": "public"},
			"rtsp://viewer:pass@192.0.2.11/stream?api_key=key&safe=1",
		},
	}

	redacted := redactAuditDetails(details)
	body, err := json.Marshal(redacted)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)

	for _, leaked := range []string{
		"plain-token",
		"private",
		"Admin123",
		"token=abc",
		"password=secret",
		"public",
		"viewer:pass",
		"api_key=key",
	} {
		if strings.Contains(text, leaked) {
			t.Fatalf("audit detail leaked %q in %s", leaked, text)
		}
	}
	if !strings.Contains(text, "safe=1") {
		t.Fatalf("expected non-sensitive query value to be preserved, got %s", text)
	}
}
