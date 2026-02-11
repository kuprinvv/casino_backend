package line_state_repo

import (
	repoModel "casino_backend/internal/repository/line_state_repo/model"
	servModel "casino_backend/internal/service/line/model"
	"log"
	"math"
	"sync"
	"time"
)

const (
	// countSpinsToSwap Количество спинов, после которого начинаем проверять необходимость корректировки
	countSpinsToSwap = 1
	// periodSpinsToCheck Периодичность проверки (каждые N спинов)
	periodSpinsToCheck = 25
	// maxAllowedRTPDeviation Максимально допустимое отклонение RTP в окне от целевого, при котором мы считаем, что нужно корректировать
	maxAllowedRTPDeviation = 5 // процентные пункты
	// критическое отклонение RTP для активации аварийного режима
	criticalRTPDeviation = 10.0
	// нормальное отклонение RTP для деактивации аварийного режима
	normalRTPDeviation = 5
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
		CurrentRTP:         95.0,
		TargetRTP:          95.0, // Можно сделать настраиваемым
		PresetIndex:        2,
		Adjustments:        make([]repoModel.AdjustmentLog, 0),
		EmergencyMode:      false,
		EmergencyDirection: "",
		SpinWindow:         make([]repoModel.SpinResult, 0),
		WindowRTP:          0,
		WindowSize:         500,
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
	r.mtx.RLock()
	defer r.mtx.RUnlock()

	//log.Println("ТЕКУЩИЙ RTP ", r.state.CurrentRTP, " ПРЕСЕТ ИНДЕКС ", r.state.PresetIndex, " РТП ОКНА ", r.state.WindowRTP)
	if r.state.TotalSpins%periodSpinsToCheck == 0 && r.state.TotalSpins > countSpinsToSwap {
		// 1. ЭКСТРЕННАЯ ПРОВЕРКА (отклонение > 20%)
		if r.emergencyCheck() {
			return r.applyEmergencyAdjustment()
		}
		// 2. СТАНДАРТНАЯ КОРРЕКТИРОВКА (отклонение > 5%)
		if r.standardCheck() {
			// Если мы не в экстренном режиме,
			// но RTP в окне отклоняется от целевого более чем на 5% - применяем стандартную корректировку
			return r.applyStandardAdjustment()
		}
	}
	return false
}

// Экстренная проверка.
// Если RTP в окне отклоняется от целевого более чем на 20% - включаем экстренный режим
func (r *StateRepo) emergencyCheck() bool {
	// Если у нас еще нет достаточного количества спинов для анализа - не делаем ничего
	if r.state.TotalSpins < countSpinsToSwap {
		return false
	}
	// Вычисляем абсолютное отклонение RTP в окне от целевого
	absoluteDiff := math.Abs(r.state.WindowRTP - r.state.TargetRTP)

	// Экстренная ситуация: отклонение > criticalRTPDeviation
	if absoluteDiff > criticalRTPDeviation {
		r.state.EmergencyMode = true
		// Определяем направление корректировки: если RTP слишком высокий - понижаем, если слишком низкий - повышаем
		if r.state.WindowRTP > r.state.TargetRTP {
			r.state.EmergencyDirection = "high"
		} else {
			r.state.EmergencyDirection = "low"
		}
		return true
	}
	// Выходим из экстренного режима
	// Если мы в экстренном режиме, но RTP уже вернулся ближе к целевому (отклонение < normalRTPDeviation) - выключаем экстренный режим
	if r.state.EmergencyMode && absoluteDiff < normalRTPDeviation {
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
	if r.state.TotalSpins < countSpinsToSwap {
		return false
	}

	if r.state.EmergencyMode {
		return false
	}

	windowDiff := math.Abs(r.state.WindowRTP - r.state.TargetRTP)

	if windowDiff > maxAllowedRTPDeviation {
		if r.state.TotalSpins > countSpinsToSwap {
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

	if windowDiff > maxAllowedRTPDeviation {
		if r.state.PresetIndex > 0 {
			newIndex = r.state.PresetIndex - 1
		} else {
			return false
		}
	} else if windowDiff < -maxAllowedRTPDeviation {
		if r.state.PresetIndex < len(servModel.RtpPresets)-1 {
			newIndex = r.state.PresetIndex + 1
		} else {
			return false
		}
	} else {
		return false
	}

	return r.applyAdjustment(newIndex, reason)
}

// Применение корректировки и логирование
func (r *StateRepo) applyAdjustment(newIndex int, reason string) bool {
	if newIndex == r.state.PresetIndex || newIndex < 0 || newIndex >= len(servModel.RtpPresets) {
		return false
	}
	newPreset := servModel.RtpPresets[newIndex].Name
	log.Println("[КОРЕКТИРОВКА ПРЕСЕТА] СТАРЫЙ ПРЕСЕТ>", r.state.CurrentRTP, " НОВЫЙ ПРЕСЕТ>", newPreset)
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
