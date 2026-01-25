package app

import (
	cascadeAPI "casino_backend/internal/api/cascade"
	lineAPI "casino_backend/internal/api/line"
	"casino_backend/internal/config"
	"casino_backend/internal/config/env"
	"casino_backend/internal/repository"
	"casino_backend/internal/repository/auth_repo"
	"casino_backend/internal/repository/cascade_repo"
	"casino_backend/internal/repository/cascade_stats_repo"
	"casino_backend/internal/repository/line_repo"
	"casino_backend/internal/repository/line_stats_repo"
	"casino_backend/internal/repository/user_repo"
	"casino_backend/internal/service"
	"casino_backend/internal/service/cascade"
	"casino_backend/internal/service/line"
	"context"

	trmpgx "github.com/avito-tech/go-transaction-manager/drivers/pgxv5/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2"
	"github.com/avito-tech/go-transaction-manager/trm/v2/manager"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ServiceProvider struct {
	//TXManager
	txManager trm.Manager

	// Database
	pgConfig config.PGConfig
	dbClient *pgxpool.Pool

	// Auth bits
	authRepo repository.AuthRepository

	// User bits
	userRepo repository.UserRepository

	// Line bits
	lineCfg       []config.LineConfig
	lineRepo      repository.LineRepository
	lineStatsRepo repository.LineStatsRepository
	lineServ      service.LineService // LineService ждет в конструкторе репозиторий пользователей, но его пока нет
	lineHand      *lineAPI.Handler

	// Cascade bits
	cascadeCfg       []config.CascadeConfig
	cascadeRepo      repository.CascadeRepository
	cascadeStatsRepo repository.CascadeStatsRepository
	cascadeServ      service.CascadeService
	cascadeHand      *cascadeAPI.Handler

	// Router and HTTP config
	httpCfg config.HTTPConfig
	router  chi.Router
}

func newServiceProvider() *ServiceProvider {
	return &ServiceProvider{}
}

func (sp *ServiceProvider) PgConfig() config.PGConfig {
	if sp.pgConfig == nil {
		cfg, err := env.NewPGConfig()
		if err != nil {
			panic("failed to get database config: " + err.Error())
		}
		sp.pgConfig = cfg
	}
	return sp.pgConfig
}

func (sp *ServiceProvider) DBClient(ctx context.Context) *pgxpool.Pool {
	if sp.dbClient == nil {
		dbc, err := pgxpool.New(ctx, sp.pgConfig.DSN())
		if err != nil {
			panic("failed to create db pool: " + err.Error())
		}
		err = dbc.Ping(ctx)
		if err != nil {
			panic("failed to ping db: " + err.Error())
		}
		sp.dbClient = dbc
	}
	return sp.dbClient
}

func (sp *ServiceProvider) AuthRepo(ctx context.Context) repository.AuthRepository {
	if sp.authRepo == nil {
		sp.authRepo = auth_repo.NewAuthRepository(sp.DBClient(ctx))
	}
	return sp.authRepo
}

func (sp *ServiceProvider) UserRepo(ctx context.Context) repository.UserRepository {
	if sp.userRepo == nil {
		sp.userRepo = user_repo.NewUserRepository(sp.DBClient(ctx))
	}
	return sp.userRepo
}

func (sp *ServiceProvider) TXManager(ctx context.Context) trm.Manager {
	if sp.txManager == nil {
		m, err := manager.New(trmpgx.NewDefaultFactory(sp.DBClient(ctx)))
		if err != nil {
			panic("failed to create tx manager: " + err.Error())
			return nil
		}

		sp.txManager = m
	}

	return sp.txManager
}

func (sp *ServiceProvider) LineCfg() []config.LineConfig {
	if sp.lineCfg == nil {
		cfg, err := env.NewLineConfigFromYAML("config.yaml")
		if err != nil {
			panic("failed to get line config: " + err.Error())
		}

		sp.lineCfg = cfg
	}
	return sp.lineCfg
}

func (sp *ServiceProvider) LineRepository(ctx context.Context) repository.LineRepository {
	if sp.lineRepo == nil {
		sp.lineRepo = line_repo.NewLineRepository(sp.DBClient(ctx))
	}
	return sp.lineRepo
}

func (sp *ServiceProvider) LineStatsRepository() repository.LineStatsRepository {
	if sp.lineStatsRepo == nil {
		sp.lineStatsRepo = line_stats_repo.NewLineStatsRepository()
	}
	return sp.lineStatsRepo
}

func (sp *ServiceProvider) LineService(ctx context.Context) service.LineService {
	if sp.lineServ == nil {
		// Добавить в аргументы репозиторий пользователей, когда он появится
		sp.lineServ = line.NewLineService(sp.LineCfg(), sp.LineRepository(ctx), sp.UserRepo(ctx), sp.LineStatsRepository())
	}
	return sp.lineServ
}

func (sp *ServiceProvider) LineHandler(ctx context.Context) *lineAPI.Handler {
	if sp.lineHand == nil {
		sp.lineHand = lineAPI.NewHandler(lineAPI.HandlerDeps{
			Serv: sp.LineService(ctx),
		})
	}
	return sp.lineHand
}

func (sp *ServiceProvider) CascadeCfg() []config.CascadeConfig {
	if sp.cascadeCfg == nil {
		cfg, err := env.NewCascadeConfigFromYAML("config.yaml")
		if err != nil {
			panic("failed to get cascade config: " + err.Error())
		}
		sp.cascadeCfg = cfg
	}
	return sp.cascadeCfg
}

func (sp *ServiceProvider) CascadeRepository(ctx context.Context) repository.CascadeRepository {
	if sp.cascadeRepo == nil {
		sp.cascadeRepo = cascade_repo.NewCascadeRepository(sp.DBClient(ctx))
	}
	return sp.cascadeRepo
}

func (sp *ServiceProvider) CascadeStatsRepository() repository.CascadeStatsRepository {
	if sp.cascadeStatsRepo == nil {
		sp.cascadeStatsRepo = cascade_stats_repo.NewCascadeStatsRepository()
	}
	return sp.cascadeStatsRepo
}

func (sp *ServiceProvider) CascadeService(ctx context.Context) service.CascadeService {
	if sp.cascadeServ == nil {
		sp.cascadeServ = cascade.NewCascadeService(sp.CascadeCfg(), sp.CascadeRepository(ctx), sp.UserRepo(ctx), sp.CascadeStatsRepository())
	}
	return sp.cascadeServ
}

func (sp *ServiceProvider) CascadeHandler(ctx context.Context) *cascadeAPI.Handler {
	if sp.cascadeHand == nil {
		sp.cascadeHand = cascadeAPI.NewHandler(cascadeAPI.HandlerDeps{Serv: sp.CascadeService(ctx)})
	}
	return sp.cascadeHand
}

func (sp *ServiceProvider) HTTPCfg() config.HTTPConfig {
	if sp.httpCfg == nil {
		cfg, err := env.NewHTTPConfig()
		if err != nil {
			panic("failed to get http config: " + err.Error())
		}
		sp.httpCfg = cfg
	}

	return sp.httpCfg
}

func (sp *ServiceProvider) Router(ctx context.Context) chi.Router {
	if sp.router == nil {
		r := chi.NewRouter()

		// CORS middleware
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   []string{"*"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: false,
			MaxAge:           60 * 15,
		}))

		// Line endpoints
		lineHandler := sp.LineHandler(ctx)
		r.Route("/line", func(rr chi.Router) {
			rr.Post("/spin", lineHandler.Spin)
			rr.Post("/buy-bonus", lineHandler.BuyBonus)
			rr.Post("/deposit", lineHandler.Deposit)
			rr.Get("/check-data", lineHandler.CheckData)
		})

		// Cascade endpoints
		cascadeHandler := sp.CascadeHandler(ctx)
		r.Route("/cascade", func(rr chi.Router) {
			rr.Post("/spin", cascadeHandler.Spin)
			rr.Post("/buy-bonus", cascadeHandler.BuyBonus)
			rr.Post("/deposit", cascadeHandler.Deposit)
			rr.Get("/check-data", cascadeHandler.CheckData)
		})

		sp.router = r
	}

	return sp.router
}
