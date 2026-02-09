package line

import (
	"casino_backend/internal/repository"
	"casino_backend/internal/service"

	"github.com/avito-tech/go-transaction-manager/trm/v2"
)

type serv struct {
	repo          repository.LineRepository
	userRepo      repository.UserRepository
	lineStatsRepo repository.LineStatsRepository
	txManager     trm.Manager
}

// NewLineService Создать новый слот 5x3
func NewLineService(
	repo repository.LineRepository,
	userRepo repository.UserRepository,
	lineStatsRepo repository.LineStatsRepository,
	txManager trm.Manager,
) service.LineService {
	return &serv{
		repo:          repo,
		userRepo:      userRepo,
		lineStatsRepo: lineStatsRepo,
		txManager:     txManager,
	}
}
