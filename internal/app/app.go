package app

import (
	"casino_backend/internal/config"
	"context"
	"log"
	"net/http"
)

type App struct {
	ServiceProvider *ServiceProvider
}

func NewApp() *App {
	return &App{}
}

func (s *App) initServiceProvider() {
	s.ServiceProvider = newServiceProvider()
}

func (s *App) Run() error {
	err := config.Load(".env")
	if err != nil {
		log.Printf("Error loading .env file: %v", err)
	}
	s.initServiceProvider()

	ctx := context.Background()
	r := s.ServiceProvider.Router(ctx)

	log.Printf("starting server at %s", s.ServiceProvider.HTTPCfg().Address())
	err = http.ListenAndServe(s.ServiceProvider.HTTPCfg().Address(), r)
	if err != nil {
		return err
	}
	return err
}
