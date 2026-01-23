package auth

import (
	"casino_backend/internal/model"
	"casino_backend/pkg/pass"
	"casino_backend/pkg/token"
	"context"
	"time"
)

func (s *serv) Register(ctx context.Context, user *model.User) (*model.AuthData, error) {
	// Хэширование пароля пользователя
	passwordHash, err := pass.HashPassword(user.Password)
	if err != nil {
		return nil, err
	}
	user.Password = passwordHash

	// Переменные для хранения результатов
	var (
		sessionID    string
		refreshToken string
		accessToken  string
	)

	// Начало транзакциии
	err = s.txManager.Do(ctx, func(ctx context.Context) error {
		// 1. Создать пользователя в бд
		user.ID, err = s.userRepo.CreateUser(ctx, user)
		if err != nil {
			return err
		}
		// 2. Генерация sessionID
		sessionID = generateSessionID()
		// 3. Генерация refresh токена
		refreshToken, err = token.GenerateRefreshToken()
		if err != nil {
			return err
		}

		// 4. Создать сессию
		err = s.authRepo.CreateSession(ctx,
			&model.Session{
				ID:           sessionID,
				UserID:       user.ID,
				RefreshToken: token.HashRefreshToken(refreshToken),
				ExpiresAt:    time.Now().Add(s.jwtConfig.RefreshTokenDuration()), // Время жизни refresh токена из конфигурации
			})
		if err != nil {
			return err
		}

		// 5. Создать access токен
		accessToken, err = token.GenerateAccessToken(
			user,
			s.jwtConfig.AccessTokenSecretKey(),
			s.jwtConfig.AccessTokenDuration())
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &model.AuthData{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		SessionID:    sessionID,
	}, nil
}
