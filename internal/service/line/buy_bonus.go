package line

import (
	"casino_backend/internal/middleware"
	"casino_backend/internal/model"
	servModel "casino_backend/internal/service/line/model"
	"context"
	"errors"
	"math/rand"
)

const bonusMult = 100

// BuyBonus Купить бонуску
func (s *serv) BuyBonus(ctx context.Context, bonusReq model.BonusSpin) (*model.BonusSpinResult, error) {

	// Получаем ID пользовтеля
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("user id not found")
	}

	// Ограничение покупки бонуски если есть фриспины
	countFreeSpins, err := s.repo.GetFreeSpinCount(ctx, userID)
	if err != nil {
		return nil, errors.New("error getting free spins")
	}
	if countFreeSpins > 0 {
		return nil, errors.New("free spins are not empty")
	}

	// Получаем пресет весов символов исходя из статистики
	preset := servModel.RtpPresets[s.lineStatsRepo.CasinoState().PresetIndex]

	// Инициализируем структуру для хранения результатов спина
	var res *model.BonusSpinResult

	// Начало транзакции, где выполняется процесс бонусного спина.
	err = s.txManager.Do(ctx, func(txCtx context.Context) error {
		// Получаем баланс пользователя
		balance, err := s.userRepo.GetBalance(txCtx, userID)
		if err != nil {
			return err
		}

		// Считаем цену бонуски
		bonusPrice := bonusReq.Bet * bonusMult
		// Проверяем хватает ли денег на покупку бонуски
		if balance < bonusPrice {
			return errors.New("not enough balance")
		}

		// Вычитаем из баланса цену бонуски
		balance -= bonusPrice

		spinRes, err := s.SpinOnce(ctx, userID, bonusReq.Bet, preset, s.GenerateBonusBoard, s.evaluateLines)
		if err != nil {
			return err
		}

		// начисляем выигрыш trigger spin
		balance += spinRes.TotalPayout

		// сохраняем фриспины
		err = s.repo.UpdateFreeSpinCount(txCtx, userID, spinRes.AwardedFreeSpins)
		if err != nil {
			return err
		}

		err = s.userRepo.UpdateBalance(txCtx, userID, balance)
		if err != nil {
			return err
		}

		res = &model.BonusSpinResult{
			Board:            spinRes.Board,
			LineWins:         spinRes.LineWins,
			ScatterCount:     spinRes.ScatterCount,
			AwardedFreeSpins: spinRes.AwardedFreeSpins,
			TotalPayout:      spinRes.TotalPayout,
			Balance:          balance,
			FreeSpinCount:    spinRes.AwardedFreeSpins,
		}

		return nil
	})

	return res, err
}

func (s *serv) GenerateBonusBoard(preset servModel.RTPPreset, ctx context.Context, userID int) [5][3]string {
	var board [5][3]string

	// выбираем 3 случайных барабана
	bonusReels := make(map[int]int, 3)
	for _, r := range rand.Perm(reels)[:3] {
		bonusReels[r] = rand.Intn(rows)
	}

	for r := 0; r < reels; r++ {
		// Выбор пресета для барабана
		reelProbs := preset.Probabilities[r]
		// Пытаемся получить строку на которой будет бонуска
		rowB, hasBonus := bonusReels[r]
		f_bonus := hasBonus

		for i := 0; i < rows; i++ {
			// Если это гарантированная позиция
			if hasBonus && i == rowB {
				board[r][i] = "B"
				continue
			}

			symbol, _ := getSymbolFromProbs(reelProbs)
			// W только если нет бонуса на барабане
			if (r == 1 || r == 2 || r == 3) && symbol == "W" && !f_bonus {
				board[r][0], board[r][1], board[r][2] = "W", "W", "W"
				break
			}

			// запрещаем вторую бонуску на барабане
			for f_bonus && symbol == "B" {
				symbol, _ = getSymbolFromProbs(reelProbs)
			}
			board[r][i] = symbol
		}
	}

	return board
}
