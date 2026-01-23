package repository

import (
	"casino_backend/internal/model"
	"context"
)

type LineRepository interface {
	GetFreeSpinCount(ctx context.Context, id int) (int, error)
	UpdateFreeSpinCount(ctx context.Context, id int, count int) error
}

type CascadeRepository interface {
	GetFreeSpinCount(ctx context.Context, id int) (int, error)
	UpdateFreeSpinCount(ctx context.Context, id int, count int) error

	GetMultiplierState(ctx context.Context, id int) ([7][7]int, [7][7]int, error)
	SetMultiplierState(ctx context.Context, id int, multMtrx, hitsMtrx [7][7]int) error
	ResetMultiplierState(ctx context.Context, id int) error
}

type AuthRepository interface {
	CreateSession(ctx context.Context, session *model.Session) error
	GetRefreshTokenBySessionID(ctx context.Context, sessionID string) (refreshToken string, err error)
	DeleteSession(ctx context.Context, sessionID string) error
	GetUserBySessionID(ctx context.Context, sessionID string) (*model.User, error)
}

type UserRepository interface {
	CreateUser(ctx context.Context, user *model.User) (id int, err error)
	GetUserByLogin(ctx context.Context, login string) (*model.User, error)

	GetBalance(ctx context.Context, id int) (int, error)
	UpdateBalance(ctx context.Context, id int, amount int) error
}
