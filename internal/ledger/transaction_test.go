package ledger

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
)

var (
	userID    = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	checking  = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	groceries = uuid.MustParse("33333333-3333-3333-3333-333333333333")
	savings   = uuid.MustParse("44444444-4444-4444-4444-444444444444")
)

func TestValidateBalancedExpense(t *testing.T) {
	tx := &Transaction{
		UserID:      userID,
		Description: "groceries",
		OccuredAt:   time.Now(),
		Postings: []Posting{
			{AccountID: groceries, AmountMinor: 5000, Currency: "EUR"},
			{AccountID: checking, AmountMinor: -5000, Currency: "EUR"},
		},
	}
	if err := tx.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateBalancedTransfer(t *testing.T) {
	tx := &Transaction{
		UserID:      userID,
		Description: "monthly savings transfer",
		OccuredAt:   time.Now(),
		Postings: []Posting{
			{AccountID: savings, AmountMinor: 20000, Currency: "EUR"},
			{AccountID: checking, AmountMinor: -20000, Currency: "EUR"},
		},
	}
	if err := tx.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateUnbalanced(t *testing.T) {
	tx := &Transaction{
		Postings: []Posting{
			{AccountID: groceries, AmountMinor: 5000, Currency: "EUR"},
			{AccountID: checking, AmountMinor: -4000, Currency: "EUR"},
		},
	}
	err := tx.Validate()
	if !errors.Is(err, ErrUnbalanced) {
		t.Fatalf("Validate = %v, want ErrUnbalanced", err)
	}
}

func TestValidateEmptyPostings(t *testing.T) {
	tx := &Transaction{Postings: nil}
	if err := tx.Validate(); !errors.Is(err, ErrEmptyTransaction) {
		t.Fatalf("Validate = %v, want ErrEmptyTransaction", err)
	}
}

func TestValidateSinglePosting(t *testing.T) {
	tx := &Transaction{
		Postings: []Posting{
			{AccountID: groceries, AmountMinor: 5000, Currency: "EUR"},
		},
	}
	if err := tx.Validate(); !errors.Is(err, ErrEmptyTransaction) {
		t.Fatalf("Validate = %v, want ErrEmptyTransaction", err)
	}
}

func TestValidateZeroAmount(t *testing.T) {
	tx := &Transaction{
		Postings: []Posting{
			{AccountID: groceries, AmountMinor: 0, Currency: "EUR"},
			{AccountID: checking, AmountMinor: 0, Currency: "EUR"},
		},
	}
	if err := tx.Validate(); !errors.Is(err, ErrZeroAmount) {
		t.Fatalf("Validate = %v, want ErrZeroAmount", err)
	}
}

func TestValidateMixedCurrency(t *testing.T) {
	tx := &Transaction{
		Postings: []Posting{
			{AccountID: groceries, AmountMinor: 5000, Currency: "EUR"},
			{AccountID: checking, AmountMinor: -5000, Currency: "USD"},
		},
	}
	if err := tx.Validate(); !errors.Is(err, ErrMixedCurrency) {
		t.Fatalf("Validate = %v, want ErrMixedCurrency", err)
	}
}

func TestValidateMultiLegBalanced(t *testing.T) {
	// A three-posting transaction: salary €3000 split €2500 to savings,
	// €500 to checking. Legal and balanced.
	tx := &Transaction{
		Postings: []Posting{
			{AccountID: checking, AmountMinor: 50000, Currency: "EUR"},
			{AccountID: savings, AmountMinor: 250000, Currency: "EUR"},
			{AccountID: groceries, AmountMinor: -300000, Currency: "EUR"}, // credit income (using groceries UUID arbitrarily)
		},
	}
	if err := tx.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateNegativeBalanced(t *testing.T) {
	// A reversal transaction: sign is flipped on both postings. Should still
	// validate (sum = 0).
	tx := &Transaction{
		Postings: []Posting{
			{AccountID: groceries, AmountMinor: -5000, Currency: "EUR"},
			{AccountID: checking, AmountMinor: 5000, Currency: "EUR"},
		},
	}
	if err := tx.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateLargeAmount(t *testing.T) {
	t.Run("half_max_int64_balanced", func(t *testing.T) {
		big := int64(math.MaxInt64 / 2)
		tx := &Transaction{
			UserID:      userID,
			Description: "large safe amounts",
			OccuredAt:   time.Now(),
			Postings: []Posting{
				{AccountID: checking, AmountMinor: big, Currency: "EUR"},
				{AccountID: savings, AmountMinor: -big, Currency: "EUR"},
			},
		}
		if err := tx.Validate(); err != nil {
			t.Fatalf("Validate with hald MaxInt64 failed: %v", err)
		}
	})
	t.Run("overflow_wrapping_results_in_unbalanced", func(t *testing.T) {
		tx := &Transaction{
			UserID:      userID,
			Description: "overflow amount case",
			OccuredAt:   time.Now(),
			Postings: []Posting{
				{AccountID: checking, AmountMinor: math.MaxInt64, Currency: "EUR"},
				{AccountID: savings, AmountMinor: math.MaxInt64, Currency: "EUR"}, // sum overflows
				{AccountID: groceries, AmountMinor: -1, Currency: "EUR"},
			},
		}
		err := tx.Validate()
		// Signed integer overflow wraps around to a non-zero value (-3),
		// which the balance check catches as ErrUnbalanced.
		if !errors.Is(err, ErrUnbalanced) {
			t.Fatalf("Validate overflow case = %v, want ErrUnbalanced", err)
		}
	})
}

func TestNewTransactionRejectsUnbalanced(t *testing.T) {
	_, err := NewTransaction(userID, "bad", time.Now(), []Posting{
		{AccountID: groceries, AmountMinor: 5000, Currency: "EUR"},
		{AccountID: checking, AmountMinor: -4000, Currency: "EUR"},
	})
	if !errors.Is(err, ErrUnbalanced) {
		t.Fatalf("NewTransaction = %v, want ErrUnbalanced", err)
	}
}

func TestAccountTypeValid(t *testing.T) {
	cases := map[AccountType]bool{
		AccountTypeAsset:     true,
		AccountTypeLiability: true,
		AccountTypeEquity:    true,
		AccountTypeIncome:    true,
		AccountTypeExpense:   true,
		"":                   false,
		"AccountTypeAsset":   false,
		"ASSET":              false, // case-sensitive
	}
	for typ, want := range cases {
		if got := typ.Valid(); got != want {
			t.Fatalf("AccountType(%q).Valid() = %v, want %v", typ, got, want)
		}
	}
}
