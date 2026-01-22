package repository

import (
	"casino_backend/internal/model"
	"context"
)

type LineRepository interface {
	GetBalance() (int, error)
	UpdateBalance(amount int) error
	GetFreeSpinCount() (int, error)
	UpdateFreeSpinCount(count int) error
}

type CascadeRepository interface {
	GetBalance() (int, error)
	UpdateBalance(amount int) error
	GetFreeSpinCount() (int, error)
	UpdateFreeSpinCount(count int) error

	GetMultiplierState() ([7][7]int, [7][7]int)
	SetMultiplierState(mult, hits [7][7]int) error
	ResetMultiplierState() error
}

type AuthRepository interface {
	CreateSession(ctx context.Context, session *model.Session) error
	GetRefreshTokenBySessionID(ctx context.Context, sessionID string) (refreshToken string, err error)
	DeleteSession(ctx context.Context, sessionID string) error
}

type UserRepository interface {
	CreateUser(ctx context.Context, user *model.User) (id int, err error)
}
