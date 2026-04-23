package cloud

import (
	"database/sql"
	"log"
	"time"
)

// BufferEntry represents a buffered MQTT message.
type BufferEntry struct {
	ID        int64
	Topic     string
	Payload   []byte
	CreatedAt time.Time
}

// Buffer provides offline store-and-forward for MQTT messages using SQLite.
type Buffer struct {
	db      *sql.DB
	maxSize int
}

// NewBuffer creates a new offline message buffer.
func NewBuffer(db *sql.DB, maxSize int) *Buffer {
	return &Buffer{
		db:      db,
		maxSize: maxSize,
	}
}

// Init creates the buffer table if it doesn't exist.
func (b *Buffer) Init() error {
	_, err := b.db.Exec(`
		CREATE TABLE IF NOT EXISTS cloud_buffer (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			topic TEXT NOT NULL,
			payload BLOB NOT NULL,
			created_at DATETIME DEFAULT (datetime('now')),
			sent INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		return err
	}

	// Create index for efficient drain queries
	_, err = b.db.Exec(`CREATE INDEX IF NOT EXISTS idx_cloud_buffer_unsent ON cloud_buffer(sent, id)`)
	return err
}

// Push adds a message to the buffer. If the buffer exceeds maxSize,
// the oldest unsent messages are discarded.
func (b *Buffer) Push(topic string, payload []byte) {
	_, err := b.db.Exec(
		`INSERT INTO cloud_buffer (topic, payload) VALUES (?, ?)`,
		topic, payload,
	)
	if err != nil {
		log.Printf("[Cloud Buffer] Failed to insert: %v", err)
		return
	}

	// Enforce max size — delete oldest unsent messages if over limit
	b.db.Exec(`
		DELETE FROM cloud_buffer
		WHERE sent = 0 AND id NOT IN (
			SELECT id FROM cloud_buffer WHERE sent = 0 ORDER BY id DESC LIMIT ?
		)
	`, b.maxSize)
}

// DrainAll returns all unsent messages in FIFO order.
func (b *Buffer) DrainAll() ([]BufferEntry, error) {
	rows, err := b.db.Query(
		`SELECT id, topic, payload, created_at FROM cloud_buffer WHERE sent = 0 ORDER BY id ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []BufferEntry
	for rows.Next() {
		var e BufferEntry
		if err := rows.Scan(&e.ID, &e.Topic, &e.Payload, &e.CreatedAt); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// MarkSent marks a buffer entry as successfully sent.
func (b *Buffer) MarkSent(id int64) {
	b.db.Exec(`UPDATE cloud_buffer SET sent = 1 WHERE id = ?`, id)
}

// Cleanup removes sent messages older than 24 hours.
func (b *Buffer) Cleanup() {
	b.db.Exec(`DELETE FROM cloud_buffer WHERE sent = 1 AND created_at < datetime('now', '-1 day')`)
}

// Count returns the number of unsent messages in the buffer.
func (b *Buffer) Count() int {
	var count int
	b.db.QueryRow(`SELECT COUNT(*) FROM cloud_buffer WHERE sent = 0`).Scan(&count)
	return count
}
