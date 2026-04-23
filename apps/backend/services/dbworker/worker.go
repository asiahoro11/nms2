package dbworker

import (
	"database/sql"
	"log"
	"sync"
)

// TaskFunc defines the function signature for database tasks
type TaskFunc func(db *sql.DB) error

// Worker handles serialized database operations
type Worker struct {
	db        *sql.DB
	taskQueue chan TaskFunc
	wg        sync.WaitGroup
	quit      chan struct{}
}

// New creates a new DB Worker
func New(db *sql.DB) *Worker {
	return &Worker{
		db:        db,
		taskQueue: make(chan TaskFunc, 1000), // Buffer for 1000 tasks
		quit:      make(chan struct{}),
	}
}

// Start begins the worker loop
func (w *Worker) Start() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		log.Println("DB Worker started")
		for {
			select {
			case task := <-w.taskQueue:
				// Execute task safely with recovery
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("DB Worker panic recovered: %v", r)
						}
					}()
					if err := task(w.db); err != nil {
						log.Printf("DB Worker task failed: %v", err)
					}
				}()
			case <-w.quit:
				log.Printf("DB Worker stopping... draining %d remaining tasks", len(w.taskQueue))
				// Drain remaining tasks before shutdown
				for {
					select {
					case task := <-w.taskQueue:
						func() {
							defer func() {
								if r := recover(); r != nil {
									log.Printf("DB Worker drain panic recovered: %v", r)
								}
							}()
							if err := task(w.db); err != nil {
								log.Printf("DB Worker drain task failed: %v", err)
							}
						}()
					default:
						log.Println("DB Worker stopped (all tasks drained)")
						return
					}
				}
			}
		}
	}()
}

// Push adds a task to the queue. Returns false if queue is full.
func (w *Worker) Push(task TaskFunc) bool {
	select {
	case w.taskQueue <- task:
		return true
	default:
		log.Println("DB Worker queue full, dropping task!")
		return false
	}
}

// Stop signals the worker to stop
func (w *Worker) Stop() {
	close(w.quit)
	w.wg.Wait()
}
