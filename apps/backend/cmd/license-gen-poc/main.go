// Made by YTSworks
// YTS工作室製作
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

const (
	productVersion = licensegen.ProductVersion
)

var port = flag.String("port", "8092", "Port to run the PoC license generator on")

type featureOption = licensegen.FeatureOption

var generatorFeatures = licensegen.FeatureOptions

type pageData struct {
	ActiveTab string

	FeatureOptions []featureOption

	FormalMachineID   string
	FormalDeviceCount int
	FormalCameraCount int
	FormalYears       int
	FormalFeatures    []string

	PoCDeviceCount  int
	PoCCameraCount  int
	PoCDurationDays int
	PoCFeatures     []string

	ResultTitle       string
	ResultDescription string
	GeneratedKey      string
	ErrorMessage      string
}

func main() {
	flag.Parse()

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/generate/formal", handleFormalGenerate)
	http.HandleFunc("/generate/poc", handlePoCGenerate)

	url := fmt.Sprintf("http://127.0.0.1:%s", *port)
	fmt.Printf("Starting Management System %s License Generator at %s\n", productVersion, url)
	openBrowser(url)

	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("failed to start generator: %v", err)
	}
}

func defaultPageData() pageData {
	return pageData{
		ActiveTab:         "formal",
		FeatureOptions:    generatorFeatures,
		FormalDeviceCount: 10,
		FormalCameraCount: 4,
		FormalYears:       1,
		FormalFeatures:    []string{"device_management"},
		PoCDeviceCount:    10,
		PoCCameraCount:    4,
		PoCDurationDays:   14,
		PoCFeatures:       []string{"device_management"},
	}
}

func handleIndex(w http.ResponseWriter, _ *http.Request) {
	renderPage(w, defaultPageData())
}

func handleFormalGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := defaultPageData()
	data.ActiveTab = "formal"
	data.FormalMachineID = strings.TrimSpace(r.FormValue("machine_id"))
	data.FormalDeviceCount = parseIntOrDefault(r.FormValue("device_count"), data.FormalDeviceCount)
	data.FormalCameraCount = parseIntOrDefault(r.FormValue("camera_count"), data.FormalCameraCount)
	data.FormalYears = parseIntOrDefault(r.FormValue("years"), data.FormalYears)
	data.FormalFeatures = normalizeSelectedFeatures(r.Form["features"])
	data.PoCFeatures = defaultPageData().PoCFeatures

	if data.FormalMachineID == "" {
		data.ErrorMessage = "machine_id is required for formal licenses"
		renderPage(w, data)
		return
	}

	key, description, err := generateLicense(
		license.FormalLicenseMode,
		data.FormalMachineID,
		data.FormalFeatures,
		data.FormalDeviceCount,
		data.FormalCameraCount,
		data.FormalYears,
		0,
	)
	if err != nil {
		data.ErrorMessage = err.Error()
		renderPage(w, data)
		return
	}

	data.ResultTitle = "Formal license generated"
	data.ResultDescription = description
	data.GeneratedKey = key
	renderPage(w, data)
}

func handlePoCGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := defaultPageData()
	data.ActiveTab = "poc"
	data.PoCDeviceCount = parseIntOrDefault(r.FormValue("device_count"), data.PoCDeviceCount)
	data.PoCCameraCount = parseIntOrDefault(r.FormValue("camera_count"), data.PoCCameraCount)
	data.PoCDurationDays = parseIntOrDefault(r.FormValue("duration_days"), 0)
	data.PoCFeatures = normalizeSelectedFeatures(r.Form["features"])
	data.FormalFeatures = defaultPageData().FormalFeatures

	if err := license.ValidatePoCDurationDays(data.PoCDurationDays); err != nil {
		data.ErrorMessage = err.Error()
		renderPage(w, data)
		return
	}

	key, description, err := generateLicense(
		license.PoCLicenseMode,
		"",
		data.PoCFeatures,
		data.PoCDeviceCount,
		data.PoCCameraCount,
		0,
		data.PoCDurationDays,
	)
	if err != nil {
		data.ErrorMessage = err.Error()
		renderPage(w, data)
		return
	}

	data.ResultTitle = "PoC license generated"
	data.ResultDescription = description
	data.GeneratedKey = key
	renderPage(w, data)
}

func normalizeSelectedFeatures(raw []string) []string {
	return licensegen.NormalizeSelectedFeatures(raw)
}

func containsFeature(slice []string, item string) bool {
	return licensegen.ContainsFeature(slice, item)
}

func generateLicense(mode, machineID string, selectedFeatures []string, deviceCount, cameraCount, years, durationDays int) (string, string, error) {
	result, err := licensegen.Generate(licensegen.GenerateInput{
		Mode:         mode,
		MachineID:    machineID,
		Features:     selectedFeatures,
		DeviceCount:  deviceCount,
		CameraCount:  cameraCount,
		Years:        years,
		DurationDays: durationDays,
	})
	if err != nil {
		return "", "", err
	}
	return result.Key, result.Description, nil
}

func parseIntOrDefault(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

func renderPage(w http.ResponseWriter, data pageData) {
	tpl := template.Must(template.New("generator").Funcs(template.FuncMap{
		"containsFeature": containsFeature,
	}).Parse(pageTemplate))
	if err := tpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
    <title>Management System ` + productVersion + ` License Generator</title>
    <style>
        :root {
            --bg: #09101d;
            --panel: #111a2b;
            --panel-2: #18243a;
            --border: rgba(255, 255, 255, 0.10);
            --text: #edf2ff;
            --muted: #99a9cc;
            --accent: #5fb3ff;
            --accent-2: #ff9d57;
            --danger: #ff7676;
        }
        * { box-sizing: border-box; }
        body {
            margin: 0;
            min-height: 100vh;
            padding: 28px;
            color: var(--text);
            background:
                radial-gradient(circle at top left, rgba(95, 179, 255, 0.20), transparent 32%),
                radial-gradient(circle at bottom right, rgba(255, 157, 87, 0.16), transparent 28%),
                linear-gradient(180deg, #08111f 0%, #09101d 100%);
            font-family: "Segoe UI", "Microsoft JhengHei", sans-serif;
        }
        .shell {
            max-width: 1240px;
            margin: 0 auto;
            border-radius: 28px;
            overflow: hidden;
            border: 1px solid rgba(255,255,255,0.08);
            background: rgba(8, 14, 24, 0.94);
            box-shadow: 0 30px 90px rgba(0,0,0,0.45);
        }
        .hero {
            padding: 36px 40px 24px;
            border-bottom: 1px solid rgba(255,255,255,0.08);
            background: linear-gradient(135deg, rgba(95, 179, 255, 0.18), rgba(255, 157, 87, 0.10));
        }
        .hero h1 {
            margin: 0 0 12px;
            font-size: 34px;
            line-height: 1.15;
        }
        .hero p {
            margin: 0;
            color: var(--muted);
            line-height: 1.7;
            max-width: 920px;
        }
        .content {
            display: grid;
            grid-template-columns: 1.15fr 0.85fr;
        }
        .forms {
            padding: 28px 32px 32px;
        }
        .side {
            padding: 28px 32px 32px;
            border-left: 1px solid rgba(255,255,255,0.08);
            background: rgba(17, 26, 43, 0.54);
        }
        .tabs {
            display: inline-flex;
            gap: 8px;
            padding: 6px;
            border: 1px solid rgba(255,255,255,0.08);
            border-radius: 999px;
            background: rgba(0,0,0,0.22);
            margin-bottom: 24px;
        }
        .tab {
            border: 0;
            background: transparent;
            color: var(--muted);
            padding: 10px 18px;
            border-radius: 999px;
            cursor: pointer;
            font-weight: 700;
        }
        .tab.active {
            background: var(--accent);
            color: white;
        }
        .panel { display: none; }
        .panel.active { display: block; }
        .grid {
            display: grid;
            grid-template-columns: repeat(2, minmax(0, 1fr));
            gap: 16px;
        }
        .full { grid-column: 1 / -1; }
        label {
            display: block;
            margin-bottom: 8px;
            color: var(--muted);
            font-size: 14px;
        }
        input, button {
            width: 100%;
            border-radius: 14px;
            border: 1px solid var(--border);
            background: var(--panel);
            color: var(--text);
            padding: 14px 16px;
            font-size: 15px;
        }
        input:focus {
            outline: none;
            border-color: rgba(95, 179, 255, 0.85);
            box-shadow: 0 0 0 3px rgba(95, 179, 255, 0.18);
        }
        .hint {
            margin-top: 8px;
            color: var(--muted);
            font-size: 12px;
            line-height: 1.6;
        }
        .submit {
            background: linear-gradient(135deg, var(--accent), #5bd4f5);
            border: 0;
            font-weight: 700;
            margin-top: 20px;
            cursor: pointer;
        }
        .submit.poc {
            background: linear-gradient(135deg, var(--accent-2), #ff5f7b);
        }
        .feature-grid {
            display: grid;
            grid-template-columns: repeat(2, minmax(0, 1fr));
            gap: 12px;
        }
        .feature-option {
            display: flex;
            gap: 12px;
            align-items: flex-start;
            margin: 0;
            padding: 14px;
            border-radius: 16px;
            border: 1px solid rgba(255,255,255,0.08);
            background: rgba(255,255,255,0.03);
            cursor: pointer;
        }
        .feature-option input[type="checkbox"] {
            width: 18px;
            min-width: 18px;
            height: 18px;
            margin: 2px 0 0;
            padding: 0;
            accent-color: #5fb3ff;
        }
        .feature-option-body {
            display: flex;
            flex-direction: column;
            gap: 4px;
        }
        .feature-option-title {
            font-size: 14px;
            font-weight: 700;
            color: var(--text);
        }
        .feature-option-desc {
            font-size: 12px;
            color: var(--muted);
            line-height: 1.5;
        }
        .card {
            background: var(--panel);
            border: 1px solid rgba(255,255,255,0.08);
            border-radius: 18px;
            padding: 20px;
            margin-bottom: 18px;
        }
        .card h2 {
            margin: 0 0 10px;
            font-size: 18px;
        }
        .error {
            border-color: rgba(255,107,107,0.35);
            background: rgba(120, 32, 32, 0.18);
            color: #ffd6d6;
        }
        .keybox {
            background: #0c1220;
            border: 1px dashed rgba(95, 179, 255, 0.45);
            border-radius: 14px;
            padding: 16px;
            font-family: Consolas, monospace;
            word-break: break-all;
            line-height: 1.7;
        }
        .pill {
            display: inline-flex;
            align-items: center;
            gap: 8px;
            border-radius: 999px;
            padding: 8px 12px;
            background: rgba(95, 179, 255, 0.12);
            color: #bbe0ff;
            font-size: 12px;
            font-weight: 700;
            text-transform: uppercase;
            letter-spacing: 0.06em;
        }
        ul {
            margin: 0;
            padding-left: 18px;
            color: var(--muted);
            line-height: 1.7;
        }
        @media (max-width: 960px) {
            body { padding: 16px; }
            .content { grid-template-columns: 1fr; }
            .side {
                border-left: 0;
                border-top: 1px solid rgba(255,255,255,0.08);
            }
            .grid, .feature-grid { grid-template-columns: 1fr; }
        }
    </style>
    <script>
        function switchTab(tab) {
            document.querySelectorAll('.tab').forEach(function(node) {
                node.classList.remove('active');
            });
            document.querySelectorAll('.panel').forEach(function(node) {
                node.classList.remove('active');
            });
            document.querySelector('[data-tab="' + tab + '"]').classList.add('active');
            document.getElementById('panel-' + tab).classList.add('active');
        }
    </script>
</head>
<body>
    <div class="shell">
        <div class="hero">
            <h1>Management System ` + productVersion + ` License Generator</h1>
            <p>Choose features by checkbox so formal and PoC licenses can be combined freely. Formal licenses stay UUID-bound. PoC licenses start counting down only after the customer activates them.</p>
        </div>
        <div class="content">
            <div class="forms">
                <div class="tabs">
                    <button class="tab {{if eq .ActiveTab "formal"}}active{{end}}" data-tab="formal" type="button" onclick="switchTab('formal')">Formal</button>
                    <button class="tab {{if eq .ActiveTab "poc"}}active{{end}}" data-tab="poc" type="button" onclick="switchTab('poc')">PoC</button>
                </div>

                <div id="panel-formal" class="panel {{if eq .ActiveTab "formal"}}active{{end}}">
                    <form method="post" action="/generate/formal">
                        <div class="grid">
                            <div class="full">
                                <label>Machine ID</label>
                                <input name="machine_id" value="{{.FormalMachineID}}" placeholder="Paste the target system UUID / machine ID" required>
                            </div>
                            <div>
                                <label>Years</label>
                                <input type="number" min="1" max="999" name="years" value="{{.FormalYears}}">
                                <div class="hint">Use 1-5 for annual licenses. Use 50 or above for permanent.</div>
                            </div>
                            <div>
                                <label>Device Count</label>
                                <input type="number" min="0" name="device_count" value="{{.FormalDeviceCount}}">
                                <div class="hint">Required only when Device Management is selected.</div>
                            </div>
                            <div>
                                <label>Camera Count</label>
                                <input type="number" min="0" name="camera_count" value="{{.FormalCameraCount}}">
                                <div class="hint">Required when Camera Viewer or Camera Recording is selected.</div>
                            </div>
                            <div class="full">
                                <label>Features</label>
                                <div class="feature-grid">
                                    {{range .FeatureOptions}}
                                    <label class="feature-option">
                                        <input type="checkbox" name="features" value="{{.Key}}" {{if containsFeature $.FormalFeatures .Key}}checked{{end}}>
                                        <span class="feature-option-body">
                                            <span class="feature-option-title">{{.Label}}</span>
                                            <span class="feature-option-desc">{{.Description}}</span>
                                        </span>
                                    </label>
                                    {{end}}
                                </div>
                            </div>
                        </div>
                        <button class="submit" type="submit">Generate Formal License</button>
                    </form>
                </div>

                <div id="panel-poc" class="panel {{if eq .ActiveTab "poc"}}active{{end}}">
                    <form method="post" action="/generate/poc">
                        <div class="grid">
                            <div>
                                <label>Duration Days</label>
                                <input type="number" min="1" max="3650" name="duration_days" value="{{.PoCDurationDays}}" required>
                                <div class="hint">The PoC timer starts when the customer activates this key, not when you generate it.</div>
                            </div>
                            <div>
                                <label>Device Count</label>
                                <input type="number" min="0" name="device_count" value="{{.PoCDeviceCount}}">
                                <div class="hint">Required only when Device Management is selected.</div>
                            </div>
                            <div>
                                <label>Camera Count</label>
                                <input type="number" min="0" name="camera_count" value="{{.PoCCameraCount}}">
                                <div class="hint">Required when Camera Viewer or Camera Recording is selected.</div>
                            </div>
                            <div class="full">
                                <label>Features</label>
                                <div class="feature-grid">
                                    {{range .FeatureOptions}}
                                    <label class="feature-option">
                                        <input type="checkbox" name="features" value="{{.Key}}" {{if containsFeature $.PoCFeatures .Key}}checked{{end}}>
                                        <span class="feature-option-body">
                                            <span class="feature-option-title">{{.Label}}</span>
                                            <span class="feature-option-desc">{{.Description}}</span>
                                        </span>
                                    </label>
                                    {{end}}
                                </div>
                            </div>
                        </div>
                        <button class="submit poc" type="submit">Generate PoC License</button>
                    </form>
                </div>
            </div>

            <aside class="side">
                <div class="card">
                    <span class="pill">` + productVersion + `</span>
                    <h2>Rules</h2>
                    <ul>
                        <li>Formal licenses are bound to the machine UUID that you paste into the form.</li>
                        <li>PoC licenses are generated without a fixed expiry timestamp inside the key.</li>
                        <li>PoC expiry is written to the system database on first activation and then enforced from that point.</li>
                        <li>If the same PoC key is activated again on the same system before expiry, the original countdown is preserved.</li>
                        <li>Device count applies only to ` + "`device_management`" + `, and camera count applies to ` + "`camera_viewer`" + ` or ` + "`camera_recording`" + `.</li>
                    </ul>
                </div>

                {{if .ErrorMessage}}
                <div class="card error">
                    <h2>Error</h2>
                    <div>{{.ErrorMessage}}</div>
                </div>
                {{end}}

                {{if .GeneratedKey}}
                <div class="card">
                    <h2>{{.ResultTitle}}</h2>
                    <div class="hint" style="margin-bottom: 12px;">{{.ResultDescription}}</div>
                    <div class="keybox">{{.GeneratedKey}}</div>
                </div>
                {{end}}
            </aside>
        </div>
    </div>
</body>
</html>`
