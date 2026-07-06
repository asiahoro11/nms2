package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHandleGenerateCustomCheckboxFeatures(t *testing.T) {
	form := url.Values{}
	form.Set("mode", "poc")
	form.Set("license_type", "custom")
	form.Set("duration_days", "30")
	form.Set("device_count", "0")
	form.Set("camera_count", "0")
	form.Add("features", "pdu")
	form.Add("features", "iot")

	req := httptest.NewRequest(http.MethodPost, "/generate", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handleGenerate(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	for _, want := range []string{"授權已產生", "PDU / UPS", "IoT / Modbus"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response to contain %q, body: %s", want, body)
		}
	}
}
