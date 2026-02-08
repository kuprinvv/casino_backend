package line

import (
	"casino_backend/internal/middleware"
	"casino_backend/internal/model"
	servModel "casino_backend/internal/service/line/model"
	"context"
	"errors"
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
		res, err = s.SpinOnce(spinReq, presetCfg)
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
func (s *serv) SpinOnce(spinReq model.LineSpin, preset servModel.RTPPreset) (*model.SpinResult, error) {
	// Генерация игрового поля
	board := s.GenerateBoard(preset)

	// Подсчет симолов бонуски "B" на игровом поле
	bonusSymbolCount := 0
	for r := 0; r < reels; r++ {
		for c := 0; c < rows; c++ {
			if board[r][c] == "B" {
				bonusSymbolCount++
			}
		}
	}

	// Выплата за символы бонуски (если 3 и более) — по таблице выплат для символа "B"
	var scatterPayout int
	if bonusSymbolCount >= 2 {
		if val, ok := servModel.PayoutTable["B"][bonusSymbolCount]; ok {
			scatterPayout = val * spinReq.Bet / 100
		}
	}

	// line wins
	lineWins := s.EvaluateLines(board, spinReq, servModel.PayoutTable)
	var lineTotal int
	for _, w := range lineWins {
		lineTotal += w.Payout
	}

	// Общая выплата за спин (линии + бонуска) с учетом максимальной выплаты
	total := s.ApplyMaxPayout(lineTotal+scatterPayout, spinReq.Bet, maxPayoutMultiplier)

	// Считает сколько дается фриспинов за символы бонуски (если 3 и более) — по таблице FreeSpinsScatter
	awardedFreeSpins := 0
	if bonusSymbolCount >= 3 {
		if v, ok := servModel.FreeSpinsScatter[bonusSymbolCount]; ok {
			awardedFreeSpins = v
		}
	}

	return &model.SpinResult{
		Board:            board,
		LineWins:         lineWins,
		ScatterCount:     bonusSymbolCount,
		ScatterPayout:    scatterPayout,
		AwardedFreeSpins: awardedFreeSpins,
		TotalPayout:      total,
		Balance:          0,
	}, nil
}

// GenerateBoard генерирует игровое поле матрицы 5x3
func (s *serv) GenerateBoard(preset servModel.RTPPreset) [5][3]string {
	var board [5][3]string
	for r := 0; r < reels; r++ {
		// Получаем веса символов для текущего барабана из пресета
		reelProbs := preset.Probabilities[r]

		// Если на верхнем символе выпало "W" на барабанах 2, 3 или 4, то заполняем весь барабан "X"
		if r == 1 || r == 2 || r == 3 {
			for row := 0; row < 3; row++ {
				symbol := getSymbolFromProbs(reelProbs)
				if symbol == "W" {
					board[r][0], board[r][1], board[r][2] = "W", "W", "W"
					break
				}
				board[r][row] = symbol
			}
		} else {
			// Для остальных барабанов (первого и последнего) просто заполняем по весам
			for row := 0; row < 3; row++ {
				board[r][row] = getSymbolFromProbs(reelProbs)
			}

		}

	}
	return board
}

// Выбор символа на основе вероятностей
func getSymbolFromProbs(probs map[string]int) string {
	num := rand.Intn(100) + 1
	cumulative := 0

	for sym, prob := range probs {
		cumulative += prob
		if num <= cumulative {
			return sym
		}
	}

	maxProb := 0
	var maxSym string
	for sym, prob := range probs {
		if prob > maxProb {
			maxProb = prob
			maxSym = sym
		}
	}
	return maxSym
}

// EvaluateLines выполняет оценку выигрышных линий
func (s *serv) EvaluateLines(board [5][3]string, spinReq model.LineSpin, payoutTable map[string]map[int]int) []model.LineWin {
	// Массив для хранения выигрышных линий
	var wins []model.LineWin

	for i, line := range servModel.PlayLines {
		symbols := make([]string, reels)
		for r := 0; r < reels; r++ {
			symbols[r] = board[r][line[r]]
		}

		// Пропускаем линии, где первый символ — скаттер
		if symbols[0] == "B" {
			continue
		}

		// Находим базовый символ (не W и не B)
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
		for c := range payoutTable[base] {
			if c < minCount {
				minCount = c // обновится до 2 для S8
			}
		}

		// Если количество совпадений больше или равно минимальному, то проверяем выплату
		if count >= minCount {
			if payTable, ok := payoutTable[base]; ok {
				if val, ok := payTable[count]; ok {
					win := model.LineWin{
						Line:   i + 1,
						Symbol: base,
						Count:  count,
						Payout: val * spinReq.Bet / 100,
					}
					wins = append(wins, win)
				}
			}
		}
	}
	return wins
}

// ApplyMaxPayout применяет лимит по максимальному выигрышу
func (s *serv) ApplyMaxPayout(amount, bet, maxMult int) int {
	maxPay := maxMult * bet
	if amount > maxPay {
		return maxPay
	}
	return amount
}
