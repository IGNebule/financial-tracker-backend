package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"financialtracker/internal/auth"
	"financialtracker/internal/sessions"
	"financialtracker/internal/users"
)

const maxLoginBodyBytes = 1 << 20

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// handleLogin returns a handler that verifies credentials and issues a
// session token. It does not require an existing session.
func handleLogin(
	findUser func(context.Context, string) (*users.User, error),
	createSession func(context.Context, uuid.UUID) (string, error),
) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxLoginBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		var req loginRequest
		if err := dec.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_JSON",
				"request body must be valid JSON with fields email and password")
			return
		}

		email := strings.ToLower(strings.TrimSpace(req.Email))
		if email == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "INVALID_CREDENTIALS",
				"email and password are required")
			return
		}

		user, err := findUser(r.Context(), email)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidPassword) {
				writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS",
					"invalid email or password")
				return
			}
			// User not found — return the SAME response as wrong password.
			// This prevents user enumeration: an attacker cannot tell whether
			// an email is registered by comparing response bodies or codes.
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS",
				"invalid email or password")
			return
		}

		if err := auth.CheckPassword(user.PasswordHash, req.Password); err != nil {
			// Same generic response as user-not-found.
			writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS",
				"invalid email or password")
			return
		}

		token, err := createSession(r.Context(), user.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(loginResponse{
			Token:     token,
			ExpiresAt: time.Now().Add(sessions.DefaultLifetime),
		})
	}
}

// ctxKeyUserID is unexported so no other package can put its own value
// under the same key. It is a distinct type, not a string, so even within
// this package a typo would not collide.
type ctxKeyUserID struct{}

// WithUserID stores the authenticated user ID on ctx. It is used by the auth
// middleware; handlers retrieve it via UserIDFromContext.
func WithUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxKeyUserID{}, userID)
}

// UserIDFromContext retrieves the authenticated user ID. The bool is false
// if the context has no user — which should be impossible inside a
// protected route, but the caller must still handle it.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxKeyUserID{}).(uuid.UUID)
	return id, ok
}

// requireAuth wraps next with a middleware that validates
// `Authorization: Bearer <token>` and puts the user ID into the request
// context. Requests without a valid token get 401 and never reach next.
func requireAuth(
	findSession func(context.Context, string) (*sessions.Session, error),
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := parseBearerToken(r.Header.Get("Authorization"))
			if !ok {
				writeError(w, http.StatusUnauthorized, "MISSING_TOKEN",
					"Authorization header with a Bearer token is required")
				return
			}
			sess, err := findSession(r.Context(), token)
			if err != nil {
				// Every failure mode — expired, unknown, DB error — returns
				// the same 401 body. Distinguishing them would leak whether a
				// token ever existed.
				writeError(w, http.StatusUnauthorized, "INVALID_TOKEN",
					"session token is invalid or expired")
				return
			}
			ctx := WithUserID(r.Context(), sess.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// parseBearerToken extracts the token from "Bearer <token>" or returns
// ok=false. Case-insensitive on the "Bearer" prefix; strips surrounding
// whitespace.
func parseBearerToken(header string) (string, bool) {
	const prefix = "bearer "
	if len(header) <= len(prefix) {
		return "", false
	}
	if !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}
