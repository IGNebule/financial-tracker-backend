package httpapi

import (
	"context"
	"net/http"

	"financialtracker/internal/sessions"
	"financialtracker/internal/users"

	"github.com/google/uuid"
)

type Deps struct {
	PingDB           func(context.Context) error
	CreateUser       func(context.Context, string, string) (*users.User, error)
	LoginFindUser    func(context.Context, string) (*users.User, error)
	LoginCreateToken func(context.Context, uuid.UUID) (string, error)
	FindSession      func(context.Context, string) (*sessions.Session, error)
}

func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()

	// Public routes — no authentication required.
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /version", handleVersion)
	mux.HandleFunc("GET /readyz", handleReady(deps.PingDB))
	mux.HandleFunc("POST /v1/echo", handleEcho)
	mux.HandleFunc("POST /v1/users", handleCreateUser(deps.CreateUser))
	mux.HandleFunc("POST /v1/login", handleLogin(deps.LoginFindUser, deps.LoginCreateToken))

	return mux
}
