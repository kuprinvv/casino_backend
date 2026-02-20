package service

import (
	"casino_backend/internal/model"
	"context"
)

type LineService interface {
	Spin(ctx context.Context, spinReq model.LineSpin) (*model.SpinResult, error)
	BuyBonus(ctx context.Context, bonusReq model.BonusSpin) (*model.BonusSpinResult, error)
}

type CascadeService interface {
	Spin(ctx context.Context, req model.CascadeSpin) (*model.CascadeSpinResult, error)
	BuyBonus(ctx context.Context, amount int) error
}

type AuthService interface {
	Register(ctx context.Context, user *model.User) (*model.AuthData, error)
	Login(ctx context.Context, user *model.User) (*model.AuthData, error)
	Refresh(ctx context.Context, data *model.AuthData) (newAccessToken string, err error)
	Logout(ctx context.Context, sessionID string) error
}

type PaymentService interface {
	Deposit(ctx context.Context, userID, amount int) error
	GetBalance(ctx context.Context, userID int) (int, error)
}
