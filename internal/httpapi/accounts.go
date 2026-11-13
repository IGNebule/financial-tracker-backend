package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"financialtracker/internal/ledger"
	"financialtracker/internal/money"
)

const maxAccountBodyBytes = 1 << 20

type CreateAccountRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Currency string `json:"currency"`
}

type accountResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Currency  string    `json:"currency"`
	CreatedAt time.Time `json:"created_at"`
}

func toAccountResponse(a *ledger.Account) accountResponse {
	return accountResponse{
		ID:        a.ID.String(),
		Name:      a.Name,
		Type:      string(a.Type),
		Currency:  a.Currency,
		CreatedAt: a.CreatedAt,
	}
}

func handleCreateAccount(createAccount func(context.Context, uuid.UUID, string, ledger.AccountType, string) (*ledger.Account, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "MISSING_TOKEN", "authentication required")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxAccountBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		var req CreateAccountRequest
		if err := dec.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "INVALID_JSON", "request body must be valid JSON with fields name, type, currency")
			return
		}

		name := strings.TrimSpace(req.Name)
		if name == "" || len(name) > 100 {
			writeError(w, http.StatusBadRequest, "INVALID_NAME", "name must be between 1 and 100 characters")
			return
		}

		typ := ledger.AccountType(strings.ToLower(strings.TrimSpace(req.Type)))
		if !typ.Valid() {
			writeError(w, http.StatusBadRequest, "INVALID_ACCOUNT_TYPE", "type must be one of asset, liability, equity, income, expense")
			return
		}
		currency := strings.ToUpper(strings.TrimSpace(req.Currency))
		if !money.IsSupportedCurrency(currency) {
			writeError(w, http.StatusBadRequest, "UNSUPPORTED_CURRENCY", "currency is not supported")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		acct, err := createAccount(ctx, userID, name, typ, currency)
		if err != nil {
			if errors.Is(err, ledger.ErrDuplicateAccountName) {
				writeError(w, http.StatusConflict, "ACCOUNT_NAME_EXISTS", "an account with this name already exist")
				return
			}
			writeError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(toAccountResponse(acct))
	}
}

func handleListAccounts(listAccounts func(context.Context, uuid.UUID) ([]*ledger.Account, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "MISSING_TOKEN", "authentication required")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		accounts, err := listAccounts(ctx, userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
			return
		}

		resp := make([]accountResponse, 0, len(accounts))
		for _, a := range accounts {
			resp = append(resp, toAccountResponse(a))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
