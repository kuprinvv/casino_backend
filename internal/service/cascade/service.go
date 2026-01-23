package cascade

import (
	"casino_backend/internal/config"
	"casino_backend/internal/repository"
	"casino_backend/internal/service"
)

type serv struct {
	cascadeConfig config.CascadeConfig
	cascadeRepo   repository.CascadeRepository
	userRepo      repository.UserRepository
}

// NewCascade Создать новый cascade
func NewCascadeService(cfg config.CascadeConfig, repo repository.CascadeRepository, userRepo repository.UserRepository) service.CascadeService {
	return &serv{
		cascadeConfig: cfg,
		cascadeRepo:   repo,
		userRepo:      userRepo,
	}
}
