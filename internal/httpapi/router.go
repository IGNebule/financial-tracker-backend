package httpapi

import (
	"context"
	"net/http"

	"financialtracker/internal/users"
)

type Deps struct {
	PingDB     func(context.Context) error
	CreateUser func(context.Context, string, string) (*users.User, error)
}

func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /version", handleVersion)
	mux.HandleFunc("POST /v1/echo", handleEcho)
	mux.HandleFunc("GET /readyz", handleReady(deps.PingDB))
	mux.HandleFunc("POST /v1/users", handleCreateUser(deps.CreateUser))
	return mux
}
