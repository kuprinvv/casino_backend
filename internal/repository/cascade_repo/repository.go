package cascade_repo

import (
	"casino_backend/internal/repository"
	"context"
	"encoding/json"
	"errors"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	table          = "sugar_rush_state"
	playerId       = "player_id"
	freeSpinsCount = "free_spins_count"
	mult           = "multipliers"
	hits           = "hits"
)

var (
	defltMult = [7][7]int{
		{1, 1, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1},
		{1, 1, 1, 1, 1, 1, 1},
	}
	defltHits = [7][7]int{
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0},
	}
)

type repo struct {
	dbc *pgxpool.Pool
}

func NewCascadeRepository(dbc *pgxpool.Pool) repository.CascadeRepository {
	return &repo{
		dbc: dbc,
	}
}

func (r *repo) GetFreeSpinCount(ctx context.Context, id int) (int, error) {
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
		return 0, err
	}
	return count, nil
}

func (r *repo) UpdateFreeSpinCount(ctx context.Context, id int, count int) error {
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

	if rowsAffected == 0 {
		// No row updated, so insert a new one with defaults for other fields
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

func (r *repo) GetMultiplierState(ctx context.Context, id int) ([7][7]int, [7][7]int, error) {
	query := sq.Select(mult, hits).
		From(table).
		Where(sq.Eq{playerId: id})

	sqlStr, args, err := query.ToSql()
	if err != nil {
		return defltMult, defltHits, err
	}

	var multJSON, hitsJSON []byte
	err = r.dbc.QueryRow(ctx, sqlStr, args...).Scan(&multJSON, &hitsJSON)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return defltMult, defltHits, nil
		}
		return defltMult, defltHits, err
	}

	var multipliers [7][7]int
	err = json.Unmarshal(multJSON, &multipliers)
	if err != nil {
		return defltMult, defltHits, err
	}

	var hitsArr [7][7]int
	err = json.Unmarshal(hitsJSON, &hitsArr)
	if err != nil {
		return defltMult, defltHits, err
	}

	return multipliers, hitsArr, nil
}

func (r *repo) SetMultiplierState(ctx context.Context, id int, multMtrx, hitsMtrx [7][7]int) error {
	multJSON, err := json.Marshal(multMtrx)
	if err != nil {
		return err
	}

	hitsJSON, err := json.Marshal(hitsMtrx)
	if err != nil {
		return err
	}

	query := sq.Update(table).
		Set(mult, multJSON).
		Set(hits, hitsJSON).
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

	if rowsAffected == 0 {
		insertQuery := sq.Insert(table).
			Columns(playerId, mult, hits).
			Values(id, multJSON, hitsJSON)

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

// ResetMultiplierState Сброс при начале платного спина
func (r *repo) ResetMultiplierState(ctx context.Context, id int) error {
	multJSON, err := json.Marshal(defltMult)
	if err != nil {
		return err
	}
	hitsJSON, err := json.Marshal(defltHits)
	if err != nil {
		return err
	}

	query := sq.Update(table).
		Set(mult, multJSON).
		Set(hits, hitsJSON).
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

	if rowsAffected == 0 {
		insertQuery := sq.Insert(table).
			Columns(playerId).
			Values(id)

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
