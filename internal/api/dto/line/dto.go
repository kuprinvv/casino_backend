package line

type LineSpinRequest struct {
	Bet int `json:"bet"` // Размер ставки (положительное целое, >0)
}

type LineSpinResponse struct {
	Board            [5][3]string `json:"board"`              // Символы (ID)
	LineWins         []LineWin    `json:"line_wins"`          // Выигрышные линии
	ScatterCount     int          `json:"scatter_count"`      // Кол-во скаттеров
	ScatterPayout    int          `json:"scatter_payout"`     // Выплата по скаттерам
	AwardedFreeSpins int          `json:"awarded_free_spins"` // Начислено фриспинов в этом спине
	TotalPayout      int          `json:"total_payout"`       // Общая выплата
	Balance          int          `json:"balance"`            // Баланс после
	FreeSpinCount    int          `json:"free_spin_count"`    // Остаток фриспинов
}
type BonusSpinResponse struct {
	Board            [5][3]string `json:"board"`              // Символы (ID)
	LineWins         []LineWin    `json:"line_wins"`          // Выигрышные линии
	ScatterCount     int          `json:"scatter_count"`      // Кол-во скаттеров
	ScatterPayout    int          `json:"scatter_payout"`     // Выплата по скаттерам
	AwardedFreeSpins int          `json:"awarded_free_spins"` // Начислено фриспинов в этом спине
	TotalPayout      int          `json:"total_payout"`       // Общая выплата
	Balance          int          `json:"balance"`            // Баланс после
	FreeSpinCount    int          `json:"free_spin_count"`    // Остаток фриспинов
}
type BonusSpinRequest struct {
	Bet int `json:"bet"` // Сумма покупки бонуса
}

type DepositRequest struct {
	Amount int `json:"amount"` // Сумма депозита
}

type DataResponse struct {
	Balance       int `json:"balance"`         // Баланс пользователя
	FreeSpinCount int `json:"free_spin_count"` // Остаток фриспинов
}

type LineWin struct {
	Line   int    `json:"line"`   // 1-20
	Symbol string `json:"symbol"` // ID символа
	Count  int    `json:"count"`  // 3-5
	Payout int    `json:"payout"` // Выплата
}
