package accesscontrol

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"
)

var (
	ErrNotLicensed    = errors.New("access_control_not_licensed")
	ErrDoorNotFound   = errors.New("door_not_found")
	ErrCardNotFound   = errors.New("card_not_found")
	ErrScheduleDenied = errors.New("schedule_card_required")
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) LicenseEnabled() bool {
	var val string
	err := s.db.QueryRow("SELECT config_value FROM system_config WHERE config_key = 'access_control_enabled'").Scan(&val)
	return err == nil && val == "1"
}

func (s *Service) ModuleStatus() (ModuleStatus, error) {
	var status ModuleStatus
	status.Enabled = s.LicenseEnabled()
	_ = s.db.QueryRow("SELECT COUNT(*) FROM ac_doors").Scan(&status.DoorCount)
	_ = s.db.QueryRow("SELECT COUNT(*) FROM ac_cards").Scan(&status.CardCount)
	return status, nil
}

func (s *Service) ListDoors() ([]Door, error) {
	rows, err := s.db.Query(`
		SELECT id, name, location, ip_address, port, username, manufacturer, model, protocol,
		       is_enabled, status, last_seen, created_at
		FROM ac_doors ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	doors := []Door{}
	for rows.Next() {
		var door Door
		_ = rows.Scan(&door.ID, &door.Name, &door.Location, &door.IPAddress, &door.Port, &door.Username,
			&door.Manufacturer, &door.Model, &door.Protocol, &door.IsEnabled, &door.Status, &door.LastSeen, &door.CreatedAt)
		doors = append(doors, door)
	}
	return doors, rows.Err()
}

func (s *Service) GetDoor(id string) (Door, error) {
	var door Door
	err := s.db.QueryRow(`
		SELECT id, name, location, ip_address, port, username, manufacturer, model, protocol,
		       is_enabled, status, last_seen, created_at
		FROM ac_doors WHERE id=?`, id).
		Scan(&door.ID, &door.Name, &door.Location, &door.IPAddress, &door.Port, &door.Username,
			&door.Manufacturer, &door.Model, &door.Protocol, &door.IsEnabled, &door.Status, &door.LastSeen, &door.CreatedAt)
	if err == sql.ErrNoRows {
		return Door{}, ErrDoorNotFound
	}
	return door, err
}

func (s *Service) CreateDoor(req DoorRequest, secret string) (int64, error) {
	if req.Port == 0 {
		req.Port = 80
	}
	if req.Protocol == "" {
		req.Protocol = "http"
	}

	encPwd, err := encryptPassword(secret, req.Password)
	if err != nil {
		return 0, err
	}

	res, err := s.db.Exec(`
		INSERT INTO ac_doors (name, location, ip_address, port, username, password_encrypted,
		                      manufacturer, model, protocol)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.Name, req.Location, req.IPAddress, req.Port, req.Username, encPwd,
		req.Manufacturer, req.Model, req.Protocol)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Service) UpdateDoor(id string, req DoorRequest, secret string) (bool, error) {
	var (
		res sql.Result
		err error
	)

	if req.Password != "" {
		encPwd, encErr := encryptPassword(secret, req.Password)
		if encErr != nil {
			return false, encErr
		}
		res, err = s.db.Exec(`UPDATE ac_doors SET name=?, location=?, ip_address=?, port=?,
			username=?, password_encrypted=?, manufacturer=?, model=?, protocol=? WHERE id=?`,
			req.Name, req.Location, req.IPAddress, req.Port, req.Username, encPwd,
			req.Manufacturer, req.Model, req.Protocol, id)
	} else {
		res, err = s.db.Exec(`UPDATE ac_doors SET name=?, location=?, ip_address=?, port=?,
			username=?, manufacturer=?, model=?, protocol=? WHERE id=?`,
			req.Name, req.Location, req.IPAddress, req.Port, req.Username,
			req.Manufacturer, req.Model, req.Protocol, id)
	}
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

func (s *Service) DeleteDoor(id string) (bool, error) {
	res, err := s.db.Exec("DELETE FROM ac_doors WHERE id=?", id)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

func (s *Service) ControlDoor(id string, action string) (ControlDoorResult, error) {
	var doorName string
	if err := s.db.QueryRow("SELECT name FROM ac_doors WHERE id=?", id).Scan(&doorName); err != nil {
		if err == sql.ErrNoRows {
			return ControlDoorResult{}, ErrDoorNotFound
		}
		return ControlDoorResult{}, err
	}

	eventType := "open"
	switch action {
	case "close", "lock":
		eventType = "close"
	default:
		eventType = "open"
	}

	doorID, _ := strconv.Atoi(id)
	_, _ = s.db.Exec(`INSERT INTO ac_events (door_id, event_type, holder_name) VALUES (?, ?, ?)`,
		doorID, eventType, "手動操作")
	_, _ = s.db.Exec("UPDATE ac_doors SET status=?, last_seen=datetime('now') WHERE id=?", eventType, id)

	return ControlDoorResult{Action: action, Door: doorName}, nil
}

func (s *Service) ListCards(q string) ([]Card, error) {
	pattern := "%" + q + "%"
	rows, err := s.db.Query(`
		SELECT id, card_number, holder_name, department, is_active, valid_from, valid_until, created_at
		FROM ac_cards
		WHERE holder_name LIKE ? OR card_number LIKE ? OR department LIKE ?
		ORDER BY id ASC`, pattern, pattern, pattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []Card{}
	for rows.Next() {
		var card Card
		_ = rows.Scan(&card.ID, &card.CardNumber, &card.HolderName, &card.Department,
			&card.IsActive, &card.ValidFrom, &card.ValidUntil, &card.CreatedAt)
		list = append(list, card)
	}
	return list, rows.Err()
}

func (s *Service) CreateCard(req CardRequest) (int64, error) {
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	res, err := s.db.Exec(`
		INSERT INTO ac_cards (card_number, holder_name, department, is_active, valid_from, valid_until)
		VALUES (?, ?, ?, ?, ?, ?)`,
		req.CardNumber, req.HolderName, req.Department, isActive, nullableString(req.ValidFrom), nullableString(req.ValidUntil))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Service) UpdateCard(id string, req CardRequest) (bool, error) {
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	res, err := s.db.Exec(`UPDATE ac_cards SET card_number=?, holder_name=?, department=?,
		is_active=?, valid_from=?, valid_until=? WHERE id=?`,
		req.CardNumber, req.HolderName, req.Department, isActive, nullableString(req.ValidFrom), nullableString(req.ValidUntil), id)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

func (s *Service) DeleteCard(id string) (bool, error) {
	res, err := s.db.Exec("DELETE FROM ac_cards WHERE id=?", id)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

func (s *Service) ListEvents(q EventsQuery) ([]Event, error) {
	limit := q.Limit
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	query := `
		SELECT e.id, e.door_id, COALESCE(d.name,'') as door_name,
		       e.card_number, e.holder_name, e.event_type, e.occurred_at
		FROM ac_events e
		LEFT JOIN ac_doors d ON d.id = e.door_id
		WHERE 1=1`
	args := []interface{}{}
	if q.DoorID != "" {
		query += " AND e.door_id=?"
		args = append(args, q.DoorID)
	}
	if q.CardNumber != "" {
		query += " AND e.card_number LIKE ?"
		args = append(args, "%"+q.CardNumber+"%")
	}
	if q.EventType != "" {
		query += " AND e.event_type=?"
		args = append(args, q.EventType)
	}
	query += " ORDER BY e.occurred_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Event{}
	for rows.Next() {
		var event Event
		_ = rows.Scan(&event.ID, &event.DoorID, &event.DoorName, &event.CardNumber, &event.HolderName, &event.EventType, &event.OccurredAt)
		list = append(list, event)
	}
	return list, rows.Err()
}

func (s *Service) AddEvent(body EventCreateRequest) (int64, error) {
	cardID := sql.NullInt64{}
	holderName := body.HolderName
	if body.CardNumber != "" {
		var cid int64
		var name string
		if s.db.QueryRow("SELECT id, holder_name FROM ac_cards WHERE card_number=?", body.CardNumber).Scan(&cid, &name) == nil {
			cardID = sql.NullInt64{Int64: cid, Valid: true}
			if holderName == "" {
				holderName = name
			}
		}
	}

	res, err := s.db.Exec(`INSERT INTO ac_events (door_id, card_id, card_number, holder_name, event_type)
		VALUES (?, ?, ?, ?, ?)`,
		body.DoorID, cardID, body.CardNumber, holderName, body.EventType)
	if err != nil {
		return 0, err
	}
	if body.DoorID != nil {
		_, _ = s.db.Exec("UPDATE ac_doors SET last_seen=datetime('now') WHERE id=?", *body.DoorID)
	}
	return res.LastInsertId()
}

func (s *Service) ListSchedules(status, cardNum string) ([]Schedule, error) {
	query := `SELECT s.id, s.card_id, s.card_number, s.door_id, COALESCE(d.name,'') as door_name,
		s.holder_name, s.department, s.allow_days, s.time_from, s.time_until,
		s.valid_from, s.valid_until, s.status, s.note,
		s.created_at, COALESCE(s.approved_at,''), COALESCE(s.approved_by,'')
		FROM ac_card_schedules s
		LEFT JOIN ac_doors d ON d.id = s.door_id
		WHERE 1=1`
	args := []interface{}{}
	if status != "" {
		query += " AND s.status = ?"
		args = append(args, status)
	}
	if cardNum != "" {
		query += " AND s.card_number LIKE ?"
		args = append(args, "%"+cardNum+"%")
	}
	query += " ORDER BY s.created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Schedule{}
	for rows.Next() {
		var item Schedule
		_ = rows.Scan(&item.ID, &item.CardID, &item.CardNumber, &item.DoorID, &item.DoorName,
			&item.HolderName, &item.Department, &item.AllowDays, &item.TimeFrom, &item.TimeUntil,
			&item.ValidFrom, &item.ValidUntil, &item.Status, &item.Note,
			&item.CreatedAt, &item.ApprovedAt, &item.ApprovedBy)
		list = append(list, item)
	}
	return list, rows.Err()
}

func (s *Service) CreateSchedule(req ScheduleRequest) (int64, error) {
	normalized := normalizeScheduleRequest(req)
	cardID := s.lookupCardID(normalized.CardNumber)
	res, err := s.db.Exec(`
		INSERT INTO ac_card_schedules
			(card_id, card_number, door_id, holder_name, department, allow_days,
			 time_from, time_until, valid_from, valid_until, status, note)
		VALUES (?,?,?,?,?,?,?,?,?,?,'pending',?)`,
		cardID, normalized.CardNumber, normalized.DoorID, normalized.HolderName, normalized.Department,
		normalized.AllowDays, normalized.TimeFrom, normalized.TimeUntil, normalized.ValidFrom, normalized.ValidUntil, normalized.Note)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Service) UpdateSchedule(id string, req ScheduleRequest) error {
	normalized := normalizeScheduleRequest(req)
	cardID := s.lookupCardID(normalized.CardNumber)
	_, err := s.db.Exec(`
		UPDATE ac_card_schedules SET
			card_id=?, card_number=?, door_id=?, holder_name=?, department=?,
			allow_days=?, time_from=?, time_until=?, valid_from=?, valid_until=?, note=?
		WHERE id=?`,
		cardID, normalized.CardNumber, normalized.DoorID, normalized.HolderName, normalized.Department,
		normalized.AllowDays, normalized.TimeFrom, normalized.TimeUntil, normalized.ValidFrom, normalized.ValidUntil, normalized.Note, id)
	return err
}

func (s *Service) ApproveSchedule(id string, username string, action string) (string, error) {
	status := "approved"
	if action == "reject" {
		status = "rejected"
	}
	_, err := s.db.Exec(`
		UPDATE ac_card_schedules SET status=?, approved_at=datetime('now'), approved_by=? WHERE id=?`,
		status, username, id)
	return status, err
}

func (s *Service) DeleteSchedule(id string) error {
	_, err := s.db.Exec("DELETE FROM ac_card_schedules WHERE id=?", id)
	return err
}

func (s *Service) CheckScheduleAccess(cardNum, doorID string) (bool, error) {
	if cardNum == "" {
		return false, ErrScheduleDenied
	}
	now := time.Now()
	today := now.Format("2006-01-02")
	timeNow := now.Format("15:04")
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekdayStr := fmt.Sprintf("%d", weekday)

	query := `SELECT id FROM ac_card_schedules
		WHERE card_number=?
		AND status='approved'
		AND valid_from <= ? AND valid_until >= ?
		AND time_from <= ? AND time_until >= ?
		AND (',' || allow_days || ',') LIKE ?`
	args := []interface{}{cardNum, today, today, timeNow, timeNow, "%," + weekdayStr + ",%"}
	if doorID != "" {
		query += " AND (door_id IS NULL OR door_id=?)"
		args = append(args, doorID)
	}
	query += " LIMIT 1"

	var foundID int
	err := s.db.QueryRow(query, args...).Scan(&foundID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func normalizeScheduleRequest(req ScheduleRequest) ScheduleRequest {
	if req.AllowDays == "" {
		req.AllowDays = "1,2,3,4,5,6,7"
	}
	if req.TimeFrom == "" {
		req.TimeFrom = "00:00"
	}
	if req.TimeUntil == "" {
		req.TimeUntil = "23:59"
	}
	return req
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func (s *Service) lookupCardID(cardNumber string) *int {
	var id int
	if err := s.db.QueryRow("SELECT id FROM ac_cards WHERE card_number=?", cardNumber).Scan(&id); err == nil {
		return &id
	}
	return nil
}

func encryptPassword(secret string, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	hash := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}
