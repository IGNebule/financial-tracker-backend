package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"financialtracker/internal/config"
	"financialtracker/internal/db"
	"financialtracker/internal/httpapi"
	"financialtracker/internal/sessions"
	"financialtracker/internal/users"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run is the real entry point. main only converts its error into a non-zero
// exit. Splitting the two ensures deferred cleanup (database.Close) fires when
// startup fails after the pool is opened — log.Fatal in main would skip it.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	database, err := db.Open(context.Background(), cfg.DB)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer database.Close()

	userRepo := users.NewRepository(database.Pool())
	sessionRepo := sessions.NewRepository(database.Pool())

	server := &http.Server{
		Addr: cfg.Addr,
		Handler: httpapi.NewRouter(httpapi.Deps{
			PingDB:           database.Ping,
			CreateUser:       userRepo.Create,
			LoginFindUser:    userRepo.FindByEmail,
			LoginCreateToken: sessionRepo.Create,
			FindSession:      sessionRepo.FindByToken,
		}),
	}

	fmt.Println("listening on", cfg.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}
