package ledger

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrUnbalanced = errors.New("ledger: transaction does not balance")
var ErrEmptyTransaction = errors.New("ledger: transaction has no postings")
var ErrMixedCurrency = errors.New("ledger: all postings in a transaction must share on currency")
var ErrZeroAmount = errors.New("ledger: posting amount must be non-zero")

type Posting struct {
	AccountID   uuid.UUID
	AmountMinor int64
	Currency    string
}

type Transaction struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Description string
	OccuredAt   time.Time
	Postings    []Posting
	CreatedAt   time.Time
}

func (t *Transaction) Validate() error {
	if len(t.Postings) < 2 {
		return ErrEmptyTransaction
	}

	var sum int64
	var currency string

	for i, p := range t.Postings {
		if p.AmountMinor == 0 {
			return fmt.Errorf("%w: posting %d", ErrZeroAmount, i)
		}
		if i == 0 {
			currency = p.Currency
		} else if p.Currency != currency {
			return fmt.Errorf("%w: %q vs %q", ErrMixedCurrency, currency, p.Currency)
		}
		sum += p.AmountMinor
	}
	if sum != 0 {
		return fmt.Errorf("%w: sum is %d", ErrUnbalanced, sum)
	}
	return nil
}

func NewTransaction(userID uuid.UUID, description string, occuredAt time.Time, postings []Posting) (*Transaction, error) {
	t := &Transaction{
		UserID:      userID,
		Description: description,
		OccuredAt:   occuredAt,
		Postings:    postings,
	}
	if err := t.Validate(); err != nil {
		return nil, err
	}
	return t, nil
}
