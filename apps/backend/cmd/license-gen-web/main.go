package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"management-server/services/license"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

var (
	port = flag.String("port", "8089", "Port to run the web server on")
)

// PageData holds data for rendering the HTML template
type PageData struct {
	Result       string
	Error        string
	GeneratedKey string
	MachineID    string
	Count        int
	Years        int
}

func main() {
	flag.Parse()

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/generate", handleGenerate)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	url := fmt.Sprintf("http://localhost:%s", *port)
	fmt.Printf("Starting License Generator Web Interface at %s\n", url)

	// Open browser automatically
	openBrowser(url)

	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, PageData{Count: 10, Years: 1})
}

func handleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	mode := r.FormValue("mode") // "subscription" or "buyout"
	machineID := strings.TrimSpace(r.FormValue("machine_id"))
	licenseType := r.FormValue("type") // "device", "alert", "combined"
	countStr := r.FormValue("count")
	yearsStr := r.FormValue("years")

	data := PageData{
		MachineID: machineID,
	}

	// Basic Validation
	if machineID == "" {
		data.Error = "請輸入客戶機器碼 (Machine ID)"
		renderTemplate(w, data)
		return
	}

	// Parse numeric inputs
	count := 0
	fmt.Sscanf(countStr, "%d", &count)
	data.Count = count

	years := 0
	fmt.Sscanf(yearsStr, "%d", &years)
	data.Years = years

	// Logic Determination
	secretKey := license.DeriveKey("NMS-LICENSE-" + strings.ToLower(machineID))
	var features []string
	var validUntil string
	var deviceCount int

	// Common Feature Sets
	alertFeatures := []string{"email", "line", "telegram", "whatsapp", "discord", "slack"}
	basicFeatures := []string{"email"}

	if mode == "buyout" {
		// --- Buyout Mode (Ori-New) ---
		// Always Permanent
		validUntil = ""

		switch licenseType {
		case "device":
			if count <= 0 {
				data.Error = "買斷設備授權: 數量必須大於 0"
				renderTemplate(w, data)
				return
			}
			features = basicFeatures
			deviceCount = count
			data.Result = fmt.Sprintf("買斷版 (永久) - 設備授權 (%d 台)", count)

		case "alert":
			// Alert Buyout: Permanent Features, 0 Devices
			features = alertFeatures
			deviceCount = 0
			data.Result = "買斷版 (永久) - 告警授權 (僅啟用告警功能)"

		case "combined":
			if count <= 0 {
				data.Error = "買斷混合授權: 數量必須大於 0"
				renderTemplate(w, data)
				return
			}
			features = alertFeatures
			deviceCount = count
			data.Result = fmt.Sprintf("買斷版 (永久) - 混合授權 (%d 台 + 告警)", count)
		}

	} else {
		// --- Subscription Mode (Original) ---
		if years < 1 || years > 10 {
			data.Error = "訂閱模式年數必須介於 1~10 年"
			renderTemplate(w, data)
			return
		}

		validUntil = time.Now().AddDate(years, 0, 0).Format("2006-01-02")

		switch licenseType {
		case "device":
			if count <= 0 {
				data.Error = "訂閱設備授權: 數量必須大於 0"
				renderTemplate(w, data)
				return
			}
			features = basicFeatures
			deviceCount = count
			data.Result = fmt.Sprintf("訂閱版 (%d 年) - 設備授權 (%d 台)", years, count)

		case "alert":
			// Original Alert logic (CLI allowed partial years, keeping consistent)
			features = alertFeatures
			deviceCount = 0
			data.Result = fmt.Sprintf("訂閱版 (%d 年) - 告警授權", years)

		case "combined":
			if count <= 0 {
				data.Error = "訂閱混合授權: 數量必須大於 0"
				renderTemplate(w, data)
				return
			}
			features = alertFeatures
			deviceCount = count
			data.Result = fmt.Sprintf("訂閱版 (%d 年) - 混合授權 (%d 台 + 告警)", years, count)
		}
	}

	// Generate Key
	key, err := license.GenerateLicenseKey(
		strings.ToLower(machineID),
		deviceCount,
		features,
		validUntil,
		secretKey,
	)

	if err != nil {
		data.Error = "?��?失�?: " + err.Error()
	} else {
		data.GeneratedKey = key
	}

	renderTemplate(w, data)
}

func renderTemplate(w http.ResponseWriter, data PageData) {
	t, err := template.New("index").Parse(htmlTemplate)
	if err != nil {
		http.Error(w, "Template Error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	t.Execute(w, data)
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
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Printf("Please open your web browser and visit: %s\n", url)
	}
}

const htmlTemplate = `
<!DOCTYPE html>
<html lang="zh-Hant">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>NMS ?��??��???/title>
    <style>
        body { font-family: "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background-color: #f4f4f9; display: flex; justify-content: center; padding-top: 50px; }
        .container { background: white; padding: 2rem; border-radius: 12px; box-shadow: 0 4px 6px rgba(0,0,0,0.1); width: 1000px; max-width: 900px; }
        h1 { color: #333; text-align: center; margin-bottom: 1.5rem; }
        .tabs { display: flex; border-bottom: 2px solid #ddd; margin-bottom: 1.5rem; }
        .tab { padding: 10px 20px; cursor: pointer; border-bottom: 3px solid transparent; font-weight: bold; color: #666; }
        .tab.active { border-bottom-color: #007bff; color: #007bff; }
        
        .form-group { margin-bottom: 1rem; }
        label { display: block; margin-bottom: 0.5rem; color: #555; font-weight: 500;}
        input[type="text"], input[type="number"], select { width: 100%; padding: 0.75rem; border: 1px solid #ddd; border-radius: 6px; box-sizing: border-box; font-size: 1rem; }
        
        .btn { display: block; width: 100%; padding: 1rem; background: #007bff; color: white; border: none; border-radius: 6px; font-size: 1.1rem; cursor: pointer; transition: background 0.2s; margin-top: 1.5rem; }
        .btn:hover { background: #0056b3; }
        
        .result-box { margin-top: 2rem; background: #e9ecef; padding: 1.5rem; border-radius: 8px; border-left: 5px solid #28a745; word-break: break-all; }
        .error-box { margin-top: 2rem; background: #f8d7da; color: #721c24; padding: 1rem; border-radius: 8px; border-left: 5px solid #dc3545; }
        
        .key-display { font-family: monospace; font-size: 1.1rem; background: #fff; padding: 1rem; border: 1px solid #ced4da; border-radius: 4px; margin-top: 0.5rem; }
        
        .hidden { display: none; }
        .desc { font-size: 0.9rem; color: #666; margin-top: 0.25rem; }
    </style>
    <script>
        function setMode(mode) {
            document.getElementById('input_mode').value = mode;
            
            document.querySelectorAll('.tab').forEach(el => el.classList.remove('active'));
            document.getElementById('tab_' + mode).classList.add('active');

            const yearGroup = document.getElementById('group_years');
            if (mode === 'buyout') {
                yearGroup.classList.add('hidden');
            } else {
                yearGroup.classList.remove('hidden');
            }
        }
        
        function onTypeChange() {
            const type = document.getElementById('input_type').value;
            const countGroup = document.getElementById('group_count');
            
            if (type === 'alert') {
                countGroup.classList.add('hidden');
            } else {
                countGroup.classList.remove('hidden');
            }
        }
        
        function init() {
            // Restore state if posted
            const urlParams = new URLSearchParams(window.location.search);
            // Simple logic: default to buyout if no post or whatever, handled by server template rendering mostly but needed for dynamic UI
        }
    </script>
</head>
<body onload="onTypeChange()">
    <div class="container">
        <h1>NMS ?��??��???/h1>
        
        <div class="tabs">
            <div id="tab_buyout" class="tab active" onclick="setMode('buyout')">Ori-New (買斷??</div>
            <div id="tab_subscription" class="tab" onclick="setMode('subscription')">Original (訂閱??</div>
        </div>

        <form method="POST" action="/generate">
            <input type="hidden" id="input_mode" name="mode" value="buyout">
            
            <div class="form-group">
                <label>機器識別�?(Machine ID)</label>
                <input type="text" name="machine_id" value="{{.MachineID}}" placeholder="請輸入客戶伺服器的 Machine ID" required>
            </div>

            <div class="form-group">
                <label>?��?類�? (License Type)</label>
                <select id="input_type" name="type" onchange="onTypeChange()">
                    <option value="device">設�??��? (增�?管�??��?)</option>
                    <option value="alert">?�警?��? (?��? Line/TG 等通知)</option>
                    <option value="combined">混�??��? (設�? + ?�警)</option>
                </select>
            </div>

            <div class="form-group" id="group_count">
                <label>設�??��? (Device Count)</label>
                <input type="number" name="count" value="{{if .Count}}{{.Count}}{{else}}10{{end}}" min="0">
                <div class="desc">請輸?��?增�??�設?��?�?/div>
            </div>

            <div class="form-group hidden" id="group_years">
                <label>?��?年�? (Years)</label>
                <input type="number" name="years" value="{{if .Years}}{{.Years}}{{else}}1{{end}}" min="1" max="10">
                <div class="desc">請輸?��??�年??(1-10�?</div>
            </div>

            <button type="submit" class="btn">?��??��??�鑰</button>
        </form>

        {{if .Error}}
        <div class="error-box">
            <strong>?�誤�?/strong> {{.Error}}
        </div>
        {{end}}

        {{if .GeneratedKey}}
        <div class="result-box">
            <h3>{{.Result}}</h3>
            <div class="desc">請�?製以下�??��?供給客戶�?/div>
            <div class="key-display" onclick="this.select();document.execCommand('copy');alert('已�?製到?�貼�?)">{{.GeneratedKey}}</div>
        </div>
        {{end}}
    </div>
</body>
</html>
`
