package cascade

import (
	"context"
	"errors"
	"math/rand"

	"casino_backend/internal/model"
)

const (
	rows = 7
	cols = 7

	// Символы: 0..6 обычные (по возрастанию ценности), 7 - бонусный (scatter-like в логике сбора)
	//symbolRegularCount = 7
	symbolBonus = 7

	// Множители на ячейках: при втором удалении x2, далее удваивается до MAX
	multiplierStart = 2
	multiplierMax   = 128 // До x128 — чтобы не переполнить int при умножении

	// Предел итераций разрешения каскадов
	maxResolveIter = 100

	// Ограничение максимального выигрыша (в кратности ставки)
	maxWinXBet = 10000
)

// Пустая ячейка
const emptyCell = -1

type cluster struct {
	symbol int
	cells  [][2]int
}

// Spin — основной метод
func (s *serv) Spin(ctx context.Context, userID int, req model.CascadeSpin) (*model.CascadeSpinResult, error) {
	if req.Bet <= 0 || req.Bet%2 != 0 {
		return nil, errors.New("bet must be positive and even")
	}

	freeSpins, err := s.repo.GetFreeSpinCount(ctx, userID)
	if err != nil {
		return nil, err
	}

	isFreeSpin := freeSpins > 0
	var balance int

	if !isFreeSpin {
		balance, err = s.userRepo.GetBalance(ctx, userID)
		if err != nil {
			return nil, err
		}
		if balance < req.Bet {
			return nil, errors.New("not enough balance")
		}
		balance -= req.Bet
		if err := s.userRepo.UpdateBalance(ctx, userID, balance); err != nil {
			return nil, err
		}
	} else {
		freeSpins--
		if err := s.repo.UpdateFreeSpinCount(ctx, userID, freeSpins); err != nil {
			return nil, err
		}
	}

	spinRes, err := s.spinOnce(ctx, userID, req.Bet, !isFreeSpin)
	if err != nil {
		return nil, err
	}

	// Начисление выигрыша
	balance, err = s.userRepo.GetBalance(ctx, userID)
	if err != nil {
		return nil, err
	}
	balance += spinRes.TotalPayout
	if err := s.userRepo.UpdateBalance(ctx, userID, balance); err != nil {
		return nil, err
	}

	// Начисление фриспинов — уже сделано внутри spinOnce → просто читаем результат
	finalFreeSpins, err := s.repo.GetFreeSpinCount(ctx, userID)
	if err != nil {
		return nil, err
	}
	spinRes.FreeSpinsLeft = finalFreeSpins

	// Заполняем индексы каскадов (0 = первый)
	for i := range spinRes.Cascades {
		spinRes.Cascades[i].CascadeIndex = i
	}

	return &model.CascadeSpinResult{
		InitialBoard:     spinRes.InitialBoard,
		Board:            spinRes.Board,
		Cascades:         spinRes.Cascades,
		TotalPayout:      spinRes.TotalPayout,
		Balance:          balance,
		ScatterCount:     spinRes.ScatterCount,
		AwardedFreeSpins: spinRes.AwardedFreeSpins,
		FreeSpinsLeft:    spinRes.FreeSpinsLeft,
		InFreeSpin:       isFreeSpin,
	}, nil
}

// spinOnce полный спин с каскадами
func (s *serv) spinOnce(ctx context.Context, userID int, bet int, resetMultipliers bool) (*model.CascadeSpinResult, error) {
	// Инициализация доски
	var board [rows][cols]int
	// hits - сколько раз ячейка участвовала в удалении кластера
	// mult - множитель клетки (x1, x2, x4, x8, x16...)
	// Загружаем состояние множителей из репозитория
	mult, hits, err := s.repo.GetMultiplierState(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Если обычный спин, то сбрасываем множители и заново их инициализируем
	if resetMultipliers {
		// Обнуляем счетчики и множители в репозитории
		if err := s.repo.ResetMultiplierState(ctx, userID); err != nil {
			return nil, err
		}
		// Загружаем заново — Reset уже поставил 1 и 0
		mult, hits, err = s.repo.GetMultiplierState(ctx, userID)
		if err != nil {
			return nil, err
		}
		// Заполняем доску заново
		s.fillBoard(&board)
	} else {
		// Фриспин — оставляем старые множители, но генерим новую доску
		s.fillBoard(&board)
		// ← Важно: множители остаются от прошлого спина!
	}

	// Сохраняем начальную доску для возврата
	initialBoard := board

	// Инициализируем каскады
	cascades := []model.CascadeStep{}
	// Общий выигрыш за спин
	var totalWin int

	for iter := 0; iter < maxResolveIter; iter++ {
		clusters := s.findClusters(board)
		if len(clusters) == 0 {
			break
		}

		step := model.CascadeStep{}

		// Обрабатываем все кластеры на доске (подсчет выигрыша, удаление, обновление множителей)
		for _, cl := range clusters {
			win := s.calculateWin(cl, mult, bet)
			totalWin += win
			avgMult := s.averageMultiplier(cl, mult)

			positions := make([]model.Position, len(cl.cells))
			for i, cell := range cl.cells {
				positions[i] = model.Position{Row: cell[0], Col: cell[1]}
			}

			step.Clusters = append(step.Clusters, model.ClusterInfo{
				Symbol:     cl.symbol,
				Cells:      positions,
				Count:      len(cl.cells),
				Payout:     win,
				Multiplier: avgMult,
			})

			s.removeCluster(cl, &board, &hits, &mult)
		}

		// Сдвигаем символы вниз и заполняем пустоты
		s.collapse(&board)
		intermediateBoard := board // Копия после collapse (upper empty)
		s.refill(&board)

		// Добавляем новые символы которые упадут на доску
		step.NewSymbols = []struct {
			model.Position
			Symbol int
		}{}
		for r := 0; r < rows; r++ {
			for c := 0; c < cols; c++ {
				if intermediateBoard[r][c] == emptyCell && board[r][c] != emptyCell {
					step.NewSymbols = append(step.NewSymbols, struct {
						model.Position
						Symbol int
					}{Position: model.Position{Row: r, Col: c}, Symbol: board[r][c]})
				}
			}
		}
		cascades = append(cascades, step)
	}

	// Сохраняем обновлённое состояние множителей
	if err := s.repo.SetMultiplierState(ctx, userID, mult, hits); err != nil {
		return nil, err
	}

	scatterCount := s.countScatters(board)
	awarded := 0
	if scatterCount >= 3 {
		if v, ok := s.cfg.BonusAwards()[scatterCount]; ok {
			awarded = v

			currentFS, err := s.repo.GetFreeSpinCount(ctx, userID)
			if err != nil {
				return nil, err
			}
			err = s.repo.UpdateFreeSpinCount(ctx, userID, currentFS+awarded)
			if err != nil {
				return nil, err
			}
		}
	}

	totalPayout := s.applyMaxPayout(totalWin, bet)

	return &model.CascadeSpinResult{
		InitialBoard:     initialBoard,
		Board:            board,
		Cascades:         cascades,
		TotalPayout:      totalPayout,
		ScatterCount:     scatterCount,
		AwardedFreeSpins: awarded,
	}, nil
}

//---------- ВСПОМОГАТЕЛЬНЫЕ МЕТОДЫ ----------

// fillBoard заполняет доску начальными символами
func (s *serv) fillBoard(board *[rows][cols]int) {
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if rand.Float64() < s.cfg.BonusProbPerColumn() {
				board[r][c] = symbolBonus
			} else {
				board[r][c] = s.randomRegularSymbol()
			}
		}
	}
}

// collapse сдвигает символы вниз, устанавливает upper empty
func (s *serv) collapse(board *[rows][cols]int) {
	for c := 0; c < cols; c++ {
		stack := make([]int, 0, rows)
		for r := 0; r < rows; r++ {
			if board[r][c] != emptyCell {
				stack = append(stack, board[r][c])
			}
		}
		for r := 0; r < rows; r++ { // Сначала очистим всю колонку
			board[r][c] = emptyCell
		}
		for i, sym := range stack {
			board[rows-len(stack)+i][c] = sym // Сдвиг вниз (bottom)
		}
		// Upper уже empty
	}
}

// refill заполняет empty (upper) новыми символами
func (s *serv) refill(board *[rows][cols]int) {
	for c := 0; c < cols; c++ {
		for r := 0; r < rows; r++ {
			if board[r][c] == emptyCell {
				if rand.Float64() < s.cfg.BonusProbPerColumn() {
					board[r][c] = symbolBonus
				} else {
					board[r][c] = s.randomRegularSymbol()
				}
			}
		}
	}
}

// randomRegularSymbol выбирает случайный обычный символ с учётом весов
func (s *serv) randomRegularSymbol() int {
	weights := s.cfg.SymbolWeights()
	total := 0
	for _, w := range weights {
		total += w
	}
	if total == 0 {
		return 0
	}
	n := rand.Intn(total)
	for sym, w := range weights {
		if n < w {
			return sym
		}
		n -= w
	}
	return 0
}

// findClusters ищет кластеры на доске
func (s *serv) findClusters(board [rows][cols]int) []cluster {
	visited := [rows][cols]bool{}
	var clusters []cluster
	dirs := [][2]int{{0, 1}, {1, 0}, {0, -1}, {-1, 0}}

	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if visited[r][c] || board[r][c] == emptyCell || board[r][c] == symbolBonus {
				continue
			}
			sym := board[r][c]
			var component [][2]int
			queue := [][2]int{{r, c}}
			visited[r][c] = true

			for len(queue) > 0 {
				cur := queue[0]
				queue = queue[1:]
				component = append(component, cur)
				cr, cc := cur[0], cur[1]
				for _, d := range dirs {
					nr, nc := cr+d[0], cc+d[1]
					if nr >= 0 && nr < rows && nc >= 0 && nc < cols &&
						!visited[nr][nc] && board[nr][nc] == sym {
						visited[nr][nc] = true
						queue = append(queue, [2]int{nr, nc})
					}
				}
			}
			if len(component) >= 5 {
				clusters = append(clusters, cluster{symbol: sym, cells: component})
			}
		}
	}
	return clusters
}

// calculateWin вычисляет выигрыш за кластер
func (s *serv) calculateWin(cl cluster, mult [rows][cols]int, bet int) int {
	// Защита от пустого кластера (на всякий случай, хотя findClusters фильтрует >=5)
	length := len(cl.cells)
	if length == 0 {
		return 0
	}

	payTable := s.cfg.PayoutTable()
	base, ok := payTable[cl.symbol]
	if !ok {
		base = 0 // или можно логгировать ошибку конфигурации
	}

	// Базовая выплата: base × количество символов
	baseWin := base * length

	// Суммируем множители по всем ячейкам кластера
	var sumMult int
	for _, cell := range cl.cells {
		sumMult += mult[cell[0]][cell[1]]
	}

	// Средний множитель (округление вниз — как в оригинале)
	avgMult := sumMult / length
	if avgMult < 1 {
		avgMult = 1
	}

	return baseWin * avgMult * bet
}

// averageMultiplier возвращает средний множитель кластера (для отображения клиенту)
func (s *serv) averageMultiplier(cl cluster, mult [rows][cols]int) int {
	length := len(cl.cells)
	if length == 0 {
		return 1
	}

	var sum int
	for _, cell := range cl.cells {
		sum += mult[cell[0]][cell[1]]
	}

	avg := sum / length
	if avg < 1 {
		avg = 1
	}
	return avg
}

// removeCluster удаляет кластер с доски и обновляет счётчики попаданий и множители
func (s *serv) removeCluster(cl cluster, board *[rows][cols]int, hits *[rows][cols]int, mult *[rows][cols]int) {
	for _, cell := range cl.cells {
		r, c := cell[0], cell[1]
		hits[r][c]++
		if hits[r][c] >= 2 {
			shift := uint(hits[r][c] - 2)
			newMult := multiplierStart << shift
			if newMult > multiplierMax {
				newMult = multiplierMax
			}
			mult[r][c] = newMult
		}
		board[r][c] = emptyCell
	}
}

// countScatters подсчитывает количество бонусных символов на доске
func (s *serv) countScatters(board [rows][cols]int) int {
	cnt := 0
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			if board[r][c] == symbolBonus {
				cnt++
			}
		}
	}
	return cnt
}

// applyMaxPayout ограничивает максимальный выигрыш
func (s *serv) applyMaxPayout(amount, bet int) int {
	maxAllowed := maxWinXBet * bet
	if amount > maxAllowed {
		return maxAllowed
	}
	return amount
}
