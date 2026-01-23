package line

import (
	"casino_backend/internal/model"
	"context"
	"errors"
	"math/rand"
)

var (
	// Линии выплат
	playLines = [][]int{
		{1, 1, 1, 1, 1},
		{0, 0, 0, 0, 0},
		{2, 2, 2, 2, 2},
		{0, 1, 2, 1, 0},
		{2, 1, 0, 1, 2},
		{0, 0, 1, 0, 0},
		{2, 2, 1, 2, 2},
		{1, 0, 0, 0, 1},
		{1, 2, 2, 2, 1},
		{1, 0, 1, 0, 1},
		{1, 2, 1, 2, 1},
		{0, 1, 0, 1, 0},
		{2, 1, 2, 1, 2},
		{1, 1, 0, 1, 1},
		{1, 1, 2, 1, 1},
		{0, 1, 1, 1, 2},
		{2, 1, 1, 1, 0},
		{0, 0, 1, 2, 2},
		{2, 2, 1, 0, 0},
		{1, 0, 2, 0, 1},
	}
)

const (
	// Барабаны
	reels = 5
	// Линии
	rows = 3
	// Стоимость покупки бонуса (x ставки)
	buyBonusMultiplier = 100
	// Максимальная выплата в кратности ставки
	maxPayoutMultiplier = 10000
)

// Spin выполняет спин с учётом баланса и фриспинов
func (s *serv) Spin(ctx context.Context, userID int, spinReq model.LineSpin) (*model.SpinResult, error) {
	// Валидация ставки
	// Если ставка меньше либо равна нулю или не кратна 2-м (т.е. нечетная) — ошибка
	if spinReq.Bet <= 0 || spinReq.Bet%2 != 0 {
		return nil, errors.New("bet must be positive and even")
	}

	// Получаем текущее количество фриспинов
	countFreeSpins, err := s.repo.GetFreeSpinCount(ctx, userID)
	if err != nil {
		return nil, errors.New("failed to get count free spins")
	}
	// Инициализируем результат (чтобы безопасно устанавливать InFreeSpin)
	var res *model.SpinResult

	// платный или фриспин?
	if countFreeSpins == 0 {
		userBalance, err := s.userRepo.GetBalance(ctx, userID)
		if err != nil {
			return nil, errors.New("failed to get user balance")
		}
		if userBalance < spinReq.Bet {
			return res, errors.New("not enough balance")
		}

		// Списание должно быть атомарным! Здесь просто пример — в реале делайте в транзакции.
		userBalance -= spinReq.Bet
		if err := s.userRepo.UpdateBalance(ctx, userID, userBalance); err != nil {
			return nil, errors.New("failed to update user balance")
		}
	} else {
		// фриспин — уменьшить счётчик сразу
		res = &model.SpinResult{InFreeSpin: true}
		if err := s.repo.UpdateFreeSpinCount(ctx, userID, countFreeSpins-1); err != nil {
			return nil, errors.New("failed to update count free spins")
		}
	}

	// делаем спин
	res, err = s.SpinOnce(ctx, userID, spinReq)
	if err != nil {
		return nil, err
	}

	// Если это был фриспин, сохраняем флаг
	if countFreeSpins > 0 {
		res.InFreeSpin = true
	}

	// обновляем баланс
	balance, err := s.userRepo.GetBalance(ctx, userID)
	if err != nil {
		return nil, errors.New("failed to get user balance")
	}
	balance += res.TotalPayout

	err = s.userRepo.UpdateBalance(ctx, userID, balance)
	if err != nil {
		return nil, errors.New("failed to update user balance")
	}

	// Если есть выигранные фриспины, добавляем их
	if res.AwardedFreeSpins > 0 {
		// Прибавляем новые спины к текущим
		currentFree, err := s.repo.GetFreeSpinCount(ctx, userID)
		if err == nil {
			_ = s.repo.UpdateFreeSpinCount(ctx, userID, currentFree+res.AwardedFreeSpins)
		}

		// Обновляем то, что увидит клиент (чтобы сразу показать +15 спинов и т.д.)
		res.FreeSpinCount = currentFree + res.AwardedFreeSpins
	}

	// Обновляем индекс свободных спинов в возвращаемом результате (актуально)
	freeCount, err := s.repo.GetFreeSpinCount(ctx, userID)
	if err != nil {
		return nil, errors.New("failed to get count free spins")
	}

	return &model.SpinResult{
		Board:            res.Board,
		LineWins:         res.LineWins,
		ScatterCount:     res.ScatterCount,
		ScatterPayout:    res.ScatterPayout,
		AwardedFreeSpins: res.AwardedFreeSpins,
		TotalPayout:      res.TotalPayout,
		Balance:          balance,
		FreeSpinCount:    freeCount,
		InFreeSpin:       res.InFreeSpin,
	}, nil
}

// SpinOnce выполняет один спин (возвращает единый SpinResult)
func (s *serv) SpinOnce(ctx context.Context, userID int, spinReq model.LineSpin) (*model.SpinResult, error) {
	countFreeSpins, err := s.repo.GetFreeSpinCount(ctx, userID)
	if err != nil {
		return nil, errors.New("failed to get count free spins")
	}

	board, err := s.GenerateBoard(countFreeSpins)
	if err != nil {
		return nil, err
	}

	// count scatters
	scatters := 0
	for r := 0; r < reels; r++ {
		for c := 0; c < rows; c++ {
			if board[r][c] == "B" {
				scatters++
			}
		}
	}

	// scatter payout
	var scatterPayout int
	if scatters > 0 {
		if val, ok := s.cfg.PayoutTable()["B"][scatters]; ok {
			scatterPayout = val * spinReq.Bet / 100
		}
	}

	// line wins
	lineWins := s.EvaluateLines(board, spinReq)
	var lineTotal int
	for _, w := range lineWins {
		lineTotal += w.Payout
	}

	total := s.ApplyMaxPayout(lineTotal+scatterPayout, spinReq.Bet, maxPayoutMultiplier)

	awarded := 0
	if scatters >= 3 {
		if v, ok := s.cfg.FreeSpinsByScatter()[scatters]; ok {
			awarded = v
		}
	}

	return &model.SpinResult{
		Board:            board,
		LineWins:         lineWins,
		ScatterCount:     scatters,
		ScatterPayout:    scatterPayout,
		AwardedFreeSpins: awarded,
		TotalPayout:      total,
		Balance:          0,
	}, nil
}

// GenerateBoard генерирует игровое поле матрицы 5x3
func (s *serv) GenerateBoard(countFreeSpins int) ([5][3]string, error) {
	var board [5][3]string

	// Добавляем вайлды только на центральные 3 барабана (индексы 1,2,3)
	wildReels := map[int]bool{}
	if countFreeSpins > 0 {
		// ГАРАНТИРОВАННО хотя бы один Wild каждый спин бонуски
		guaranteedReel := 1 + rand.Intn(3) // 1, 2 или 3 → барабаны 2,3,4
		wildReels[guaranteedReel] = true
		// Остальные два барабана могут тоже стать Wild с шансом 6%
		for reel := 1; reel <= 3; reel++ {
			if reel != guaranteedReel && rand.Float64() < s.cfg.WildChance() {
				wildReels[reel] = true
			}
		}
	} else { // Обычная игра — обычный шанс 6% на каждый центральный барабан
		for reel := 1; reel <= 3; reel++ {
			if rand.Float64() < s.cfg.WildChance() {
				wildReels[reel] = true
			}
		}
	}

	// Заполняем остальное случайными символами
	symbolWeights := s.cfg.SymbolWeights()

	// Отслеживаем, уже выпал ли скаттер на этом барабане
	hasScatter := make([]bool, reels) // false по умолчанию

	for r := 0; r < reels; r++ {
		for row := 0; row < rows; row++ {
			if wildReels[r] {
				board[r][row] = "W"
				continue
			}

			var sym string
			if hasScatter[r] {
				// На этом барабане уже есть скаттер → больше нельзя
				sym = s.RandomWeightedNoScatter(symbolWeights)
			} else {
				// Обычный ролл, скаттер ещё разрешён
				sym = s.RandomWeighted(symbolWeights)
			}

			board[r][row] = sym

			// Если только что выпал скаттер — помечаем барабан
			if sym == "B" {
				hasScatter[r] = true
			}
		}
	}
	return board, nil
}

// EvaluateLines выполняет оценку выигрышных линий
func (s *serv) EvaluateLines(board [5][3]string, spinReq model.LineSpin) []model.LineWin {
	var wins []model.LineWin
	for i, line := range playLines {
		symbols := make([]string, reels)
		for r := 0; r < 5; r++ {
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
		for c := range s.cfg.PayoutTable()[base] {
			if c < minCount {
				minCount = c // обновится до 2 для S8
			}
		}

		if count >= minCount {
			if payTable, ok := s.cfg.PayoutTable()[base]; ok {
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

// RandomWeighted выполняет взвешенный случайный выбор символа
func (s *serv) RandomWeighted(symbolWeights map[string]int) string {
	total := 0
	for _, w := range symbolWeights {
		total += w
	}
	if total <= 0 {
		for s := range symbolWeights {
			return s
		}
		return ""
	}
	r := rand.Intn(total)
	for s, w := range symbolWeights {
		if r < w {
			return s
		}
		r -= w
	}
	for s := range symbolWeights {
		return s
	}
	return ""
}

// RandomWeightedNoScatter — выбирает символ по весам, но полностью исключает скаттер "B"
func (s *serv) RandomWeightedNoScatter(symbolWeights map[string]int) string {
	total := 0
	for sym, w := range symbolWeights {
		if sym != "B" { // полностью игнорируем скаттер
			total += w
		}
	}
	if total <= 0 {
		for sym := range symbolWeights {
			if sym != "B" {
				return sym
			}
		}
		return ""
	}
	r := rand.Intn(total)
	current := 0
	for sym, w := range symbolWeights {
		if sym == "B" {
			continue
		}
		if r < current+w {
			return sym
		}
		current += w
	}

	// fallback
	for sym := range symbolWeights {
		if sym != "B" {
			return sym
		}
	}
	return ""
}

// ApplyMaxPayout применяет лимит по максимальному выигрышу
func (s *serv) ApplyMaxPayout(amount, bet, maxMult int) int {
	maxPay := maxMult * bet
	if amount > maxPay {
		return maxPay
	}
	return amount
}
