package topology

import (
	"database/sql"
	"strconv"
)

func (s *Service) ResolveLinkInterfaces(input CreateLinkInput) (string, string, int64, error) {
	var sourceIfName, targetIfName string
	var sourceSpeed, targetSpeed int64

	if input.SourceIfID != nil && *input.SourceIfID > 0 {
		if err := s.db.QueryRow(
			"SELECT COALESCE(if_name, if_desc, ''), COALESCE(if_speed, 0) FROM device_interfaces WHERE id = ?",
			*input.SourceIfID,
		).Scan(&sourceIfName, &sourceSpeed); err != nil && err != sql.ErrNoRows {
			return "", "", 0, err
		}
	}
	if input.TargetIfID != nil && *input.TargetIfID > 0 {
		if err := s.db.QueryRow(
			"SELECT COALESCE(if_name, if_desc, ''), COALESCE(if_speed, 0) FROM device_interfaces WHERE id = ?",
			*input.TargetIfID,
		).Scan(&targetIfName, &targetSpeed); err != nil && err != sql.ErrNoRows {
			return "", "", 0, err
		}
	}

	if input.SourceIfName != "" {
		sourceIfName = input.SourceIfName
	}
	if input.TargetIfName != "" {
		targetIfName = input.TargetIfName
	}

	linkSpeed := input.LinkSpeed
	if linkSpeed == 0 {
		if sourceSpeed > 0 {
			linkSpeed = sourceSpeed
		} else if targetSpeed > 0 {
			linkSpeed = targetSpeed
		}
	}

	return sourceIfName, targetIfName, linkSpeed, nil
}

func (s *Service) CreateLink(input CreateLinkInput, meta MutationMeta) (CreateLinkResult, error) {
	sourceIfName, targetIfName, linkSpeed, err := s.ResolveLinkInterfaces(input)
	if err != nil {
		return CreateLinkResult{}, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return CreateLinkResult{}, err
	}

	result, err := tx.Exec(`
		INSERT INTO topology_links (
			source_device_id, target_device_id, source_if_id, target_if_id,
			source_if_name, target_if_name, link_speed, link_type, link_label, is_manual
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
	`, input.SourceDeviceID, input.TargetDeviceID, input.SourceIfID, input.TargetIfID,
		sourceIfName, targetIfName, linkSpeed, input.LinkType, input.LinkLabel)
	if err != nil {
		_ = tx.Rollback()
		return CreateLinkResult{}, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return CreateLinkResult{}, err
	}

	newValues := map[string]any{
		"source_device_id": input.SourceDeviceID,
		"target_device_id": input.TargetDeviceID,
		"source_if_id":     intPtrValue(input.SourceIfID),
		"target_if_id":     intPtrValue(input.TargetIfID),
		"source_if_name":   sourceIfName,
		"target_if_name":   targetIfName,
		"link_speed":       linkSpeed,
		"link_type":        input.LinkType,
		"link_label":       input.LinkLabel,
		"is_manual":        true,
	}

	if err := s.insertTopologyChangeLogTx(
		tx,
		meta,
		"create_link",
		"topology_link",
		strconv.FormatInt(id, 10),
		"default",
		map[string]any{"reason": "manual_link_created"},
		nil,
		newValues,
	); err != nil {
		_ = tx.Rollback()
		return CreateLinkResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return CreateLinkResult{}, err
	}

	return CreateLinkResult{
		ID:           id,
		SourceIfName: sourceIfName,
		TargetIfName: targetIfName,
		LinkSpeed:    linkSpeed,
	}, nil
}

func (s *Service) UpdateLink(id string, input map[string]interface{}, meta MutationMeta) (UpdateLinkResult, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return UpdateLinkResult{}, err
	}

	before, err := s.loadLinkSnapshotTx(tx, id)
	if err != nil {
		_ = tx.Rollback()
		return UpdateLinkResult{}, err
	}

	if err := s.applyLinkUpdateTx(tx, id, input); err != nil {
		_ = tx.Rollback()
		return UpdateLinkResult{}, err
	}

	after, err := s.loadLinkSnapshotTx(tx, id)
	if err != nil {
		_ = tx.Rollback()
		return UpdateLinkResult{}, err
	}

	if err := s.insertTopologyChangeLogTx(
		tx,
		meta,
		"update_link",
		"topology_link",
		id,
		"default",
		map[string]any{
			"reason":         "manual_link_updated",
			"updated_fields": mapKeys(input),
		},
		before,
		after,
	); err != nil {
		_ = tx.Rollback()
		return UpdateLinkResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return UpdateLinkResult{}, err
	}

	return UpdateLinkResult{
		Before: before,
		After:  after,
	}, nil
}

func (s *Service) DeleteLink(id string, meta MutationMeta) (DeleteLinkResult, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return DeleteLinkResult{}, err
	}

	before, err := s.loadLinkSnapshotTx(tx, id)
	if err != nil {
		_ = tx.Rollback()
		return DeleteLinkResult{}, err
	}

	if _, err := tx.Exec("DELETE FROM topology_links WHERE id = ?", id); err != nil {
		_ = tx.Rollback()
		return DeleteLinkResult{}, err
	}

	if err := s.insertTopologyChangeLogTx(
		tx,
		meta,
		"delete_link",
		"topology_link",
		id,
		"default",
		map[string]any{"reason": "manual_link_deleted"},
		before,
		nil,
	); err != nil {
		_ = tx.Rollback()
		return DeleteLinkResult{}, err
	}

	if err := tx.Commit(); err != nil {
		return DeleteLinkResult{}, err
	}

	return DeleteLinkResult{Before: before}, nil
}

func (s *Service) loadLinkSnapshotTx(tx *sql.Tx, id string) (map[string]interface{}, error) {
	var (
		sourceDeviceID sql.NullInt64
		targetDeviceID sql.NullInt64
		sourceIfID     sql.NullInt64
		targetIfID     sql.NullInt64
		linkSpeed      sql.NullInt64
		isManual       sql.NullBool
		linkType       sql.NullString
		linkLabel      sql.NullString
		sourceIfName   sql.NullString
		targetIfName   sql.NullString
	)

	err := tx.QueryRow(`
		SELECT source_device_id, target_device_id, source_if_id, target_if_id,
		       link_speed, is_manual, link_type, link_label, source_if_name, target_if_name
		FROM topology_links
		WHERE id = ?
	`, id).Scan(
		&sourceDeviceID,
		&targetDeviceID,
		&sourceIfID,
		&targetIfID,
		&linkSpeed,
		&isManual,
		&linkType,
		&linkLabel,
		&sourceIfName,
		&targetIfName,
	)
	if err == sql.ErrNoRows {
		return map[string]interface{}{}, nil
	}
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"source_device_id": int64OrZero(sourceDeviceID),
		"target_device_id": int64OrZero(targetDeviceID),
		"source_if_id":     int64OrZero(sourceIfID),
		"target_if_id":     int64OrZero(targetIfID),
		"link_speed":       int64OrZero(linkSpeed),
		"is_manual":        boolOrFalse(isManual),
		"link_type":        stringOrEmpty(linkType),
		"link_label":       stringOrEmpty(linkLabel),
		"source_if_name":   stringOrEmpty(sourceIfName),
		"target_if_name":   stringOrEmpty(targetIfName),
	}, nil
}

func (s *Service) applyLinkUpdateTx(tx *sql.Tx, id string, input map[string]interface{}) error {
	if linkSpeed, ok := input["link_speed"]; ok {
		if _, err := tx.Exec("UPDATE topology_links SET link_speed = ? WHERE id = ?", linkSpeed, id); err != nil {
			return err
		}
	}
	if linkLabel, ok := input["link_label"]; ok {
		if _, err := tx.Exec("UPDATE topology_links SET link_label = ? WHERE id = ?", linkLabel, id); err != nil {
			return err
		}
	}
	if sourceIfName, ok := input["source_if_name"]; ok {
		if _, err := tx.Exec("UPDATE topology_links SET source_if_name = ? WHERE id = ?", sourceIfName, id); err != nil {
			return err
		}
	}
	if targetIfName, ok := input["target_if_name"]; ok {
		if _, err := tx.Exec("UPDATE topology_links SET target_if_name = ? WHERE id = ?", targetIfName, id); err != nil {
			return err
		}
	}
	return nil
}

func intPtrValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func int64OrZero(value sql.NullInt64) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

func stringOrEmpty(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func boolOrFalse(value sql.NullBool) bool {
	if !value.Valid {
		return false
	}
	return value.Bool
}

func mapKeys(values map[string]interface{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
