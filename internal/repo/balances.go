package repo

import (
	"context"
	"fmt"
	"time"
)

// UserBalance represents computed balance fields for a user (per JID).
type UserBalance struct {
	UserID             string
	WAID               string
	WAJID              *string
	DepositedConfirmed int64
	SpentConfirmed     int64
	SaldoConfirmed     int64
	TotalDeposited     int64
	TotalSpent         int64
	DepositedPending   int64
	SpentPending       int64
	UpdatedAt          *time.Time
}

// GetUserBalance loads the latest computed balance from public.user_balances_table.
func (r *PostgresRepository) GetUserBalance(ctx context.Context, userID string) (*UserBalance, error) {
	const q = `
SELECT user_id, wa_id, wa_jid,
       deposited_confirmed, spent_confirmed, saldo_confirmed,
       total_deposited, total_spent, deposited_pending, spent_pending,
       updated_at
FROM user_balances_table
WHERE user_id = $1
LIMIT 1;
`
	row := r.pool.QueryRow(ctx, q, userID)
	var ub UserBalance
	if err := row.Scan(
		&ub.UserID,
		&ub.WAID,
		&ub.WAJID,
		&ub.DepositedConfirmed,
		&ub.SpentConfirmed,
		&ub.SaldoConfirmed,
		&ub.TotalDeposited,
		&ub.TotalSpent,
		&ub.DepositedPending,
		&ub.SpentPending,
		&ub.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("get user balance: %w", err)
	}
	return &ub, nil
}
