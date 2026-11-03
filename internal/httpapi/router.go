package httpapi

import (
	"context"
	"net/http"
)

type Deps struct {
	PingDB func(context.Context) error
}

func NewRouter(deps Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("GET /version", handleVersion)
	mux.HandleFunc("POST /v1/echo", handleEcho)
	mux.HandleFunc("GET /readyz", handleReady(deps.PingDB))
	return mux
}
