package topology

import (
	"database/sql"
	"strconv"
)

func (s *Service) UpdatePositions(input UpdatePositionsInput, meta MutationMeta) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}

	deviceIDs := make([]int, 0, len(input.Positions))
	for _, pos := range input.Positions {
		deviceIDs = append(deviceIDs, pos.DeviceID)
	}

	beforePositions, err := s.loadPositionsByDeviceIDsTx(tx, deviceIDs)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	for _, pos := range input.Positions {
		if _, err := tx.Exec("UPDATE devices SET pos_x = ?, pos_y = ? WHERE id = ?", pos.PosX, pos.PosY, pos.DeviceID); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	afterPositions, err := s.loadPositionsByDeviceIDsTx(tx, deviceIDs)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	snapshot, err := s.loadCurrentLayoutTx(tx)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := s.insertTopologyChangeLogTx(
		tx,
		meta,
		"update_positions",
		"topology_layout",
		snapshot.LayoutID,
		snapshot.GraphID,
		map[string]any{
			"updated_count": len(input.Positions),
			"reason":        "positions_updated",
		},
		map[string]any{"positions": beforePositions},
		map[string]any{"positions": afterPositions},
	); err != nil {
		_ = tx.Rollback()
		return err
	}

	if err := s.insertLayoutSnapshotTx(tx, meta, "positions_updated", snapshot); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (s *Service) loadPositionsByDeviceIDsTx(tx *sql.Tx, deviceIDs []int) ([]map[string]any, error) {
	positions := make([]map[string]any, 0, len(deviceIDs))
	seen := make(map[int]struct{}, len(deviceIDs))

	for _, deviceID := range deviceIDs {
		if _, exists := seen[deviceID]; exists {
			continue
		}
		seen[deviceID] = struct{}{}

		var posX, posY float64
		err := tx.QueryRow(
			"SELECT CAST(COALESCE(pos_x, 0) AS REAL), CAST(COALESCE(pos_y, 0) AS REAL) FROM devices WHERE id = ?",
			deviceID,
		).Scan(&posX, &posY)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}

		positions = append(positions, map[string]any{
			"device_id": deviceID,
			"pos_x":     posX,
			"pos_y":     posY,
		})
	}

	return positions, nil
}

func (s *Service) loadCurrentLayoutTx(tx *sql.Tx) (CanonicalLayout, error) {
	rows, err := tx.Query(`
		SELECT id, CAST(COALESCE(pos_x, 0) AS REAL), CAST(COALESCE(pos_y, 0) AS REAL)
		FROM devices
		ORDER BY id
	`)
	if err != nil {
		return CanonicalLayout{}, err
	}
	defer rows.Close()

	positions := make([]CanonicalNodePos, 0)
	for rows.Next() {
		var (
			deviceID int
			posX     float64
			posY     float64
		)
		if err := rows.Scan(&deviceID, &posX, &posY); err != nil {
			return CanonicalLayout{}, err
		}
		positions = append(positions, CanonicalNodePos{
			NodeID: "device:" + strconv.Itoa(deviceID),
			X:      posX,
			Y:      posY,
		})
	}

	return CanonicalLayout{
		LayoutID:      "default-layout",
		GraphID:       "default",
		LayoutType:    "manual",
		NodePositions: positions,
	}, nil
}
