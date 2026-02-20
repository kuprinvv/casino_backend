package line_repo

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
	table          = "line_game_state"
	playerId       = "user_id"
	freeSpinsCount = "free_spins_count"
	wildData       = "wild_data"
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
		Where(sq.Eq{playerId: id}).
		PlaceholderFormat(sq.Dollar)

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
		Where(sq.Eq{playerId: id}).
		PlaceholderFormat(sq.Dollar)

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
			Values(id, count).
			PlaceholderFormat(sq.Dollar)

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

func (r *repo) CreateLineGameState(ctx context.Context, id int) error {
	// Формируем запрос на вставку, если записи не существует
	query := sq.Insert(table).
		Columns(playerId, freeSpinsCount).
		Values(id, 0).
		Suffix("ON CONFLICT (" + playerId + ") DO NOTHING").
		PlaceholderFormat(sq.Dollar)

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

// GetWildData - получение данных о вайлдах у пользователя. Возвращает пустой слайс, если записи нет
func (r *repo) GetWildData(ctx context.Context, id int) ([]model.WildData, error) {
	// Формируем запрос
	query := sq.Select(wildData).
		From(table).
		Where(sq.Eq{playerId: id}).
		PlaceholderFormat(sq.Dollar)

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	var rawData [][]int
	err = r.dbc.QueryRow(ctx, sqlStr, args...).Scan(&rawData)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []model.WildData{}, nil
		}
		return nil, err
	}

	// Конвертируем [][]float64 в []model.WildData
	var wildDatas []model.WildData
	for _, item := range rawData {
		if len(item) != 3 {
			return nil, errors.New("invalid wild data structure: expected 3 elements per array")
		}
		wildDatas = append(wildDatas, model.WildData{
			Reel:       item[0],
			Row:        item[1],
			Multiplier: item[2],
		})
	}

	return wildDatas, nil
}

// UpdateWildData - обновление данных о вайлдах у пользователя. Если записи нет, создается новая с указанными данными
func (r *repo) UpdateWildData(ctx context.Context, id int, data []model.WildData) error {
	// Конвертируем []model.WildData в [][]float64
	var rawData [][]int
	for _, wd := range data {
		rawData = append(rawData, []int{wd.Reel, wd.Row, wd.Multiplier})
	}

	// Формируем запрос на обновление
	query := sq.Update(table).
		Set(wildData, rawData).
		Where(sq.Eq{playerId: id}).
		PlaceholderFormat(sq.Dollar)

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
			Columns(playerId, freeSpinsCount, wildData).
			Values(id, 0, rawData).
			PlaceholderFormat(sq.Dollar)

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
