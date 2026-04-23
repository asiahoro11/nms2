package topology

import (
	"database/sql"
	"encoding/json"
)

func marshalJSONOrEmpty(value any) string {
	if value == nil {
		return ""
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func (s *Service) insertTopologyChangeLogTx(
	tx *sql.Tx,
	meta MutationMeta,
	action string,
	entityType string,
	entityID string,
	graphID string,
	detail map[string]any,
	oldValues map[string]any,
	newValues map[string]any,
) error {
	if graphID == "" {
		graphID = "default"
	}

	_, err := tx.Exec(`
		INSERT INTO topology_change_logs (
			actor, source_ip, action, entity_type, entity_id, graph_id,
			detail_json, old_values_json, new_values_json, correlation_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		meta.Actor,
		meta.SourceIP,
		action,
		entityType,
		entityID,
		graphID,
		marshalJSONOrEmpty(detail),
		marshalJSONOrEmpty(oldValues),
		marshalJSONOrEmpty(newValues),
		meta.CorrelationID,
	)
	return err
}

func (s *Service) insertLayoutSnapshotTx(
	tx *sql.Tx,
	meta MutationMeta,
	reason string,
	snapshot CanonicalLayout,
) error {
	_, err := tx.Exec(`
		INSERT INTO topology_layout_snapshots (
			actor, source_ip, graph_id, layout_id, layout_type,
			reason, snapshot_json, correlation_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`,
		meta.Actor,
		meta.SourceIP,
		snapshot.GraphID,
		snapshot.LayoutID,
		snapshot.LayoutType,
		reason,
		marshalJSONOrEmpty(snapshot),
		meta.CorrelationID,
	)
	return err
}
