// Made by YTSworks
// YTS工作室製作
package scheduler

import (
	"database/sql"
	"log"
	"management-server/config"
	"management-server/services/dbworker"
	"management-server/services/license"
	"management-server/services/snmp"
	"time"
)

type Scheduler struct {
	collector *snmp.Collector
	db        *sql.DB
	version   string
	quit      chan struct{}
}

func New(cfg *config.Config, db *sql.DB, worker *dbworker.Worker) *Scheduler {
	return &Scheduler{
		collector: snmp.NewCollector(cfg, db, worker, cfg.SNMP.Community, cfg.SNMP.Timeout, cfg.SNMP.Retries),
		db:        db,
		version:   cfg.System.Version,
		quit:      make(chan struct{}),
	}
}

func (s *Scheduler) runtimeLocked() bool {
	return license.ShouldRuntimeLockdown(s.db, s.version)
}

func (s *Scheduler) Start() {
	log.Println("Scheduler started - polling every 60 seconds")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Scheduler panic: %v", r)
			}
		}()

		// Initial poll immediately
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Scheduler initial poll panic: %v", r)
				}
			}()
			if s.runtimeLocked() {
				return
			}
			s.collector.PollAllDevices()
		}()

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("Scheduler poll panic: %v", r)
						}
					}()
					if s.runtimeLocked() {
						return
					}
					s.collector.PollAllDevices()
				}()
			case <-s.quit:
				return
			}
		}
	}()
}

func (s *Scheduler) Stop() {
	close(s.quit)
	log.Println("Scheduler stopped")
}

func (s *Scheduler) GetCollector() *snmp.Collector {
	return s.collector
}
