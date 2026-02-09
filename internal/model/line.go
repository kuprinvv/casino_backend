package model

type LineSpin struct {
	Bet int
}

type SpinResult struct {
	Board            [5][3]string
	LineWins         []LineWin
	ScatterCount     int
	AwardedFreeSpins int
	TotalPayout      int
	Balance          int
	FreeSpinCount    int
	InFreeSpin       bool
}

type LineWin struct {
	Line   int
	Symbol string
	Count  int
	Payout int
}

type Data struct {
	Balance       int // Теперь экспортировано (большая буква)
	FreeSpinCount int // Теперь экспортировано
}
