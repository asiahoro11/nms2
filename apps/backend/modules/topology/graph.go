package topology

func (s *Service) loadNodes() ([]Node, error) {
	rows, err := s.db.Query(`
		SELECT id, name, ip_address, device_type, is_online, image_path,
		       sys_name, sys_uptime, sys_location,
		       CAST(COALESCE(pos_x, 0) AS REAL), CAST(COALESCE(pos_y, 0) AS REAL),
		       COALESCE(snmp_version, 2), COALESCE(is_name_custom, 0)
		FROM devices
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	nodes := make([]Node, 0)
	for rows.Next() {
		n := Node{}

		var (
			id           int64
			name         string
			ipAddress    interface{}
			deviceType   interface{}
			isOnline     interface{}
			imgPath      interface{}
			sysName      interface{}
			sysUptime    interface{}
			sysLoc       interface{}
			posX         interface{}
			posY         interface{}
			snmpVersion  interface{}
			isNameCustom interface{}
		)

		if err := rows.Scan(
			&id, &name, &ipAddress, &deviceType, &isOnline,
			&imgPath, &sysName, &sysUptime, &sysLoc,
			&posX, &posY, &snmpVersion, &isNameCustom,
		); err != nil {
			continue
		}

		n.ID = int(id)
		n.Name = name
		n.IPAddress = toString(ipAddress)
		n.DeviceType = toString(deviceType)
		if n.DeviceType == "" {
			n.DeviceType = "unknown"
		}
		n.IsOnline = toInt64(isOnline) != 0
		n.IsNameCustom = toInt64(isNameCustom) != 0
		n.PosX = toFloat64(posX)
		n.PosY = toFloat64(posY)
		n.SnmpVersion = int(toInt64(snmpVersion))

		if imgStr := toString(imgPath); imgStr != "" {
			n.ImagePath = &imgStr
		}
		if sysNameStr := toString(sysName); sysNameStr != "" {
			n.SysName = &sysNameStr
		}
		if sysUptimeStr := toString(sysUptime); sysUptimeStr != "" {
			n.SysUptime = &sysUptimeStr
		}
		if sysLocStr := toString(sysLoc); sysLocStr != "" {
			n.SysLocation = &sysLocStr
		}

		nodes = append(nodes, n)
	}

	return nodes, nil
}

func (s *Service) loadLinks() ([]Link, error) {
	rows, err := s.db.Query(`
		SELECT l.id, l.source_device_id, l.target_device_id, l.source_if_id, l.target_if_id,
		       l.source_if_name, l.target_if_name,
		       l.link_speed, l.bandwidth_usage,
		       l.link_type, l.link_label, l.is_manual,
		       si.if_speed as source_speed,
		       ti.if_speed as target_speed,
		       si.bandwidth_in as source_bw_in,
		       si.bandwidth_out as source_bw_out,
		       ti.bandwidth_in as target_bw_in,
		       ti.bandwidth_out as target_bw_out
		FROM topology_links l
		LEFT JOIN device_interfaces si ON l.source_if_id = si.id
		LEFT JOIN device_interfaces ti ON l.target_if_id = ti.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	links := make([]Link, 0)
	for rows.Next() {
		columns := make([]interface{}, 18)
		columnPointers := make([]interface{}, 18)
		for i := range columns {
			columnPointers[i] = &columns[i]
		}

		if err := rows.Scan(columnPointers...); err != nil {
			continue
		}

		link := Link{}
		link.ID = int(toInt64(columns[0]))
		link.Source = int(toInt64(columns[1]))
		link.Target = int(toInt64(columns[2]))

		if columns[3] != nil {
			value := int(toInt64(columns[3]))
			link.SourceIfID = &value
		}
		if columns[4] != nil {
			value := int(toInt64(columns[4]))
			link.TargetIfID = &value
		}
		if value := toString(columns[5]); value != "" {
			link.SourceIfName = &value
		}
		if value := toString(columns[6]); value != "" {
			link.TargetIfName = &value
		}
		if value := toString(columns[10]); value != "" {
			link.LinkLabel = &value
		}

		link.LinkSpeed = toInt64(columns[7])
		link.BandwidthUsage = toInt64(columns[8])
		link.LinkType = toString(columns[9])
		if link.LinkType == "" {
			link.LinkType = "auto"
		}
		link.IsManual = toInt64(columns[11]) != 0

		sourceSpeed := toInt64(columns[12])
		targetSpeed := toInt64(columns[13])
		if sourceSpeed > 0 {
			link.LinkSpeed = sourceSpeed
		} else if targetSpeed > 0 {
			link.LinkSpeed = targetSpeed
		}

		if link.SourceIfID != nil && *link.SourceIfID > 0 {
			link.BandwidthIn = toInt64(columns[14])
			link.BandwidthOut = toInt64(columns[15])
			if link.BandwidthIn == 0 && link.BandwidthOut == 0 && link.TargetIfID != nil && *link.TargetIfID > 0 {
				link.BandwidthIn = toInt64(columns[16])
				link.BandwidthOut = toInt64(columns[17])
			}
		} else if link.TargetIfID != nil && *link.TargetIfID > 0 {
			link.BandwidthIn = toInt64(columns[16])
			link.BandwidthOut = toInt64(columns[17])
		}

		links = append(links, link)
	}

	return links, nil
}
