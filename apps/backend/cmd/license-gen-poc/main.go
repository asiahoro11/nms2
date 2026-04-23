package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"management-server/services/license"
)

const (
	formalSecretSeed = "NMS-LICENSE-"
	pocSecretSeed    = "NMS-POC-LICENSE-v1.2.1-PoC"
)

var port = flag.String("port", "8092", "Port to run the PoC license generator on")

type pageData struct {
	ActiveTab string

	FormalMachineID   string
	FormalType        string
	FormalDeviceCount int
	FormalCameraCount int
	FormalYears       int

	PoCType        string
	PoCDeviceCount int
	PoCCameraCount int
	PoCValidUntil  string

	ResultTitle       string
	ResultDescription string
	GeneratedKey      string
	ErrorMessage      string
}

type licenseProfile struct {
	Features    []string
	DeviceCount int
	CameraCount int
	Label       string
}

func main() {
	flag.Parse()

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/generate/formal", handleFormalGenerate)
	http.HandleFunc("/generate/poc", handlePoCGenerate)

	url := fmt.Sprintf("http://127.0.0.1:%s", *port)
	fmt.Printf("Starting Management System v1.2.1-PoC License Generator at %s\n", url)
	openBrowser(url)

	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("failed to start generator: %v", err)
	}
}

func defaultPageData() pageData {
	return pageData{
		ActiveTab:         "formal",
		FormalType:        "device",
		FormalDeviceCount: 10,
		FormalCameraCount: 4,
		FormalYears:       1,
		PoCType:           "device",
		PoCDeviceCount:    10,
		PoCCameraCount:    4,
		PoCValidUntil:     time.Now().Add(72 * time.Hour).Format("2006-01-02T15:04"),
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
	data.FormalType = strings.TrimSpace(r.FormValue("license_type"))
	fmt.Sscanf(r.FormValue("device_count"), "%d", &data.FormalDeviceCount)
	fmt.Sscanf(r.FormValue("camera_count"), "%d", &data.FormalCameraCount)
	fmt.Sscanf(r.FormValue("years"), "%d", &data.FormalYears)

	if data.FormalMachineID == "" {
		data.ErrorMessage = "正式授權必須填入 Machine ID。"
		renderPage(w, data)
		return
	}

	key, description, err := generateLicense(
		license.FormalLicenseMode,
		data.FormalMachineID,
		data.FormalType,
		data.FormalDeviceCount,
		data.FormalCameraCount,
		data.FormalYears,
		"",
	)
	if err != nil {
		data.ErrorMessage = err.Error()
		renderPage(w, data)
		return
	}

	data.ResultTitle = "正式授權產生完成"
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
	data.PoCType = strings.TrimSpace(r.FormValue("license_type"))
	fmt.Sscanf(r.FormValue("device_count"), "%d", &data.PoCDeviceCount)
	fmt.Sscanf(r.FormValue("camera_count"), "%d", &data.PoCCameraCount)
	data.PoCValidUntil = strings.TrimSpace(r.FormValue("valid_until"))

	if data.PoCValidUntil == "" {
		data.ErrorMessage = "PoC 授權必須指定到期時間。"
		renderPage(w, data)
		return
	}

	key, description, err := generateLicense(
		license.PoCLicenseMode,
		"",
		data.PoCType,
		data.PoCDeviceCount,
		data.PoCCameraCount,
		0,
		data.PoCValidUntil,
	)
	if err != nil {
		data.ErrorMessage = err.Error()
		renderPage(w, data)
		return
	}

	data.ResultTitle = "PoC 授權產生完成"
	data.ResultDescription = description
	data.GeneratedKey = key
	renderPage(w, data)
}

func generateLicense(mode, machineID, licenseType string, deviceCount, cameraCount, years int, validUntil string) (string, string, error) {
	licenseType = strings.ToLower(strings.TrimSpace(licenseType))
	if licenseType == "" {
		licenseType = "device"
	}

	if mode == license.FormalLicenseMode && years < 1 {
		years = 1
	}

	profile, err := resolveLicenseType(licenseType, deviceCount, cameraCount)
	if err != nil {
		return "", "", err
	}

	if mode == license.PoCLicenseMode {
		if _, err := license.ParseLicenseTime(validUntil); err != nil {
			return "", "", fmt.Errorf("PoC 到期時間格式無效，請使用合法日期時間")
		}
	} else {
		if license.IsPermanentYears(years) {
			validUntil = ""
		} else {
			validUntil = time.Now().AddDate(years, 0, 0).Format("2006-01-02")
		}
	}

	secretKey := license.DeriveKey(formalSecretSeed + strings.ToLower(strings.TrimSpace(machineID)))
	if mode == license.PoCLicenseMode {
		secretKey = license.DeriveKey(pocSecretSeed)
	}

	key, err := license.GenerateLicenseKeyAdvanced(
		mode,
		machineID,
		profile.DeviceCount,
		profile.CameraCount,
		profile.Features,
		validUntil,
		secretKey,
	)
	if err != nil {
		return "", "", err
	}

	modeLabel := "正式授權"
	if mode == license.PoCLicenseMode {
		modeLabel = "PoC 授權"
	}

	description := fmt.Sprintf(
		"%s | 類型：%s | 設備數：%d | 攝影機數：%d | 到期：%s",
		modeLabel,
		profile.Label,
		profile.DeviceCount,
		profile.CameraCount,
		displayExpiry(validUntil),
	)
	return key, description, nil
}

func resolveLicenseType(licenseType string, deviceCount, cameraCount int) (licenseProfile, error) {
	switch licenseType {
	case "device":
		if deviceCount <= 0 {
			return licenseProfile{}, fmt.Errorf("設備管理授權的設備數量必須大於 0")
		}
		return licenseProfile{
			Features:    []string{"device_management"},
			DeviceCount: deviceCount,
			Label:       "設備管理",
		}, nil
	case "alert":
		return licenseProfile{
			Features: []string{"line", "telegram", "whatsapp", "discord", "slack"},
			Label:    "告警通報",
		}, nil
	case "camera":
		if cameraCount <= 0 {
			return licenseProfile{}, fmt.Errorf("攝影機授權的攝影機數量必須大於 0")
		}
		return licenseProfile{
			Features:    []string{"camera_viewer"},
			CameraCount: cameraCount,
			Label:       "攝影機監控",
		}, nil
	case "access_control":
		return licenseProfile{
			Features: []string{"access_control"},
			Label:    "門禁管理",
		}, nil
	case "pdu":
		return licenseProfile{
			Features: []string{"pdu"},
			Label:    "PDU/UPS",
		}, nil
	case "combined":
		if deviceCount <= 0 {
			return licenseProfile{}, fmt.Errorf("設備加告警授權的設備數量必須大於 0")
		}
		return licenseProfile{
			Features:    []string{"device_management", "line", "telegram", "whatsapp", "discord", "slack"},
			DeviceCount: deviceCount,
			Label:       "設備管理 + 告警通報",
		}, nil
	case "full":
		if deviceCount <= 0 {
			return licenseProfile{}, fmt.Errorf("完整授權的設備數量必須大於 0")
		}
		if cameraCount <= 0 {
			return licenseProfile{}, fmt.Errorf("完整授權的攝影機數量必須大於 0")
		}
		return licenseProfile{
			Features:    []string{"device_management", "line", "telegram", "whatsapp", "discord", "slack", "camera_viewer", "access_control", "pdu"},
			DeviceCount: deviceCount,
			CameraCount: cameraCount,
			Label:       "完整授權",
		}, nil
	default:
		return licenseProfile{}, fmt.Errorf("不支援的授權類型：%s", licenseType)
	}
}

func displayExpiry(validUntil string) string {
	if strings.TrimSpace(validUntil) == "" {
		return "永久"
	}
	return validUntil
}

func renderPage(w http.ResponseWriter, data pageData) {
	tpl := template.Must(template.New("generator").Parse(pageTemplate))
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
    <title>Management System v1.2.1-PoC License Generator</title>
    <style>
        :root {
            --bg: #0b1220;
            --panel: #121c2f;
            --panel-2: #1a2740;
            --border: rgba(255, 255, 255, 0.10);
            --text: #e8eefc;
            --muted: #9cacd0;
            --accent: #7c5cff;
            --accent-2: #ff8a4c;
            --danger: #ff6b6b;
        }
        * { box-sizing: border-box; }
        body {
            margin: 0;
            min-height: 100vh;
            padding: 28px;
            background:
                radial-gradient(circle at top left, rgba(124, 92, 255, 0.22), transparent 30%),
                radial-gradient(circle at bottom right, rgba(255, 138, 76, 0.16), transparent 28%),
                linear-gradient(180deg, #09101c 0%, #0b1220 100%);
            color: var(--text);
            font-family: "Segoe UI", "Microsoft JhengHei", sans-serif;
        }
        .shell {
            max-width: 1220px;
            margin: 0 auto;
            border-radius: 28px;
            overflow: hidden;
            border: 1px solid rgba(255,255,255,0.08);
            background: rgba(10, 16, 28, 0.92);
            box-shadow: 0 28px 80px rgba(0,0,0,0.45);
        }
        .hero {
            padding: 36px 40px 24px;
            border-bottom: 1px solid rgba(255,255,255,0.08);
            background: linear-gradient(135deg, rgba(124, 92, 255, 0.18), rgba(255, 138, 76, 0.10));
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
            grid-template-columns: 1.08fr 0.92fr;
        }
        .forms {
            padding: 28px 32px 32px;
        }
        .side {
            padding: 28px 32px 32px;
            border-left: 1px solid rgba(255,255,255,0.08);
            background: rgba(18, 28, 47, 0.46);
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
        input, select, button {
            width: 100%;
            border-radius: 14px;
            border: 1px solid var(--border);
            background: var(--panel);
            color: var(--text);
            padding: 14px 16px;
            font-size: 15px;
        }
        input:focus, select:focus {
            outline: none;
            border-color: rgba(124, 92, 255, 0.8);
            box-shadow: 0 0 0 3px rgba(124, 92, 255, 0.16);
        }
        .hint {
            margin-top: 8px;
            color: var(--muted);
            font-size: 12px;
            line-height: 1.6;
        }
        .submit {
            background: linear-gradient(135deg, var(--accent), #9f6bff);
            border: 0;
            font-weight: 700;
            margin-top: 20px;
            cursor: pointer;
        }
        .submit.poc {
            background: linear-gradient(135deg, var(--accent-2), #ff5f7b);
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
            border: 1px dashed rgba(124,92,255,0.45);
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
            background: rgba(124,92,255,0.12);
            color: #c9bcff;
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
            .grid { grid-template-columns: 1fr; }
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
            <h1>Management System v1.2.1-PoC License Generator</h1>
            <p>正式授權會綁定 Machine ID。PoC 授權不綁定 UUID，可自由指定設備數量、攝影機數量與到期時間，時間到了之後系統會自動鎖定授權能力。</p>
        </div>
        <div class="content">
            <div class="forms">
                <div class="tabs">
                    <button class="tab {{if eq .ActiveTab "formal"}}active{{end}}" data-tab="formal" type="button" onclick="switchTab('formal')">正式授權</button>
                    <button class="tab {{if eq .ActiveTab "poc"}}active{{end}}" data-tab="poc" type="button" onclick="switchTab('poc')">PoC 授權</button>
                </div>

                <div id="panel-formal" class="panel {{if eq .ActiveTab "formal"}}active{{end}}">
                    <form method="post" action="/generate/formal">
                        <div class="grid">
                            <div class="full">
                                <label>Machine ID</label>
                                <input name="machine_id" value="{{.FormalMachineID}}" placeholder="請輸入正式授權要綁定的 Machine ID" required>
                            </div>
                            <div>
                                <label>授權類型</label>
                                <select name="license_type">
                                    <option value="device" {{if eq .FormalType "device"}}selected{{end}}>設備管理</option>
                                    <option value="alert" {{if eq .FormalType "alert"}}selected{{end}}>告警通報</option>
                                    <option value="camera" {{if eq .FormalType "camera"}}selected{{end}}>攝影機監控</option>
                                    <option value="access_control" {{if eq .FormalType "access_control"}}selected{{end}}>門禁管理</option>
                                    <option value="pdu" {{if eq .FormalType "pdu"}}selected{{end}}>PDU/UPS</option>
                                    <option value="combined" {{if eq .FormalType "combined"}}selected{{end}}>設備管理 + 告警通報</option>
                                    <option value="full" {{if eq .FormalType "full"}}selected{{end}}>完整授權</option>
                                </select>
                            </div>
                            <div>
                                <label>年限</label>
                                <input type="number" min="1" max="999" name="years" value="{{.FormalYears}}">
                                <div class="hint">1 到 5 年為一般正式授權；50 年以上自動視為永久授權。</div>
                            </div>
                            <div>
                                <label>設備數量</label>
                                <input type="number" min="0" name="device_count" value="{{.FormalDeviceCount}}">
                            </div>
                            <div>
                                <label>攝影機數量</label>
                                <input type="number" min="0" name="camera_count" value="{{.FormalCameraCount}}">
                            </div>
                        </div>
                        <button class="submit" type="submit">產生正式授權</button>
                    </form>
                </div>

                <div id="panel-poc" class="panel {{if eq .ActiveTab "poc"}}active{{end}}">
                    <form method="post" action="/generate/poc">
                        <div class="grid">
                            <div>
                                <label>授權類型</label>
                                <select name="license_type">
                                    <option value="device" {{if eq .PoCType "device"}}selected{{end}}>設備管理</option>
                                    <option value="alert" {{if eq .PoCType "alert"}}selected{{end}}>告警通報</option>
                                    <option value="camera" {{if eq .PoCType "camera"}}selected{{end}}>攝影機監控</option>
                                    <option value="access_control" {{if eq .PoCType "access_control"}}selected{{end}}>門禁管理</option>
                                    <option value="pdu" {{if eq .PoCType "pdu"}}selected{{end}}>PDU/UPS</option>
                                    <option value="combined" {{if eq .PoCType "combined"}}selected{{end}}>設備管理 + 告警通報</option>
                                    <option value="full" {{if eq .PoCType "full"}}selected{{end}}>完整授權</option>
                                </select>
                            </div>
                            <div>
                                <label>到期時間</label>
                                <input type="datetime-local" name="valid_until" value="{{.PoCValidUntil}}" required>
                                <div class="hint">PoC 授權不需要 Machine ID。時間到後會自動失效。</div>
                            </div>
                            <div>
                                <label>設備數量</label>
                                <input type="number" min="0" name="device_count" value="{{.PoCDeviceCount}}">
                            </div>
                            <div>
                                <label>攝影機數量</label>
                                <input type="number" min="0" name="camera_count" value="{{.PoCCameraCount}}">
                            </div>
                        </div>
                        <button class="submit poc" type="submit">產生 PoC 授權</button>
                    </form>
                </div>
            </div>

            <aside class="side">
                <div class="card">
                    <span class="pill">v1.2.1-PoC</span>
                    <h2>授權規則</h2>
                    <ul>
                        <li>正式授權會綁定 Machine ID，適合正式交付與長期使用。</li>
                        <li>PoC 授權不綁定 UUID，適合展示、驗證與限期測試。</li>
                        <li>PoC 可以自訂設備數量、攝影機數量與到期時間。</li>
                        <li>PoC 到期後，系統會依授權檢查結果自動鎖定功能。</li>
                        <li>v1.2.1-PoC 預設不開放設備管理權限，需透過授權啟用。</li>
                    </ul>
                </div>

                {{if .ErrorMessage}}
                <div class="card error">
                    <h2>產生失敗</h2>
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
