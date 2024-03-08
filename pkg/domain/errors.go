package domain

import "errors"

var (
	ErrInsufficientFunds          = errors.New("account does not have available limit for this debit amount")
	ErrUnknownBankTransactionType = errors.New("unknown bank transaction type")
	ErrNotFound                   = errors.New("account not found")
)
