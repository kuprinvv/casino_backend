package model

import "time"

type Session struct {
	ID           string
	UserID       int
	RefreshToken string
	ExpiresAt    time.Time
}
