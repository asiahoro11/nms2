// Made by YTSworks
// YTS工作室製作
package topology

import "fmt"

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	case []byte:
		return string(s)
	default:
		return fmt.Sprintf("%v", s)
	}
}

func toInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch i := v.(type) {
	case int64:
		return i
	case int32:
		return int64(i)
	case int:
		return int64(i)
	case bool:
		if i {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func toFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch f := v.(type) {
	case float64:
		return f
	case float32:
		return float64(f)
	case int64:
		return float64(f)
	case int32:
		return float64(f)
	case int:
		return float64(f)
	default:
		return 0
	}
}

func formatSpeed(bps int64) string {
	if bps >= 1000000000000 {
		return fmt.Sprintf("%.0f Tbps", float64(bps)/1000000000000)
	}
	if bps >= 1000000000 {
		return fmt.Sprintf("%.0f Gbps", float64(bps)/1000000000)
	}
	if bps >= 1000000 {
		return fmt.Sprintf("%.0f Mbps", float64(bps)/1000000)
	}
	if bps >= 1000 {
		return fmt.Sprintf("%.0f Kbps", float64(bps)/1000)
	}
	return fmt.Sprintf("%d bps", bps)
}
