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

	// Получаем пресет весов символов исходя из статистики
	preset := servModel.RtpPresets[s.lineStatsRepo.CasinoState().PresetIndex]

	// Инициализируем структуру для хранения результатов спина
	var res *model.BonusSpinResult

	// Начало транзакции, где выполняется процесс бонусного спина.
	err := s.txManager.Do(ctx, func(txCtx context.Context) error {
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

		spinRes, err := s.SpinBonus(bonusReq.Bet, preset)
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

// SpinBonus выполняет один спин (возвращает единый SpinResult)
func (s *serv) SpinBonus(bet int, preset servModel.RTPPreset) (*model.SpinResult, error) {
	// Генерация игрового поля
	board := s.GenerateBonusBoard(preset)

	// Подсчет символов бонуса "B" на игровом поле
	bonusCount := s.bonusSymbolCount(board)

	// Выигрыши по линиям
	lineWins := s.EvaluateLines(board, bet)
	lineTotalPayout := s.TotalPayoutLines(lineWins)

	// Общая выплата за спин
	total := s.ApplyMaxPayout(lineTotalPayout, bet, maxPayoutMultiplier)

	// Считает сколько дается фриспинов за символы бонуски (если 3 и более) — по таблице FreeSpinsScatter
	// Фриспины за бонус-символы
	countFreeSpins := s.CountBonusSpin(bonusCount)

	return &model.SpinResult{
		Board:            board,
		LineWins:         lineWins,
		ScatterCount:     bonusCount,
		AwardedFreeSpins: countFreeSpins,
		TotalPayout:      total,
		Balance:          0,
	}, nil
}

func (s *serv) GenerateBonusBoard(preset servModel.RTPPreset) [5][3]string {

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
