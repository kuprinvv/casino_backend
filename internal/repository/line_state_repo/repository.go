package line_state_repo

import (
	repoModel "casino_backend/internal/repository/line_state_repo/model"
	servModel "casino_backend/internal/service/line/model"
	"log"
	"math"
	"sync"
	"time"
)

// Реализация репозитория для хранения состояния казино
type StateRepo struct {
	mtx   sync.RWMutex
	state repoModel.CasinoState
}

// NewLineStatsRepository Конструктор для создания нового репозитория с начальным состоянием
func NewLineStatsRepository() *StateRepo {
	initialState := repoModel.CasinoState{
		TotalSpins:         0,
		TotalBet:           0,
		TotalPayout:        0,
		CurrentRTP:         100.0,
		TargetRTP:          95.0, // Можно сделать настраиваемым
		PresetIndex:        0,
		Adjustments:        make([]repoModel.AdjustmentLog, 0),
		EmergencyMode:      false,
		EmergencyDirection: "",
		SpinWindow:         make([]repoModel.SpinResult, 0),
		WindowRTP:          0,
		WindowSize:         3000,
	}
	return &StateRepo{
		state: initialState,
	}
}

// CasinoState Получение текущего состояния казино
// Является геттером для структуры состояния казино
// Возвращает копию структуры CasinoState
func (r *StateRepo) CasinoState() repoModel.CasinoState {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.state
}

// UpdateState Обновление состояния казино после спина
func (r *StateRepo) UpdateState(bet, payout float64) {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	r.state.TotalSpins++
	r.state.TotalBet += bet
	r.state.TotalPayout += payout
	if r.state.TotalBet > 0 {
		r.state.CurrentRTP = r.state.TotalPayout / r.state.TotalBet * 100
	}

	// Добавляем спин в окно
	spinRTP := 0.0
	if bet > 0 {
		spinRTP = payout / bet * 100
	}
	r.state.SpinWindow = append(r.state.SpinWindow, repoModel.SpinResult{
		Bet:    bet,
		Payout: payout,
		RTP:    spinRTP,
	})

	// Поддерживаем размер окна
	if len(r.state.SpinWindow) > r.state.WindowSize {
		r.state.SpinWindow = r.state.SpinWindow[1:]
	}

	// Пересчитываем RTP в окне (объеденил с функцией recalculateWindowRTP)
	var windowBet, windowPayout float64
	for _, spin := range r.state.SpinWindow {
		windowBet += spin.Bet
		windowPayout += spin.Payout
	}

	if windowBet > 0 {
		r.state.WindowRTP = windowPayout / windowBet * 100
	} else {
		r.state.WindowRTP = 0
	}
}

// SmartAutoAdjust УМНАЯ АВТОМАТИЧЕСКАЯ РЕГУЛИРОВКА RTP
func (r *StateRepo) SmartAutoAdjust() bool {
	if r.state.TotalSpins%50 == 0 && r.state.TotalSpins > 1000 {
		// 1. ЭКСТРЕННАЯ ПРОВЕРКА (отклонение > 20%)
		if r.emergencyCheck() {
			return r.applyEmergencyAdjustment()
		}
		// 2. СТАНДАРТНАЯ КОРРЕКТИРОВКА (отклонение > 5%)
		if r.standardCheck() {
			return r.applyStandardAdjustment()
		}
	}
	return false
}

// Экстренная проверка.
// Если RTP в окне отклоняется от целевого более чем на 20% - включаем экстренный режим
func (r *StateRepo) emergencyCheck() bool {
	if len(r.state.SpinWindow) < 1000 {
		return false
	}
	absoluteDiff := math.Abs(r.state.WindowRTP - r.state.TargetRTP)

	// Экстренная ситуация: отклонение > 20%
	if absoluteDiff > 20.0 {
		r.state.EmergencyMode = true

		if r.state.WindowRTP > r.state.TargetRTP {
			r.state.EmergencyDirection = "high"
		} else {
			r.state.EmergencyDirection = "low"
		}
		log.Println("Экстренная ситуация: отклонение > 20%", absoluteDiff)
		return true
	}
	// Выходим из экстренного режима
	if r.state.EmergencyMode && absoluteDiff < 10.0 {
		r.state.EmergencyMode = false
		r.state.EmergencyDirection = ""
	}
	return false
}

// Применение экстренной корректировки
// Если RTP слишком высокий - понижаем, если слишком низкий - повышаем
func (r *StateRepo) applyEmergencyAdjustment() bool {
	adjustmentReason := "Экстренная корректировка"

	var newIndex int
	if r.state.EmergencyDirection == "high" {
		if r.state.PresetIndex > 0 {
			newIndex = r.state.PresetIndex - 1
			log.Printf("%s (RTP слишком высокий: %.1f%%)", adjustmentReason, r.state.WindowRTP)
		} else {
			return false
		}
	} else {
		if r.state.PresetIndex < len(servModel.RtpPresets)-1 {
			newIndex = r.state.PresetIndex + 1
			log.Printf("%s (RTP слишком низкий: %.1f%%)", adjustmentReason, r.state.WindowRTP)
		} else {
			return false
		}
	}

	return r.applyAdjustment(newIndex, adjustmentReason)
}

// Стандартная проверка
func (r *StateRepo) standardCheck() bool {
	if len(r.state.SpinWindow) < 1000 {
		return false
	}

	if r.state.EmergencyMode {
		return false
	}

	windowDiff := math.Abs(r.state.WindowRTP - r.state.TargetRTP)

	if windowDiff > 5.0 {
		if r.state.TotalSpins > 3000 {
			return true
		}
	}

	return false
}

// Применение стандартной корректировки
func (r *StateRepo) applyStandardAdjustment() bool {
	windowDiff := r.state.WindowRTP - r.state.TargetRTP
	var newIndex int
	var reason string

	if windowDiff > 5.0 {
		if r.state.PresetIndex > 0 {
			newIndex = r.state.PresetIndex - 1
			log.Printf("RTP в окне высокий: %.1f%% (цель: %.1f%%)", r.state.WindowRTP, r.state.TargetRTP)
		} else {
			return false
		}
	} else if windowDiff < -5.0 {
		if r.state.PresetIndex < len(servModel.RtpPresets)-1 {
			newIndex = r.state.PresetIndex + 1
			log.Printf("RTP в окне низкий: %.1f%% (цель: %.1f%%)", r.state.WindowRTP, r.state.TargetRTP)
		} else {
			return false
		}
	} else {
		return false
	}

	return r.applyAdjustment(newIndex, reason)
}

func (r *StateRepo) applyAdjustment(newIndex int, reason string) bool {
	if newIndex == r.state.PresetIndex || newIndex < 0 || newIndex >= len(servModel.RtpPresets) {
		return false
	}

	newPreset := servModel.RtpPresets[newIndex].Name

	profit := r.state.TotalBet - r.state.TotalPayout

	adjustment := repoModel.AdjustmentLog{
		Timestamp: time.Now(),
		NewPreset: newPreset,
		Reason:    reason,
		WindowRTP: r.state.WindowRTP,
		Profit:    profit,
	}
	r.state.Adjustments = append(r.state.Adjustments, adjustment)

	r.state.PresetIndex = newIndex

	return true
}
