package converter

import (
	"casino_backend/internal/api/dto/line"
	"casino_backend/internal/model"
)

func ToLineSpin(req line.LineSpinRequest) model.LineSpin {
	return model.LineSpin{
		Bet: req.Bet,
	}
}

func ToLineSpinResponse(resp model.SpinResult) line.LineSpinResponse {
	return line.LineSpinResponse{
		Board:            resp.Board,
		LineWins:         toLineWins(resp.LineWins),
		ScatterCount:     resp.ScatterCount,
		AwardedFreeSpins: resp.AwardedFreeSpins,
		TotalPayout:      resp.TotalPayout,
		Balance:          resp.Balance,
		FreeSpinCount:    resp.FreeSpinCount,
		InFreeSpin:       resp.InFreeSpin,
	}
}

func toLineWins(lineWins []model.LineWin) []line.LineWin {
	result := make([]line.LineWin, len(lineWins))
	for i, l := range lineWins {
		result[i] = line.LineWin{
			Line:   l.Line,
			Symbol: l.Symbol,
			Count:  l.Count,
			Payout: l.Payout,
		}
	}
	return result
}

func ToDataResponse(data model.Data) line.DataResponse {
	return line.DataResponse{
		Balance:       data.Balance,
		FreeSpinCount: data.FreeSpinCount,
	}
}
