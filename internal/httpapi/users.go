package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"financialtracker/internal/auth"
	"financialtracker/internal/users"
)

const maxCreateUserBodyBytes = 1 << 20

type createUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type createUserResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func handleCreateUser(createUser func(context.Context, string, string) (*users.User, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxCreateUserBodyBytes)

		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		var req createUserRequest
		if err := dec.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body must be valid JSON with fields email and password")
			return
		}

		email := strings.ToLower(strings.TrimSpace(req.Email))
		if !isValidEmail(email) {
			writeError(w, http.StatusBadRequest, "INVALID_EMAIL", "email must be a valid email address")
			return
		}
		if len(req.Password) < 8 {
			writeError(w, http.StatusBadRequest, "INVALID_PASSWORD", "password must be at least 8 characters")
			return
		}
		if len(req.Password) > 72 {
			writeError(w, http.StatusBadRequest, "INVALID_PASSWORD", "password must be 72 bytes or fewer")
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		user, err := createUser(ctx, email, hash)
		if err != nil {
			if errors.Is(err, users.ErrDuplicateEmail) {
				writeError(w, http.StatusConflict, "EMAIL_EXISTS", "email is already registered")
				return
			}
			writeError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(createUserResponse{
			ID:    user.ID.String(),
			Email: user.Email,
		})
	}
}

func isValidEmail(s string) bool {
	if len(s) == 0 || len(s) > 254 {
		return false
	}
	if strings.Count(s, "@") != 1 {
		return false
	}
	at := strings.IndexByte(s, '@')
	if at == 0 || at == len(s)-1 {
		return false
	}
	if strings.ContainsAny(s, " \t\r\n") {
		return false
	}

	return true
}
