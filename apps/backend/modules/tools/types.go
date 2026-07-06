// Made by YTSworks
// YTS工作室製作
package tools

type PingRequest struct {
	Targets []string `json:"targets" binding:"required"`
}

type TracerouteRequest struct {
	Target string `json:"target" binding:"required"`
}

type Response struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
}
