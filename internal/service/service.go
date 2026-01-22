package service

import (
	"casino_backend/internal/model"
	"context"
)

type LineService interface {
	Spin(ctx context.Context, spinReq model.LineSpin) (*model.SpinResult, error)
	BuyBonus(amount int) error
	Deposit(amount int) error
	CheckData() (*model.Data, error)
}

type CascadeService interface {
	Spin(ctx context.Context, req model.CascadeSpin) (*model.CascadeSpinResult, error)
	BuyBonus(amount int) error
	Deposit(amount int) error
	CheckData() (*model.CascadeData, error)
}

type AuthService interface {
	Register(ctx context.Context, user *model.User) (*model.AuthData, error)
	Login(ctx context.Context, login, password string) (accessToken string, sessionID string, err error)
	Refresh(ctx context.Context, sessionID string) (newAccessToken string, err error)
	Logout(ctx context.Context, sessionID string) error
}

type PaymentService interface {
	Deposit(ctx context.Context, amount int) error
	GetBalance(ctx context.Context) (int, error)
}
