package syslog

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

type Receiver struct {
	port int
	db   *sql.DB
	conn *net.UDPConn
	stop chan struct{}
}

func NewReceiver(port int, db *sql.DB) *Receiver {
	return &Receiver{
		port: port,
		db:   db,
		stop: make(chan struct{}),
	}
}

func (r *Receiver) Start() {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", r.port))
	if err != nil {
		log.Printf("Syslog: Failed to resolve address: %v", err)
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("Syslog: Failed to listen: %v", err)
		return
	}
	r.conn = conn

	log.Printf("Syslog receiver started on port %d", r.port)

	go r.listen()
}

func (r *Receiver) listen() {
	buf := make([]byte, 2048)
	for {
		select {
		case <-r.stop:
			return
		default:
			n, src, err := r.conn.ReadFromUDP(buf)
			if err != nil {
				if !strings.Contains(err.Error(), "use of closed network connection") {
					log.Printf("Syslog receive error: %v", err)
				}
				continue
			}

			message := string(buf[:n])
			sourceIP := src.IP.String()

			// Try to find device ID from IP
			var deviceID sql.NullInt64
			r.db.QueryRow("SELECT id FROM devices WHERE ip_address = ?", sourceIP).Scan(&deviceID)

			event := NormalizeSyslogMessage(message)
			event.Context["source_ip"] = sourceIP

			var deviceName string
			var macAddress string
			if deviceID.Valid {
				_ = r.db.QueryRow(
					"SELECT COALESCE(name, ''), COALESCE(mac_address, '') FROM devices WHERE id = ?",
					deviceID.Int64,
				).Scan(&deviceName, &macAddress)
				if deviceName != "" {
					event.Context["device_name"] = deviceName
				}
				if macAddress != "" {
					event.Context["mac_address"] = macAddress
				}
			}

			_, err = r.db.Exec(`
				INSERT INTO syslogs (device_id, source_ip, severity, facility, message, received_at) 
				VALUES (?, ?, ?, ?, ?, ?)
			`, deviceID, sourceIP, event.Severity, event.Facility, event.RawMessage, time.Now().Format("2006-01-02 15:04:05"))

			if err != nil {
				log.Printf("Error saving syslog: %v", err)
				continue
			}

			_, err = r.db.Exec(`
				INSERT INTO device_logs (
					occurred_at, device_id, device_name, ip_address, mac_address, facility,
					severity, raw_message, normalized_message, matched_rule, ack_status, context_json
				)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			`,
				time.Now().Format("2006-01-02 15:04:05"),
				deviceID,
				deviceName,
				sourceIP,
				macAddress,
				event.Facility,
				event.Severity,
				event.RawMessage,
				event.NormalizedMessage,
				event.MatchedRule,
				"unacked",
				MarshalContextJSON(event.Context),
			)
			if err != nil {
				log.Printf("Error saving normalized device log: %v", err)
			}
		}
	}
}

func (r *Receiver) Stop() {
	close(r.stop)
	if r.conn != nil {
		r.conn.Close()
	}
	log.Println("Syslog receiver stopped")
}
