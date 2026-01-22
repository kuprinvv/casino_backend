package model

import (
	"github.com/golang-jwt/jwt/v5"
)

type User struct {
	ID       int
	Name     string
	Login    string
	Password string
	Balance  uint
}

type UserClaims struct {
	jwt.RegisteredClaims
}
