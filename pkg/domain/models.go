package domain

import "github.com/jackc/pgx/v5/pgtype"

type Account struct {
	Id           int                `json:"id"`
	Name         string             `json:"name"`
	Balance      int                `json:"balance"`
	BalanceLimit int                `json:"balance_limit"`
	CreatedAt    pgtype.Timestamptz `json:"created_at"`
}

type Transaction struct {
	Id          int                `json:"id"`
	AccountId   int                `json:"account_id"`
	Amount      int                `json:"amount"`
	Type        string             `json:"type"`
	Description string             `json:"description"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
}
