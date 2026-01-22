package auth

import (
	"casino_backend/internal/model"
	"casino_backend/pkg/pass"
	"casino_backend/pkg/token"
	"context"
	"time"

	"github.com/google/uuid"
)

var sessionTTL = time.Now().Add(time.Hour * 24 * 30)

// Register TODO Егор сделай dto для возврващаемых значений
func (s *serv) Register(ctx context.Context, user *model.User) (accessToken string, sessionID string, _ error) {
	// Хэширование пароля пользователя
	passwordHash, err := pass.HashPassword(user.Password)
	if err != nil {
		return "", "", err
	}
	user.Password = passwordHash

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
		refreshToken, err := token.GenerateRefreshToken()
		if err != nil {
			return err
		}

		// 3. Создать сессию
		err = s.authRepo.CreateSession(ctx,
			&model.Session{
				ID:           sessionID,
				UserID:       user.ID,
				RefreshToken: token.HashRefreshToken(refreshToken),
				ExpiresAt:    time.Now().Add(time.Hour * 24 * 30),
			})
		if err != nil {
			return err
		}

		// 4. Создать access токен
		AccessToken, err := token.GenerateAccessToken(user, []byte("fsfsd"), time.Minute*15)
		if err != nil {
			return err
		}

		return nil
	})
	return "", "", err
}

func generateSessionID() string {
	return uuid.New().String()
}
