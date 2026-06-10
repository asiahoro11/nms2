package handlers

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	logsmodule "management-server/modules/logs"

	"github.com/gin-gonic/gin"
	_ "modernc.org/sqlite"
)

func setupLogExportTestHandler(t *testing.T) *Handler {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`
		CREATE TABLE system_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			level TEXT,
			service TEXT,
			event_code TEXT,
			message TEXT,
			context_json TEXT,
			node_name TEXT,
			build_version TEXT,
			review_status TEXT DEFAULT 'pending',
			reviewed_by TEXT,
			reviewed_at TEXT,
			review_note TEXT
		);
		INSERT INTO system_logs (
			service, level, event_code, message, occurred_at,
			context_json, node_name, build_version, review_status,
			reviewed_by, reviewed_at, review_note
		)
		VALUES (
			'nms', 'info', 'test_event', 'export row', '2026-05-25 10:00:00',
			'', 'test-node', 'test-build', 'pending',
			'', '', ''
		);
	`); err != nil {
		t.Fatalf("seed test db: %v", err)
	}

	return &Handler{
		db:   db,
		logs: logsmodule.NewService(db),
	}
}

func performLogExportRequest(t *testing.T, h *Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/log-center/export", h.ExportLogCenter)
	router.GET("/log-center/evidence-bundle", h.ExportLogEvidenceBundle)

	req := httptest.NewRequest(http.MethodGet, target, nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestExportLogCenterFormats(t *testing.T) {
	h := setupLogExportTestHandler(t)

	tests := []struct {
		name        string
		target      string
		contentType string
		extension   string
		bodyPrefix  string
	}{
		{"csv", "/log-center/export?type=system_logs&format=csv", "text/csv; charset=utf-8", ".csv", "ID,Service,Level"},
		{"pdf", "/log-center/export?type=system_logs&format=pdf", "application/pdf", ".pdf", "%PDF"},
		{"json", "/log-center/export?type=system_logs&format=json", "application/json", ".json", "{\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := performLogExportRequest(t, h, tt.target)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
			}
			if got := recorder.Header().Get("Content-Type"); got != tt.contentType {
				t.Fatalf("content type = %q, want %q", got, tt.contentType)
			}
			if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, tt.extension) {
				t.Fatalf("content disposition = %q, want %s filename", got, tt.extension)
			}
			if body := recorder.Body.String(); !strings.HasPrefix(body, tt.bodyPrefix) {
				t.Fatalf("body prefix = %q, want %q", body[:min(len(body), 32)], tt.bodyPrefix)
			}
		})
	}
}

func TestExportLogCenterRejectsUnknownFormat(t *testing.T) {
	h := setupLogExportTestHandler(t)

	recorder := performLogExportRequest(t, h, "/log-center/export?type=system_logs&format=xlsx")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestExportLogEvidenceBundleFormats(t *testing.T) {
	h := setupLogExportTestHandler(t)

	tests := []struct {
		name       string
		format     string
		entryName  string
		bodyPrefix string
	}{
		{"json", "json", "system_logs.json", "[\n"},
		{"csv", "csv", "system_logs.csv", "ID,Service,Level"},
		{"pdf", "pdf", "system_logs.pdf", "%PDF"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := performLogExportRequest(t, h, "/log-center/evidence-bundle?type=system_logs&format="+tt.format)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body: %s", recorder.Code, http.StatusOK, recorder.Body.String())
			}
			if got := recorder.Header().Get("Content-Type"); got != "application/zip" {
				t.Fatalf("content type = %q, want application/zip", got)
			}
			if got := recorder.Header().Get("Content-Disposition"); !strings.Contains(got, "_"+tt.format+"_") || !strings.Contains(got, ".zip") {
				t.Fatalf("content disposition = %q, want format-specific zip filename", got)
			}

			reader, err := zip.NewReader(bytes.NewReader(recorder.Body.Bytes()), int64(recorder.Body.Len()))
			if err != nil {
				t.Fatalf("open zip: %v", err)
			}
			entry := readZipEntry(t, reader, tt.entryName)
			if !strings.HasPrefix(string(entry), tt.bodyPrefix) {
				t.Fatalf("%s prefix = %q, want %q", tt.entryName, string(entry[:min(len(entry), 32)]), tt.bodyPrefix)
			}
			manifest := string(readZipEntry(t, reader, "manifest.json"))
			if !strings.Contains(manifest, `"format": "`+tt.format+`"`) {
				t.Fatalf("manifest missing format %q:\n%s", tt.format, manifest)
			}
		})
	}
}

func readZipEntry(t *testing.T, reader *zip.Reader, name string) []byte {
	t.Helper()

	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open zip entry %s: %v", name, err)
		}
		defer rc.Close()
		content, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read zip entry %s: %v", name, err)
		}
		return content
	}
	t.Fatalf("zip entry %s not found", name)
	return nil
}
