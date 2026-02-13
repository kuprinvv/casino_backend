package line

import (
	"casino_backend/internal/middleware"
	"casino_backend/internal/model"
	servModel "casino_backend/internal/service/line/model"
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
)

const (
	// Барабаны
	reels = 5
	// Линии
	rows = 3
	// Максимальная выплата в кратности ставки
	maxPayoutMultiplier = 10000
)

// Spin выполняет спин с учётом баланса и фриспинов
func (s *serv) Spin(ctx context.Context, spinReq model.LineSpin) (*model.SpinResult, error) {
	// Валидация ставки
	// Если ставка меньше либо равна нулю или не кратна 2-м (т.е. Нечетная) — ошибка
	if spinReq.Bet <= 0 || spinReq.Bet%2 != 0 {
		return nil, errors.New("bet must be positive and even")
	}

	// Получаем ID пользователя
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("user id not found in context")
	}

	// Получаем пресет весов символов исходя из статистики
	presetCfg := servModel.RtpPresets[s.lineStatsRepo.CasinoState().PresetIndex]

	// Инициализируем структуру для хранения результатов спина
	var res *model.SpinResult

	// Начало транзакции где выполняется процесс спина.
	err := s.txManager.Do(ctx, func(txCtx context.Context) error {
		// Получаем текущее количество фриспинов внутри транзакции
		countFreeSpins, err := s.repo.GetFreeSpinCount(txCtx, userID)
		if err != nil {
			// Елси этих данных нет, то значит создаем их по умолчанию
			err = s.repo.CreateLineGameState(ctx, userID)
			if err != nil {
				log.Println(err)
				return errors.New("failed to get count free spins in Line Repo")
			}
			countFreeSpins = 0
		}

		// Локальная переменная для баланса
		var userBalance int

		//TODO Бойлерплейт ниже. Два раза получаем баланс когда можно получить один раз до условия

		// Платный спин
		// Если счетчик фриспинов нулевой, то списываем деньги с баланса
		if countFreeSpins == 0 {
			// Получаем баланс пользователя
			userBalance, err = s.userRepo.GetBalance(txCtx, userID)
			if err != nil {
				return errors.New("failed to get user balance")
			}
			if userBalance < spinReq.Bet {
				return errors.New("not enough balance")
			}

			// Списание ставки, обновление баланса пользователя
			userBalance -= spinReq.Bet
			if err := s.userRepo.UpdateBalance(txCtx, userID, userBalance); err != nil {
				return errors.New("failed to update user balance")
			}
		} else { // Иначе режим фриспинов.
			// Уменьшаем счетчик фриспинов на 1
			if err := s.repo.UpdateFreeSpinCount(txCtx, userID, countFreeSpins-1); err != nil {
				return errors.New("failed to update count free spins")
			}
			// Получаем баланс для последующего начисления
			userBalance, err = s.userRepo.GetBalance(txCtx, userID)
			if err != nil {
				return errors.New("failed to get user balance")
			}
		}

		// КЛЮЧЕВОЙ ВЫЗОВ
		// Делаем спин (передаём countFreeSpins как параметр)
		res, err = s.SpinOnce(spinReq.Bet, presetCfg, s.GenerateBoard)
		if err != nil {
			return err
		}

		// Устанавливаем флаг InFreeSpin, если это был фриспин
		if countFreeSpins > 0 {
			res.InFreeSpin = true
		}

		// Начисление выигрыша
		userBalance += res.TotalPayout
		if err := s.userRepo.UpdateBalance(txCtx, userID, userBalance); err != nil {
			return errors.New("failed to update user balance")
		}

		// Если есть выигранные фриспины, добавляем их
		if res.AwardedFreeSpins > 0 {
			// Получаем текущее количество фриспинов (после возможного уменьшения)
			currentFree, err := s.repo.GetFreeSpinCount(txCtx, userID)
			if err != nil {
				return errors.New("failed to get count free spins")
			}
			// Прибавляем новые спины
			if err := s.repo.UpdateFreeSpinCount(txCtx, userID, currentFree+res.AwardedFreeSpins); err != nil {
				return errors.New("failed to update count free spins")
			}
			// Обновляем то, что увидит клиент
			res.FreeSpinCount = currentFree + res.AwardedFreeSpins
		}

		// Получаем финальное количество фриспинов для возврата
		freeCount, err := s.repo.GetFreeSpinCount(txCtx, userID)
		if err != nil {
			return errors.New("failed to get count free spins")
		}

		// Устанавливаем финальные значения в res
		res.Balance = userBalance
		res.FreeSpinCount = freeCount // Финальное значение (перезапишет, если было awarded)

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Обновляем статистику
	s.lineStatsRepo.UpdateState(float64(spinReq.Bet), float64(res.TotalPayout))

	// АВТОМАТИЧЕСКАЯ РЕГУЛИРОВКА
	s.lineStatsRepo.SmartAutoAdjust()

	return res, nil
}

// SpinOnce выполняет один спин (возвращает единый SpinResult)
func (s *serv) SpinOnce(bet int, preset servModel.RTPPreset, generateBoard func(preset servModel.RTPPreset) [5][3]string) (*model.SpinResult, error) {
	// Генерация игрового поля
	board := generateBoard(preset)

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

// GenerateBoard генерирует игровое поле матрицы 5x3
func (s *serv) GenerateBoard(preset servModel.RTPPreset) [5][3]string {
	var board [5][3]string
	for r := 0; r < reels; r++ {
		// Выбор пресета для барабана
		reelProbs := preset.Probabilities[r]
		//
		f_bonus := false

		for i := 0; i < rows; i++ {
			symbol, _ := getSymbolFromProbs(reelProbs)
			if (r == 1 || r == 2 || r == 3) && (symbol == "W") {
				board[r][0], board[r][1], board[r][2] = "W", "W", "W"
				break
			}
			if f_bonus && symbol == "B" {
				symbol, _ = getSymbolFromProbs(reelProbs)
			}
			if symbol == "B" {
				f_bonus = true
			}
			board[r][i] = symbol
		}

	}
	return board
}

// Выбор символа на основе вероятностей
// Функции симуляции
func getSymbolFromProbs(probs map[string]int) (string, error) {
	total := 0
	for _, prob := range probs {
		total += prob
	}

	if total != 100 {
		return "", fmt.Errorf("сумма вероятностей должна быть 100, got %d", total)
	}

	num := rand.Intn(100) + 1
	cumulative := 0

	for sym, prob := range probs {
		cumulative += prob
		if num <= cumulative {
			return sym, nil
		}
	}

	return "", errors.New("не удалось выбрать символ")
}

// bonusSymbolCount подсчет колличества бонусных символов
func (s *serv) bonusSymbolCount(board [5][3]string) int {
	count := 0
	for r := 0; r < 5; r++ {
		for c := 0; c < 3; c++ {
			if board[r][c] == "B" {
				count++
			}
		}
	}
	return count
}

// EvaluateLines выполняет оценку выигрышных линий
func (s *serv) EvaluateLines(board [5][3]string, bet int) []model.LineWin {
	// Массив для хранения выигрышных линий
	var wins []model.LineWin

	for i, line := range servModel.PlayLines {
		// Заполняем по линиям
		symbols := make([]string, reels)
		for r := 0; r < reels; r++ {
			symbols[r] = board[r][line[r]]
		}

		// Находим базовый символ (не W и не B) !!!
		var base string
		for _, sym := range symbols {
			if sym != "W" && sym != "B" {
				base = sym
				break
			}
		}
		if base == "" {
			continue
		}

		// Считаем последовательность base + W с первого барабана
		count := 0
		for _, sym := range symbols {
			if sym == base || sym == "W" {
				count++
			} else {
				break
			}
		}

		// Определяем минимальное количество символов для выплаты
		minCount := 3
		for c := range servModel.PayoutTable[base] {
			if c < minCount {
				minCount = c // обновится до 2 для S8
			}
		}

		// Если количество совпадений больше или равно минимальному, то проверяем выплату
		if count >= minCount {
			if payTable, ok := servModel.PayoutTable[base]; ok {
				if val, ok := payTable[count]; ok {
					win := model.LineWin{
						Line:   i + 1,
						Symbol: base,
						Count:  count,
						Payout: val * bet / 100,
					}
					wins = append(wins, win)
				}
			}
		}
	}
	return wins
}

// TotalPayoutLines подсчет выплаты за линии
func (s *serv) TotalPayoutLines(lineWins []model.LineWin) int {
	lineTotal := 0
	for _, w := range lineWins {
		lineTotal += w.Payout
	}
	return lineTotal
}

// ApplyMaxPayout применяет лимит по максимальному выигрышу
func (s *serv) ApplyMaxPayout(amount, bet, maxMult int) int {
	maxPay := maxMult * bet
	if amount > maxPay {
		return maxPay
	}
	return amount
}

// CountBonusSpin считает сколько дается фриспинов за символы бонуски
func (s *serv) CountBonusSpin(bonusCount int) int {
	awardedFreeSpins := 0
	if bonusCount >= 3 {
		if v, ok := servModel.FreeSpinsScatter[bonusCount]; ok {
			awardedFreeSpins = v
		}
	}
	return awardedFreeSpins
}
