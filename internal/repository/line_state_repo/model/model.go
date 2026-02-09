package model

import "time"

// Состояние казино
type CasinoState struct {
	TotalSpins  int     // Сколько всего спинов сделано
	TotalBet    float64 // Сумма всех ставок
	TotalPayout float64 // Сумма всех выплат

	CurrentRTP float64 // Текущий RTP = (TotalPayout/TotalBet)*100
	TargetRTP  float64 // Какой RTP хотим получить (например 95%)

	PresetIndex int // Индекс текущего пресета вероятностей (0-11)

	Adjustments []AdjustmentLog // Лог изменений RTP

	EmergencyMode      bool   // Флаг режима "аварийного" изменения RTP
	EmergencyDirection string // Направление "аварийного" изменения ("up" или "down")

	SpinWindow []SpinResult // Окно последних спинов для анализа
	WindowRTP  float64      // RTP в окне последних спинов
	WindowSize int          // Размер окна для анализа RTP
}

// Лог изменений RTP
type AdjustmentLog struct {
	Timestamp time.Time
	NewPreset string
	Reason    string
	WindowRTP float64
	Profit    float64
}

// Результат спина для окна
type SpinResult struct {
	Bet    float64
	Payout float64
	RTP    float64
}
