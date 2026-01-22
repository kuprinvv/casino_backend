package auth

import (
	"casino_backend/internal/repository"

	"github.com/avito-tech/go-transaction-manager/trm/v2"
)

type serv struct {
	txManager trm.Manager
	userRepo  repository.UserRepository
	authRepo  repository.AuthRepository
}

func NewService() *serv {
	return &serv{}
}
