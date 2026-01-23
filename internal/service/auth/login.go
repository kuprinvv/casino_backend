package auth

import (
	"casino_backend/internal/model"
	"casino_backend/pkg/pass"
	"casino_backend/pkg/token"
	"context"
	"errors"
	"time"
)

func (s *serv) Login(ctx context.Context, user *model.User) (*model.AuthData, error) {
	// Получение пользователя из бд по логину
	userRepo, err := s.userRepo.GetUserByLogin(ctx, user.Login)
	if err != nil {
		return nil, err
	}

	// Верификация пароля
	if !pass.VerifyPassword(userRepo.Password, user.Password) {
		return nil, errors.New("invalid password")
	}

	// Генерация sessionID
	sessionID := generateSessionID()

	// Генерация refresh токена
	refreshToken, err := token.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	// Создать сессию
	err = s.authRepo.CreateSession(ctx,
		&model.Session{
			ID:           sessionID,
			UserID:       userRepo.ID,
			RefreshToken: token.HashRefreshToken(refreshToken),
			ExpiresAt:    time.Now().Add(time.Hour * 24 * 30),
		})
	if err != nil {
		return nil, err
	}

	// Создать access токен
	accessToken, err := token.GenerateAccessToken(
		user,
		s.jwtConfig.AccessTokenSecretKey(),
		s.jwtConfig.AccessTokenDuration())
	if err != nil {
		return nil, err
	}

	return &model.AuthData{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		SessionID:    sessionID,
	}, nil
}
