// Made by YTSworks
// YTS工作室製作
package topology

import "database/sql"

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}
