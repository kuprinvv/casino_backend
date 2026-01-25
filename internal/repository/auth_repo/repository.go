package auth_repo

import (
	"casino_backend/internal/model"
	"casino_backend/internal/repository"
	"context"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	table          = "sessions"
	colSessionID   = "session_id"
	colUserID      = "user_id"
	colRefreshHash = "refresh_hash"
	colExpiredTime = "expired_time"
)

type repo struct {
	dbc *pgxpool.Pool
}

func NewAuthRepository(dbc *pgxpool.Pool) repository.AuthRepository {
	return &repo{
		dbc: dbc,
	}
}

// CreateSession - создает сессию в БД
// Принимает model.Session - (ID, UserID, RefreshToken, ExpiresAt)
func (r *repo) CreateSession(ctx context.Context, session *model.Session) error {
	// Формируем запрос
	query := sq.Insert(table).
		Columns(colSessionID, colUserID, colRefreshHash, colExpiredTime).
		Values(session.ID, session.UserID, session.RefreshToken, session.ExpiresAt)

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = r.dbc.Exec(ctx, sqlStr, args...)
	if err != nil {
		return err
	}

	return nil
}

// GetRefreshTokenBySessionID - получить refresh token по session ID из БД
// Возвращает refresh token в виде строки
func (r *repo) GetRefreshTokenBySessionID(ctx context.Context, sessionID string) (string, error) {
	// Формируем запрос
	query := sq.Select(colRefreshHash).
		From(table).
		Where(sq.Eq{colSessionID: sessionID})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return "", err
	}

	var refreshHash string
	err = r.dbc.QueryRow(ctx, sqlStr, args...).Scan(&refreshHash)
	if err != nil {
		return "", err
	}

	return refreshHash, nil
}

// GetUserIDBySessionID - получить ID пользователя по session ID из БД
// возвращает ID пользователя
func (r *repo) GetUserIDBySessionID(ctx context.Context, sessionID string) (int, error) {
	// Формируем запрос
	query := sq.Select(colUserID).
		From(table).
		Where(sq.Eq{colSessionID: sessionID})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	var userID int
	err = r.dbc.QueryRow(ctx, sqlStr, args...).Scan(&userID)
	if err != nil {
		return 0, err
	}

	return userID, nil
}

// DeleteSession - удаляет сессию из БД.
// Принимает sessionID которую надо удалить
func (r *repo) DeleteSession(ctx context.Context, sessionID string) error {
	// Формируем запрос
	query := sq.Delete(table).
		Where(sq.Eq{colSessionID: sessionID})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = r.dbc.Exec(ctx, sqlStr, args...)
	if err != nil {
		return err
	}

	return nil
}

// GetUserBySessionID - возвращает model пользователя (ID, Name, Login, Password, Balance) по session ID
func (r *repo) GetUserBySessionID(ctx context.Context, sessionID string) (*model.User, error) {
	// Формируем запрос
	query := sq.Select("u.id", "u.name", "u.login", "u.password_hash", "u.balance").
		From(table + " s").
		Join("users u ON s." + colUserID + " = u.id").
		Where(sq.Eq{"s." + colSessionID: sessionID})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	var user model.User
	var balance int64
	err = r.dbc.QueryRow(ctx, sqlStr, args...).Scan(&user.ID, &user.Name, &user.Login, &user.Password, &balance)
	if err != nil {
		return nil, err
	}

	user.Balance = uint(balance)
	return &user, nil
}
