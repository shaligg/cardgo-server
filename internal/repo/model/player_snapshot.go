package model

import "time"

type PlayerSnapshot struct {
	UID       string `gorm:"primaryKey;size:64"`
	Version   int64
	Payload   []byte
	UpdatedAt time.Time
}
