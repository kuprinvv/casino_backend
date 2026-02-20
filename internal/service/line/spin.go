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
	"strconv"
)

const (
	// Барабаны
	reels = 5
	// Линии
	rows = 3
	// Максимальная выплата в кратности ставки
	maxPayoutMultiplier = 10000
	// Диапазон для обычного рандома
	normalRange = 100
	// Диапазон для бонусного рандома
	bonusRange = 1000
)

// Spin выполняет спин с учётом баланса и фриспинов
func (s *serv) Spin(ctx context.Context, spinReq model.LineSpin) (*model.SpinResult, error) {
	// Валидация ставки. Если ставка меньше либо равна нулю или не кратна 2-м (т.е. Нечетная) — ошибка
	if spinReq.Bet <= 0 || spinReq.Bet%2 != 0 {
		return nil, errors.New("bet must be positive and even")
	}

	// Получаем ID пользователя
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("user id not found in context")
	}

	// Инициализируем структуру для хранения результатов спина
	var res *model.SpinResult
	// Переменная для хранения пресета весов символов, который будет выбран в зависимости от наличия фриспинов
	var presetCfg servModel.RTPPreset

	// Начало транзакции где выполняется процесс спина.
	err := s.txManager.Do(ctx, func(txCtx context.Context) error {
		// Получаем текущее количество фриспинов внутри транзакции
		countFreeSpins, err := s.repo.GetFreeSpinCount(txCtx, userID)
		// TODO: вот с этой хуйнёй что-то сделать
		if err != nil {
			// Елси этих данных нет, то значит создаем их по умолчанию
			err = s.repo.CreateLineGameState(ctx, userID)
			if err != nil {
				log.Println(err)
				return errors.New("failed to get count free spins in Line Repo")
			}
			countFreeSpins = 0
		}

		// Получаем баланс пользователя один раз
		userBalance, err := s.userRepo.GetBalance(txCtx, userID)
		if err != nil {
			return errors.New("failed to get user balance")
		}
		if userBalance < spinReq.Bet {
			return errors.New("not enough balance")
		}

		// Платный спин
		if countFreeSpins == 0 {
			// Списание ставки, обновление баланса пользователя
			userBalance -= spinReq.Bet
			// Списываем деньги с баланса
			if err := s.userRepo.UpdateBalance(txCtx, userID, userBalance); err != nil {
				return errors.New("failed to update user balance")
			}
			// Получаем пресет весов символов исходя из статистики
			presetCfg = servModel.RtpPresets[s.lineStatsRepo.CasinoState().PresetIndex]

			// КЛЮЧЕВОЙ ВЫЗОВ: ОБЫЧНЫЙ СПИН
			// ВЫПОЛНЯЕМ СПИН
			res, err = s.SpinOnce(ctx, userID, spinReq.Bet, presetCfg, s.generateBoard, s.evaluateLines)
			if err != nil {
				return err
			}

		} else { // Иначе режим фриспинов.
			// Уменьшаем счетчик фриспинов на 1
			if err := s.repo.UpdateFreeSpinCount(txCtx, userID, countFreeSpins-1); err != nil {
				return errors.New("failed to update count free spins")
			}

			// КЛЮЧЕВОЙ ВЫЗОВ: ФРИСПИН
			// ВЫПОЛНЯЕМ СПИН
			res, err = s.SpinOnce(ctx, userID, spinReq.Bet, servModel.BonusRtpPreset, s.generateBoardWithWilds, s.evaluateLinesWithWilds)
			if err != nil {
				return err
			}
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

// ------- ОБЫЧНЫЙ СПИН -------

// SpinOnce выполняет один спин (возвращает единый SpinResult)
func (s *serv) SpinOnce(
	ctx context.Context,
	userID int,
	bet int,
	preset servModel.RTPPreset,
	generateBoard func(preset servModel.RTPPreset, ctx context.Context, userID int) [5][3]string,
	evaluateLines func(board [5][3]string, bet int) []model.LineWin,
) (*model.SpinResult, error) {
	// Генерация игрового поля
	board := generateBoard(preset, ctx, userID)

	// Подсчет символов бонуса "B" на игровом поле
	bonusCount := s.bonusSymbolCount(board)

	// Выигрыши по линиям
	lineWins := evaluateLines(board, bet)
	lineTotalPayout := s.TotalPayoutLines(lineWins)

	// Общая выплата за спин
	total := s.applyMaxPayout(lineTotalPayout, bet, maxPayoutMultiplier)

	// Считает сколько дается фриспинов за символы бонуски (если 3 и более) — по таблице FreeSpinsScatter
	// Фриспины за бонус-символы
	countFreeSpins := s.countBonusSpin(bonusCount)

	return &model.SpinResult{
		Board:            board,
		LineWins:         lineWins,
		ScatterCount:     bonusCount,
		AwardedFreeSpins: countFreeSpins,
		TotalPayout:      total,
	}, nil
}

// generateBoard генерирует игровое поле матрицы 5x3
func (s *serv) generateBoard(preset servModel.RTPPreset, ctx context.Context, userID int) [5][3]string {
	var board [5][3]string
	for r := 0; r < reels; r++ {
		// Выбор пресета для барабана
		reelProbs := preset.Probabilities[r]
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
func getSymbolFromProbs(probs map[string]int) (string, error) {
	total := 0
	for _, prob := range probs {
		total += prob
	}

	if total != normalRange {
		return "", fmt.Errorf("сумма вероятностей должна быть 100, got %d", total)
	}

	num := rand.Intn(normalRange) + 1
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

// evaluateLines выполняет оценку выигрышных линий
func (s *serv) evaluateLines(board [5][3]string, bet int) []model.LineWin {
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

// applyMaxPayout применяет лимит по максимальному выигрышу
func (s *serv) applyMaxPayout(amount, bet, maxMult int) int {
	maxPay := maxMult * bet
	if amount > maxPay {
		return maxPay
	}
	return amount
}

// countBonusSpin считает сколько дается фриспинов за символы бонуски
func (s *serv) countBonusSpin(bonusCount int) int {
	awardedFreeSpins := 0
	if bonusCount >= 3 {
		if v, ok := servModel.FreeSpinsScatter[bonusCount]; ok {
			awardedFreeSpins = v
		}
	}
	return awardedFreeSpins
}

// ------------- Функции фриспина -------------

func (s *serv) generateBoardWithWilds(preset servModel.RTPPreset, ctx context.Context, userID int) [5][3]string {
	var board [5][3]string

	wildData, err := s.repo.GetWildData(ctx, userID)
	if err != nil {
		return [5][3]string{}
	}
	for _, wild := range wildData {
		board[wild.Reel][wild.Row] = "W" + strconv.Itoa(wild.Multiplier)
	}

	for r := 0; r < reels; r++ {
		// Выбор пресета для барабана
		reelProbs := preset.Probabilities[r]

		for i := 0; i < rows; i++ {
			// Если на этой позиции уже есть символ пропускаем генерацию
			if board[r][i] != "" {
				continue
			}

			// Генерация символа для текущей позиции
			symbol, err := s.getSymbolFromBonusProbs(reelProbs)
			if err != nil {
				return [5][3]string{}
			}

			// Добавляем символ на игровое поле
			board[r][i] = symbol

			// Если это Wild - добавляем его в список новых Wild для залипания
			if isWild(symbol) {
				multiplier := servModel.WildMultiplier[symbol]
				wildData = append(wildData, model.WildData{
					Reel:       r,
					Row:        i,
					Multiplier: multiplier,
				})
				err = s.repo.UpdateWildData(ctx, userID, wildData)
				if err != nil {
					return [5][3]string{}
				}
			}
		}

	}

	return board
}

func (s *serv) getSymbolFromBonusProbs(probs map[string]int) (string, error) {
	total := 0
	for _, prob := range probs {
		total += prob
	}

	if total != bonusRange {
		return "", fmt.Errorf("сумма вероятностей должна быть 1000, got %d", total)
	}

	num := rand.Intn(bonusRange) + 1
	cumulative := 0

	for sym, prob := range probs {
		cumulative += prob
		if num <= cumulative {
			return sym, nil
		}
	}

	return "", errors.New("не удалось выбрать символ")
}

func (s *serv) evaluateLinesWithWilds(board [5][3]string, bet int) []model.LineWin {
	// Массив для хранения выигрышных линий
	var wins []model.LineWin

	for i, line := range servModel.PlayLines {
		// Массив для хранения символов на текущей линии
		symbols := make([]string, reels)
		sumMultipliers := 0 // Сумма множителей от Wild на линии
		countWilds := 0     // Счетчик Wild на линии

		// Проходим по каждому барабану и собираем символы для текущей линии
		for r := 0; r < reels; r++ {
			symbol := board[r][line[r]]    // Получаем символ с игрового поля для текущей линии
			symbols[r] = board[r][line[r]] // Сохраняем символ в массиве для линии

			// Если символ - Wild, добавляем его множитель к сумме
			if isWild(symbol) {
				sumMultipliers += servModel.WildMultiplier[symbol]
				countWilds++
			}
		}

		// Находим базовый символ (не W и не B) !!!
		var base string
		for _, sym := range symbols {
			if !isWild(sym) && sym != "B" {
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
			if sym == base || isWild(sym) {
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
					var Payouts int

					// Если есть Wild, то рассчитываем по формуле
					if countWilds > 0 {
						// Пояснение формулы:
						// coeffFromTable - базовый коэффициент из таблицы для данного количества символов
						// baseCoeff - базовый коэффициент для одного символа (без Wild)
						// bonusCoeff - итоговый коэффициент с учетом Wild и их множителей
						coeffFromTable := float64(val) / 100.0
						baseCoeff := float64(count) / coeffFromTable
						bonusCoeff := coeffFromTable - (baseCoeff * float64(countWilds)) + (baseCoeff * float64(sumMultipliers))
						Payouts = int(bonusCoeff * float64(bet))
					} else { // Если нет Wild, то просто по таблице
						Payouts = val * bet / 100
					}

					win := model.LineWin{
						Line:   i + 1,
						Symbol: base,
						Count:  count,
						Payout: Payouts,
					}
					wins = append(wins, win)

				}
			}
		}
	}
	return wins
}

func isWild(symbol string) bool {
	switch symbol {
	case "W2", "W3", "W4", "W5":
		return true
	default:
		return false
	}
}
