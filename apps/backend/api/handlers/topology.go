package handlers

import (
	"net/http"
	"strconv"

	topologymodule "management-server/modules/topology"

	"github.com/gin-gonic/gin"
)

// GetTopology returns the current topology graph while delegating graph assembly
// to the extracted topology module service.
func (h *Handler) GetTopology(c *gin.Context) {
	graph, err := h.topology.LoadGraph()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Data: graph})
}

// CreateTopologyLink creates a manual topology link while keeping HTTP/audit
// concerns in the existing handler layer.
func (h *Handler) CreateTopologyLink(c *gin.Context) {
	var input topologymodule.CreateLinkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.WriteAuditFromContext(c, "create_topology_link", "topology", "failed", map[string]interface{}{
			"module": "topology",
			"reason": "invalid_request",
			"error":  err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	if input.LinkType == "" {
		input.LinkType = "manual"
	}

	result, err := h.topology.CreateLink(input, h.topologyMutationMeta(c))
	if err != nil {
		h.WriteAuditFromContext(c, "create_topology_link", "topology", "failed", map[string]interface{}{
			"module": "topology",
			"reason": "create_link_failed",
			"error":  err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteAuditFromContext(c, "create_topology_link", auditResource("topology_link", result.ID), "success", map[string]interface{}{
		"module":         "topology",
		"resource_type":  "topology_link",
		"resource_id":    strconv.FormatInt(result.ID, 10),
		"source_device":  input.SourceDeviceID,
		"target_device":  input.TargetDeviceID,
		"source_if_id":   input.SourceIfID,
		"target_if_id":   input.TargetIfID,
		"source_if_name": result.SourceIfName,
		"target_if_name": result.TargetIfName,
		"link_speed":     result.LinkSpeed,
		"link_type":      input.LinkType,
		"link_label":     input.LinkLabel,
	})
	c.JSON(http.StatusCreated, Response{Success: true, Data: map[string]int64{"id": result.ID}})
}

// UpdateTopologyLink updates a manual topology link via the extracted topology service.
func (h *Handler) UpdateTopologyLink(c *gin.Context) {
	id := c.Param("id")
	var input map[string]interface{}
	if err := c.ShouldBindJSON(&input); err != nil {
		h.WriteAuditFromContext(c, "update_topology_link", auditResource("topology_link", id), "failed", map[string]interface{}{
			"module": "topology",
			"reason": "invalid_request",
			"error":  err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	result, err := h.topology.UpdateLink(id, input, h.topologyMutationMeta(c))
	if err != nil {
		h.WriteAuditFromContext(c, "update_topology_link", auditResource("topology_link", id), "failed", map[string]interface{}{
			"module": "topology",
			"reason": "update_link_failed",
			"error":  err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	h.WriteAuditFromContext(c, "update_topology_link", auditResource("topology_link", id), "success", map[string]interface{}{
		"module":        "topology",
		"resource_type": "topology_link",
		"resource_id":   id,
		"old_values":    result.Before,
		"new_values":    result.After,
		"config_change": true,
		"change_scope":  "topology_link",
	})
	c.JSON(http.StatusOK, Response{Success: true, Message: "Link updated"})
}

// DeleteTopologyLink removes a manual topology link.
func (h *Handler) DeleteTopologyLink(c *gin.Context) {
	id := c.Param("id")
	result, err := h.topology.DeleteLink(id, h.topologyMutationMeta(c))
	if err != nil {
		h.WriteAuditFromContext(c, "delete_topology_link", auditResource("topology_link", id), "failed", map[string]interface{}{
			"module": "topology",
			"reason": "delete_link_failed",
			"error":  err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	result.Before["module"] = "topology"
	result.Before["resource_type"] = "topology_link"
	result.Before["resource_id"] = id
	h.WriteAuditFromContext(c, "delete_topology_link", auditResource("topology_link", id), "success", result.Before)
	c.JSON(http.StatusOK, Response{Success: true, Message: "Link deleted"})
}

// UpdateDevicePositions persists topology layout coordinates while leaving
// topology relation data in the module service.
func (h *Handler) UpdateDevicePositions(c *gin.Context) {
	var input topologymodule.UpdatePositionsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		h.WriteAuditFromContext(c, "update_topology_positions", "topology", "failed", map[string]interface{}{
			"module": "topology",
			"reason": "invalid_request",
			"error":  err.Error(),
		})
		c.JSON(http.StatusBadRequest, Response{Success: false, Error: err.Error()})
		return
	}

	if err := h.topology.UpdatePositions(input, h.topologyMutationMeta(c)); err != nil {
		h.WriteAuditFromContext(c, "update_topology_positions", "topology", "failed", map[string]interface{}{
			"module": "topology",
			"reason": "db_update_failed",
			"error":  err.Error(),
		})
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}

	positions := make([]map[string]interface{}, 0, len(input.Positions))
	for _, pos := range input.Positions {
		positions = append(positions, map[string]interface{}{
			"device_id": pos.DeviceID,
			"pos_x":     pos.PosX,
			"pos_y":     pos.PosY,
		})
	}
	h.WriteAuditFromContext(c, "update_topology_positions", "topology", "success", map[string]interface{}{
		"module":        "topology",
		"resource_type": "topology",
		"resource_id":   "layout",
		"new_values": map[string]interface{}{
			"positions": positions,
		},
		"config_change": true,
		"change_scope":  "topology_positions",
		"updated_count": len(input.Positions),
	})
	c.JSON(http.StatusOK, Response{Success: true, Message: "Positions updated"})
}

func (h *Handler) topologyMutationMeta(c *gin.Context) topologymodule.MutationMeta {
	return topologymodule.MutationMeta{
		Actor:         h.auditUsername(c),
		SourceIP:      c.ClientIP(),
		CorrelationID: c.GetHeader("X-Request-ID"),
	}
}

// DiscoverTopology keeps discovery orchestration in handlers while moving graph
// storage concerns into the topology module.
func (h *Handler) DiscoverTopology(c *gin.Context) {
	h.WriteSystemLog("notice", "topology", "topology_discovery_started", "topology discovery started", map[string]interface{}{
		"collector_ready": h.snmpCollector != nil,
	})
	h.WriteAuditFromContext(c, "discover_topology", "topology", "success", map[string]interface{}{
		"module":          "topology",
		"collector_ready": h.snmpCollector != nil,
		"method":          "lldp",
	})
	if h.snmpCollector != nil {
		h.WriteSystemLog("notice", "topology", "topology_discovery_dispatch_queued", "topology discovery dispatch queued", map[string]interface{}{
			"method": "lldp",
		})
		go h.snmpCollector.DiscoverLLDPTopology()
	} else {
		h.WriteSystemLog("warning", "topology", "topology_discovery_dispatch_skipped", "topology discovery dispatch skipped", map[string]interface{}{
			"method": "lldp",
			"reason": "snmp_collector_unavailable",
		})
	}

	c.JSON(http.StatusOK, Response{Success: true, Message: "LLDP discovery started. Refresh in a few seconds to see results."})
}

// GetDeviceInterfacesForLink returns interface candidates for manual topology linking.
func (h *Handler) GetDeviceInterfacesForLink(c *gin.Context) {
	interfaces, err := h.topology.LoadDeviceInterfaces(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, Response{Success: true, Data: interfaces})
}
