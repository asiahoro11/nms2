package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"management-server/cmd/internal/licensegen"
	"management-server/services/license"
)

var port = flag.String("port", "8089", "Port to run the web server on")

type pageData struct {
	MachineID      string
	Mode           string
	LicenseType    string
	DeviceCount    int
	CameraCount    int
	Years          int
	DurationDays   int
	Features       []string
	FeatureOptions []licensegen.FeatureOption
	Result         *licensegen.GenerateResult
	Error          string
}

func main() {
	flag.Parse()

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/generate", handleGenerate)

	url := fmt.Sprintf("http://127.0.0.1:%s", *port)
	fmt.Printf("Starting Management System %s License Generator at %s\n", licensegen.ProductVersion, url)
	openBrowser(url)

	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("failed to start license generator: %v", err)
	}
}

func defaultPageData() pageData {
	return pageData{
		Mode:           license.FormalLicenseMode,
		LicenseType:    "full",
		DeviceCount:    10,
		CameraCount:    4,
		Years:          1,
		DurationDays:   14,
		FeatureOptions: licensegen.FeatureOptions,
	}
}

func handleIndex(w http.ResponseWriter, _ *http.Request) {
	renderPage(w, defaultPageData())
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := defaultPageData()
	data.MachineID = strings.TrimSpace(r.FormValue("machine_id"))
	data.Mode = strings.TrimSpace(r.FormValue("mode"))
	data.LicenseType = strings.TrimSpace(r.FormValue("license_type"))
	data.DeviceCount = parseIntOrDefault(r.FormValue("device_count"), data.DeviceCount)
	data.CameraCount = parseIntOrDefault(r.FormValue("camera_count"), data.CameraCount)
	data.Years = parseIntOrDefault(r.FormValue("years"), data.Years)
	data.DurationDays = parseIntOrDefault(r.FormValue("duration_days"), data.DurationDays)
	if strings.EqualFold(data.LicenseType, "custom") {
		data.Features = licensegen.NormalizeSelectedFeatures(r.Form["features"])
	}

	result, err := licensegen.Generate(licensegen.GenerateInput{
		Mode:         data.Mode,
		MachineID:    data.MachineID,
		LicenseType:  data.LicenseType,
		Features:     data.Features,
		DeviceCount:  data.DeviceCount,
		CameraCount:  data.CameraCount,
		Years:        data.Years,
		DurationDays: data.DurationDays,
	})
	if err != nil {
		data.Error = err.Error()
		renderPage(w, data)
		return
	}

	data.Result = &result
	renderPage(w, data)
}

func renderPage(w http.ResponseWriter, data pageData) {
	if data.FeatureOptions == nil {
		data.FeatureOptions = licensegen.FeatureOptions
	}
	tpl := template.Must(template.New("generator").Funcs(template.FuncMap{
		"containsFeature": licensegen.ContainsFeature,
	}).Parse(pageTemplate))
	if err := tpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func parseIntOrDefault(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

func parseCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		log.Printf("warning: failed to open browser: %v", err)
	}
}

const pageTemplate = `<!DOCTYPE html>
<html lang="zh-Hant">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Management System 授權產生器</title>
    <style>
        body { margin:0; min-height:100vh; padding:32px; font-family:"Microsoft JhengHei", "Segoe UI", Arial, sans-serif; background:#0f172a; color:#e5e7eb; }
        .shell { max-width:960px; margin:0 auto; }
        h1 { margin:0 0 8px; font-size:30px; }
        p { color:#94a3b8; }
        form, .card { border:1px solid #334155; background:#111827; border-radius:10px; padding:22px; margin-top:18px; }
        .grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:16px; }
        .full { grid-column:1/-1; }
        label { display:block; margin-bottom:6px; color:#cbd5e1; font-size:14px; }
        input, select { width:100%; box-sizing:border-box; border:1px solid #334155; border-radius:8px; padding:11px 12px; background:#020617; color:#f8fafc; }
        input[type="checkbox"] { width:auto; }
        button { margin-top:18px; width:100%; border:0; border-radius:8px; padding:13px 16px; background:#38bdf8; color:#082f49; font-weight:700; cursor:pointer; }
        .hint { margin-top:6px; color:#94a3b8; font-size:12px; line-height:1.5; }
        .feature-grid { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:10px; margin-top:10px; }
        .feature-option { display:flex; gap:10px; align-items:flex-start; margin:0; padding:12px; border:1px solid #334155; border-radius:8px; background:#020617; cursor:pointer; }
        .feature-option input { margin-top:3px; flex:0 0 auto; }
        .feature-option-title { display:block; color:#f8fafc; font-weight:700; }
        .feature-option-desc { display:block; margin-top:4px; color:#94a3b8; font-size:12px; line-height:1.45; }
        .custom-disabled { opacity:.48; }
        .custom-disabled .feature-option { cursor:not-allowed; }
        .error { border-color:#ef4444; color:#fecaca; }
        .key { word-break:break-all; font-family:Consolas, monospace; line-height:1.7; background:#020617; border:1px dashed #38bdf8; border-radius:8px; padding:14px; }
        @media (max-width:720px) { body { padding:16px; } .grid, .feature-grid { grid-template-columns:1fr; } }
    </style>
</head>
<body>
    <main class="shell">
        <h1>Management System ` + licensegen.ProductVersion + ` 授權產生器</h1>
        <p>單一工具支援正式授權、PoC、買斷授權與自訂功能授權。所有授權類型都使用與後端一致的功能規則。</p>

        <form method="post" action="/generate">
            <div class="grid">
                <div>
                    <label>授權模式</label>
                    <select name="mode">
                        <option value="formal" {{if eq .Mode "formal"}}selected{{end}}>正式授權</option>
                        <option value="poc" {{if eq .Mode "poc"}}selected{{end}}>PoC 試用授權</option>
                    </select>
                </div>
                <div>
                    <label>授權類型</label>
                    <select name="license_type" id="license-type-select">
                        <option value="device" {{if eq .LicenseType "device"}}selected{{end}}>設備管理</option>
                        <option value="alert" {{if eq .LicenseType "alert"}}selected{{end}}>告警通道</option>
                        <option value="camera" {{if eq .LicenseType "camera"}}selected{{end}}>攝影機檢視</option>
                        <option value="camera_recording" {{if eq .LicenseType "camera_recording"}}selected{{end}}>攝影機錄影</option>
                        <option value="access_control" {{if eq .LicenseType "access_control"}}selected{{end}}>門禁管理</option>
                        <option value="pdu" {{if eq .LicenseType "pdu"}}selected{{end}}>PDU / UPS</option>
                        <option value="iot" {{if eq .LicenseType "iot"}}selected{{end}}>IoT / Modbus</option>
                        <option value="combined" {{if eq .LicenseType "combined"}}selected{{end}}>設備與告警</option>
                        <option value="full" {{if eq .LicenseType "full"}}selected{{end}}>完整功能</option>
                        <option value="custom" {{if eq .LicenseType "custom"}}selected{{end}}>自訂功能</option>
                    </select>
                </div>
                <div class="full">
                    <label>Machine ID</label>
                    <input name="machine_id" value="{{.MachineID}}" placeholder="正式授權必填；PoC 可留空">
                </div>
                <div>
                    <label>授權年限</label>
                    <input type="number" min="1" max="999" name="years" value="{{.Years}}">
                    <div class="hint">正式授權使用。輸入 50 以上代表永久買斷。</div>
                </div>
                <div>
                    <label>PoC 天數</label>
                    <input type="number" min="1" max="3650" name="duration_days" value="{{.DurationDays}}">
                    <div class="hint">PoC 使用，從第一次啟用開始計算。</div>
                </div>
                <div>
                    <label>設備數量</label>
                    <input type="number" min="0" name="device_count" value="{{.DeviceCount}}">
                </div>
                <div>
                    <label>攝影機數量</label>
                    <input type="number" min="0" name="camera_count" value="{{.CameraCount}}">
                </div>
                <div class="full">
                    <label>自訂啟用模組</label>
                    <div class="hint">僅在授權類型選擇「自訂功能」時使用。請直接勾選要啟用的模組，不需要手動輸入功能代碼。</div>
                    <div class="feature-grid" id="custom-feature-grid">
                        {{range .FeatureOptions}}
                        <label class="feature-option">
                            <input type="checkbox" name="features" value="{{.Key}}" {{if containsFeature $.Features .Key}}checked{{end}}>
                            <span>
                                <span class="feature-option-title">{{.Label}}</span>
                                <span class="feature-option-desc">{{.Description}}</span>
                            </span>
                        </label>
                        {{end}}
                    </div>
                </div>
            </div>
            <button type="submit">產生授權</button>
        </form>

        {{if .Error}}
        <div class="card error">{{.Error}}</div>
        {{end}}

        {{if .Result}}
        <div class="card">
            <h2>授權已產生</h2>
            <p>{{.Result.Description}}</p>
            <div class="key">{{.Result.Key}}</div>
        </div>
        {{end}}
    </main>
    <script>
        const licenseTypeSelect = document.getElementById('license-type-select');
        const customFeatureGrid = document.getElementById('custom-feature-grid');
        function syncCustomFeatures() {
            const enabled = licenseTypeSelect && licenseTypeSelect.value === 'custom';
            if (customFeatureGrid) {
                customFeatureGrid.classList.toggle('custom-disabled', !enabled);
                customFeatureGrid.querySelectorAll('input[type="checkbox"]').forEach((input) => {
                    input.disabled = !enabled;
                });
            }
        }
        if (licenseTypeSelect) {
            licenseTypeSelect.addEventListener('change', syncCustomFeatures);
            syncCustomFeatures();
        }
    </script>
</body>
</html>`
