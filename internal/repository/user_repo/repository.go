package user_repo

import (
	"casino_backend/internal/model"
	"casino_backend/internal/repository"
	"context"
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	table           = "users"
	colID           = "id"
	colName         = "name"
	colLogin        = "login"
	colPasswordHash = "password_hash"
	colBalance      = "balance"
)

type repo struct {
	dbc *pgxpool.Pool
}

func NewUserRepository(dbc *pgxpool.Pool) repository.UserRepository {
	return &repo{
		dbc: dbc,
	}
}

// CreateUser - создает нового пользователя в БД.
// Возвращает ID созданного пользователя
func (r *repo) CreateUser(ctx context.Context, user *model.User) (int, error) {
	// Формируем запрос
	query := sq.Insert(table).
		Columns(colName, colLogin, colPasswordHash, colBalance).
		Values(user.Name, user.Login, user.Password, int64(user.Balance)).
		Suffix("RETURNING " + colID)

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	var id int
	err = r.dbc.QueryRow(ctx, sqlStr, args...).Scan(&id)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// GetUserByLogin - возвращает модель пользователя (ID, Name, Login, Password, Balance) по его логину
func (r *repo) GetUserByLogin(ctx context.Context, login string) (*model.User, error) {
	// Формируем запрос
	query := sq.Select(colID, colName, colLogin, colPasswordHash, colBalance).
		From(table).
		Where(sq.Eq{colLogin: login})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	var user model.User
	var balance int64
	err = r.dbc.QueryRow(ctx, sqlStr, args...).Scan(&user.ID, &user.Name, &user.Login, &user.Password, &balance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, err
	}

	user.Balance = uint(balance)
	return &user, nil
}

// GetBalance - получение баланса пользователя по его ID
// Возвращает баланс пользователя
func (r *repo) GetBalance(ctx context.Context, id int) (int, error) {
	// Формируем запрос
	query := sq.Select(colBalance).
		From(table).
		Where(sq.Eq{colID: id})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	var balance int64
	err = r.dbc.QueryRow(ctx, sqlStr, args...).Scan(&balance)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}

	return int(balance), nil
}

// UpdateBalance - обновляет баланс пользователя.
// Принимает ID пользователя и новую сумму баланса
func (r *repo) UpdateBalance(ctx context.Context, id int, amount int) error {
	// Формируем запрос
	query := sq.Update(table).
		Set(colBalance, int64(amount)).
		Where(sq.Eq{colID: id})

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
