package line_repo

import (
	"casino_backend/internal/repository"
	"context"
	"database/sql"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	table          = "free_spins_count"
	playerId       = "user_id"
	freeSpinsCount = "free_spins_count"
)

type repo struct {
	dbc *pgxpool.Pool
}

func NewLineRepository(dbc *pgxpool.Pool) repository.LineRepository {
	return &repo{
		dbc: dbc,
	}
}

// GetFreeSpinCount - получение количества бесплатных спинов у пользователя
// Возвращает 0, если записи нет
func (r *repo) GetFreeSpinCount(ctx context.Context, id int) (int, error) {
	// Формируем запрос
	query := sq.Select(freeSpinsCount).
		From(table).
		Where(sq.Eq{playerId: id})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	var count int
	err = r.dbc.QueryRow(ctx, sqlStr, args...).Scan(&count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}

	return count, nil
}

// UpdateFreeSpinCount - обновление количества бесплатных спинов у пользователя
// Если записи нет, создается новая с указанным количеством спинов
func (r *repo) UpdateFreeSpinCount(ctx context.Context, id int, count int) error {
	// Формируем запрос
	query := sq.Update(table).
		Set(freeSpinsCount, count).
		Where(sq.Eq{playerId: id})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return err
	}

	res, err := r.dbc.Exec(ctx, sqlStr, args...)
	if err != nil {
		return err
	}

	rowsAffected := res.RowsAffected()

	// Если rowsAffected = 0 - то записи не существует и делаем вставку
	if rowsAffected == 0 {
		insertQuery := sq.Insert(table).
			Columns(playerId, freeSpinsCount).
			Values(id, count)

		sqlStr, args, err = insertQuery.ToSql()
		if err != nil {
			return err
		}

		_, err = r.dbc.Exec(ctx, sqlStr, args...)
		if err != nil {
			return err
		}
	}
	return nil
}
