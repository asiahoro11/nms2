package topology

import (
	"fmt"
	"time"
)

func (s *Service) BuildCanonicalGraph() (CanonicalGraph, error) {
	nodes, err := s.loadNodes()
	if err != nil {
		return CanonicalGraph{}, err
	}

	links, err := s.loadLinks()
	if err != nil {
		return CanonicalGraph{}, err
	}

	canonicalNodes := make([]CanonicalNode, 0, len(nodes))
	layoutPositions := make([]CanonicalNodePos, 0, len(nodes))
	for _, node := range nodes {
		status := "offline"
		if node.IsOnline {
			status = "online"
		}

		canonicalNodes = append(canonicalNodes, CanonicalNode{
			ID:          fmt.Sprintf("device:%d", node.ID),
			Name:        node.Name,
			DisplayName: node.Name,
			Type:        node.DeviceType,
			IPAddress:   node.IPAddress,
			Status:      status,
			Source:      "nms_sync",
			ExternalID:  fmt.Sprintf("%d", node.ID),
			Confidence:  1.0,
			Attributes: map[string]any{
				"snmp_version":   node.SnmpVersion,
				"is_name_custom": node.IsNameCustom,
				"image_path":     valueOrEmpty(node.ImagePath),
				"sys_name":       valueOrEmpty(node.SysName),
				"sys_uptime":     valueOrEmpty(node.SysUptime),
				"sys_location":   valueOrEmpty(node.SysLocation),
			},
		})

		layoutPositions = append(layoutPositions, CanonicalNodePos{
			NodeID: fmt.Sprintf("device:%d", node.ID),
			X:      node.PosX,
			Y:      node.PosY,
		})
	}

	canonicalLinks := make([]CanonicalLink, 0, len(links))
	for _, link := range links {
		discoverySource := "auto"
		if link.IsManual {
			discoverySource = "manual"
		}

		sourcePortID := ""
		targetPortID := ""
		if link.SourceIfID != nil {
			sourcePortID = fmt.Sprintf("port:%d", *link.SourceIfID)
		}
		if link.TargetIfID != nil {
			targetPortID = fmt.Sprintf("port:%d", *link.TargetIfID)
		}

		canonicalLinks = append(canonicalLinks, CanonicalLink{
			ID:              fmt.Sprintf("link:%d", link.ID),
			SourceNodeID:    fmt.Sprintf("device:%d", link.Source),
			TargetNodeID:    fmt.Sprintf("device:%d", link.Target),
			SourcePortID:    sourcePortID,
			TargetPortID:    targetPortID,
			LinkType:        link.LinkType,
			Status:          "connected",
			SpeedLimit:      link.LinkSpeed,
			TrafficInBps:    link.BandwidthIn,
			TrafficOutBps:   link.BandwidthOut,
			DiscoverySource: discoverySource,
			Confidence:      1.0,
			Attributes: map[string]any{
				"is_manual":       link.IsManual,
				"bandwidth_usage": link.BandwidthUsage,
				"label":           valueOrEmpty(link.LinkLabel),
				"source_if_name":  valueOrEmpty(link.SourceIfName),
				"target_if_name":  valueOrEmpty(link.TargetIfName),
				"source_if_id":    derefInt(link.SourceIfID),
				"target_if_id":    derefInt(link.TargetIfID),
			},
		})
	}

	return CanonicalGraph{
		SchemaVersion: "1.0",
		GraphID:       "default",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Nodes:         canonicalNodes,
		Links:         canonicalLinks,
		Layouts: []CanonicalLayout{
			{
				LayoutID:      "default-layout",
				GraphID:       "default",
				LayoutType:    "manual",
				NodePositions: layoutPositions,
			},
		},
		Meta: map[string]any{
			"node_count": len(canonicalNodes),
			"link_count": len(canonicalLinks),
			"source":     "new_nms_sync",
		},
	}, nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
