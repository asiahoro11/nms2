package topology

import (
	"fmt"
	"strconv"
	"strings"
)

// LoadGraph preserves the legacy API payload while internally deriving it from
// the canonical graph. This keeps current admin/monitor consumers unchanged
// while making canonical graph the module's source of truth.
func (s *Service) LoadGraph() (Data, error) {
	canonical, err := s.BuildCanonicalGraph()
	if err != nil {
		return Data{}, err
	}
	return projectLegacyData(canonical), nil
}

func projectLegacyData(graph CanonicalGraph) Data {
	positionByNodeID := map[string]CanonicalNodePos{}
	for _, layout := range graph.Layouts {
		for _, pos := range layout.NodePositions {
			positionByNodeID[pos.NodeID] = pos
		}
	}

	nodes := make([]Node, 0, len(graph.Nodes))
	for _, node := range graph.Nodes {
		position := positionByNodeID[node.ID]

		legacyNode := Node{
			ID:           parseCanonicalNumericID(node.ID),
			Name:         firstNonEmptyString(node.DisplayName, node.Name),
			IPAddress:    node.IPAddress,
			DeviceType:   firstNonEmptyString(node.Type, "unknown"),
			IsOnline:     strings.EqualFold(node.Status, "online"),
			PosX:         position.X,
			PosY:         position.Y,
			SnmpVersion:  intFromAttributes(node.Attributes, "snmp_version"),
			IsNameCustom: boolFromAttributes(node.Attributes, "is_name_custom"),
		}

		if value := stringFromAttributes(node.Attributes, "image_path"); value != "" {
			legacyNode.ImagePath = &value
		}
		if value := stringFromAttributes(node.Attributes, "sys_name"); value != "" {
			legacyNode.SysName = &value
		}
		if value := stringFromAttributes(node.Attributes, "sys_uptime"); value != "" {
			legacyNode.SysUptime = &value
		}
		if value := stringFromAttributes(node.Attributes, "sys_location"); value != "" {
			legacyNode.SysLocation = &value
		}

		nodes = append(nodes, legacyNode)
	}

	links := make([]Link, 0, len(graph.Links))
	for _, link := range graph.Links {
		legacyLink := Link{
			ID:             parseCanonicalNumericID(link.ID),
			Source:         parseCanonicalNumericID(link.SourceNodeID),
			Target:         parseCanonicalNumericID(link.TargetNodeID),
			LinkSpeed:      link.SpeedLimit,
			BandwidthUsage: int64FromAttributes(link.Attributes, "bandwidth_usage"),
			BandwidthIn:    link.TrafficInBps,
			BandwidthOut:   link.TrafficOutBps,
			LinkType:       firstNonEmptyString(link.LinkType, "auto"),
			IsManual:       boolFromAttributes(link.Attributes, "is_manual"),
		}

		if value := stringFromAttributes(link.Attributes, "label"); value != "" {
			legacyLink.LinkLabel = &value
		}
		if value := stringFromAttributes(link.Attributes, "source_if_name"); value != "" {
			legacyLink.SourceIfName = &value
		}
		if value := stringFromAttributes(link.Attributes, "target_if_name"); value != "" {
			legacyLink.TargetIfName = &value
		}

		if value := intPtrFromAttribute(link.Attributes, "source_if_id"); value != nil && *value > 0 {
			legacyLink.SourceIfID = value
		}
		if value := intPtrFromAttribute(link.Attributes, "target_if_id"); value != nil && *value > 0 {
			legacyLink.TargetIfID = value
		}

		links = append(links, legacyLink)
	}

	return Data{Nodes: nodes, Links: links}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseCanonicalNumericID(id string) int {
	parts := strings.Split(id, ":")
	raw := parts[len(parts)-1]
	value, _ := strconv.Atoi(raw)
	return value
}

func stringFromAttributes(attributes map[string]any, key string) string {
	if attributes == nil {
		return ""
	}
	value, ok := attributes[key]
	if !ok || value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func intFromAttributes(attributes map[string]any, key string) int {
	if attributes == nil {
		return 0
	}
	value, ok := attributes[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case bool:
		if v {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func int64FromAttributes(attributes map[string]any, key string) int64 {
	if attributes == nil {
		return 0
	}
	value, ok := attributes[key]
	if !ok || value == nil {
		return 0
	}
	switch v := value.(type) {
	case int:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	default:
		return 0
	}
}

func boolFromAttributes(attributes map[string]any, key string) bool {
	if attributes == nil {
		return false
	}
	value, ok := attributes[key]
	if !ok || value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

func intPtrFromAttribute(attributes map[string]any, key string) *int {
	value := intFromAttributes(attributes, key)
	if value == 0 {
		return nil
	}
	result := value
	return &result
}

func derefInt(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
