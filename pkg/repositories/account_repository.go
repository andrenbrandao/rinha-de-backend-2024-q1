package repositories

import (
	"context"
	"errors"

	"github.com/andrenbrandao/rinha-de-backend-2024-q1/pkg/domain"
	"github.com/jackc/pgx/v5"
)

func GetAccount(accountId string, tx pgx.Tx, ctx context.Context) (domain.Account, error) {
	var currAccount domain.Account
	row := tx.QueryRow(ctx, "SELECT balance, balance_limit FROM accounts WHERE id = $1 FOR UPDATE;", accountId)
	err := row.Scan(&currAccount.Balance, &currAccount.BalanceLimit)

	if errors.Is(err, pgx.ErrNoRows) {
		return currAccount, domain.ErrNotFound
	}
	if err != nil {
		return currAccount, err
	}

	return currAccount, nil
}
