package cascade

import (
	"context"
	"errors"
)

// Купить бонуску
func (s *serv) BuyBonus(ctx context.Context, userID int, amount int) error {
	cost := amount

	balance, err := s.userRepo.GetBalance(ctx, userID)
	if err != nil {
		return errors.New("failed to get user balance")
	}
	if balance < cost {
		return errors.New("not enough balance for bonus buy")
	}
	err = s.userRepo.UpdateBalance(ctx, userID, balance-cost)
	if err != nil {
		return errors.New("failed to update balance after bonus buy")
	}

	if err := s.repo.ResetMultiplierState(ctx, userID); err != nil {
		return errors.New("failed to reset mult state")
	}

	err = s.repo.UpdateFreeSpinCount(ctx, userID, 10)
	if err != nil {
		return errors.New("failed to update free spin count after bonus buy")
	}
	return nil
}
