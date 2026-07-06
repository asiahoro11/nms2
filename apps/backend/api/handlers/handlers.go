// Made by YTSworks
// YTS工作室製作
package handlers

import (
	"database/sql"
	"management-server/config"
	acmodule "management-server/modules/accesscontrol"
	adminmodule "management-server/modules/admin"
	authmodule "management-server/modules/auth"
	backupmodule "management-server/modules/backup"
	cameramodule "management-server/modules/camera"
	dashboardmodule "management-server/modules/dashboard"
	devicesmodule "management-server/modules/devices"
	iotmodule "management-server/modules/iot"
	licensemodule "management-server/modules/license"
	logsmodule "management-server/modules/logs"
	notificationsmodule "management-server/modules/notifications"
	pdumodule "management-server/modules/pdu"
	reportsmodule "management-server/modules/reports"
	toolsmodule "management-server/modules/tools"
	topologymodule "management-server/modules/topology"
	websshmodule "management-server/modules/webssh"
	"management-server/pkg/loginlimiter"
	"management-server/services/snmp"
	"time"
)

type Handler struct {
	config        *config.Config
	db            *sql.DB
	snmpCollector *snmp.Collector
	accessControl *acmodule.Service
	admin         *adminmodule.Service
	auth          *authmodule.Service
	backup        *backupmodule.Service
	camera        *cameramodule.Service
	dashboard     *dashboardmodule.Service
	devices       *devicesmodule.Service
	iot           *iotmodule.Service
	license       *licensemodule.Service
	logs          *logsmodule.Service
	notifications *notificationsmodule.Service
	pdu           *pdumodule.Service
	reports       *reportsmodule.Service
	topology      *topologymodule.Service
	tools         *toolsmodule.Service
	webssh        *websshmodule.Service
	startTime     time.Time

	// loginAccountLimiter throttles failures per client-IP + username;
	// loginIPLimiter caps total failures per client-IP across all usernames.
	loginAccountLimiter *loginlimiter.Limiter
	loginIPLimiter      *loginlimiter.Limiter
}

func New(cfg *config.Config, db *sql.DB, collector *snmp.Collector) *Handler {
	h := &Handler{
		config:        cfg,
		db:            db,
		snmpCollector: collector,
		accessControl: acmodule.NewService(db),
		admin:         adminmodule.NewService(db),
		auth:          authmodule.NewService(db, cfg),
		backup:        backupmodule.NewService(db, cfg, collector),
		dashboard:     dashboardmodule.NewService(db, cfg.System.Version, cfg.System.Name),
		devices:       devicesmodule.NewService(db),
		iot:           iotmodule.NewService(db),
		license:       licensemodule.NewService(db, cfg),
		logs:          logsmodule.NewService(db),
		notifications: notificationsmodule.NewService(db, cfg),
		pdu:           pdumodule.NewService(db),
		reports:       reportsmodule.NewService(db),
		topology:      topologymodule.NewService(db),
		tools:         toolsmodule.NewService(),
		webssh: websshmodule.NewService(func(id string) (string, error) {
			var ip string
			if err := db.QueryRow(`SELECT ip_address FROM devices WHERE id=?`, id).Scan(&ip); err != nil {
				return "", err
			}
			return ip, nil
		}),
		startTime: time.Now(),

		loginAccountLimiter: loginlimiter.New(5, 15*time.Minute, 15*time.Minute),
		loginIPLimiter:      loginlimiter.New(30, 15*time.Minute, 15*time.Minute),
	}
	h.camera = cameramodule.NewService(db, cfg.Security.JWTSecret, cameramodule.RuntimeHooks{
		DeviceLog: h.WriteDeviceLog,
		SystemLog: h.WriteSystemLog,
	})
	return h
}

// Response is the standard API response wrapper
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// PaginatedResponse wraps paginated API results
type PaginatedResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Total   int         `json:"total"`
	Page    int         `json:"page"`
	Limit   int         `json:"limit"`
}
