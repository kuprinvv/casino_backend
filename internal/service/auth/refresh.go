package auth

import (
	"casino_backend/internal/model"
	"casino_backend/pkg/token"
	"context"
	"errors"
)

func (s *serv) Refresh(ctx context.Context, data *model.AuthData) (newAccessToken string, err error) {
	// Получение хэша refresh токена из хранилища по sessionID
	refreshTokenHash, err := s.authRepo.GetRefreshTokenBySessionID(ctx, data.SessionID)
	if err != nil {
		return "", err
	}

	// Верификация переданного refresh токена с хэшем из хранилища
	if !token.VerifyRefreshToken(data.RefreshToken, refreshTokenHash) {
		return "", errors.New("invalid refresh token")
	}

	// Получение пользователя по sessionID
	user, err := s.authRepo.GetUserBySessionID(ctx, data.SessionID)
	if err != nil {
		return "", err
	}

	// Генерация нового access токена
	newAccessToken, err = token.GenerateAccessToken(
		user,
		s.jwtConfig.AccessTokenSecretKey(),
		s.jwtConfig.AccessTokenDuration())
	if err != nil {
		return "", err
	}

	return newAccessToken, nil
}
