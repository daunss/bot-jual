package repo

import (
	"context"
	"encoding/json"
	"fmt"
)

// InsertOrder stores a new order record.
func (r *PostgresRepository) InsertOrder(ctx context.Context, order Order) (*Order, error) {
	meta, err := toJSON(order.Metadata)
	if err != nil {
		return nil, err
	}
	metaParam := jsonParam(meta)

	const q = `
INSERT INTO orders (user_id, order_ref, product_code, amount, fee, status, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, user_id, order_ref, product_code, amount, fee, status, metadata, created_at, updated_at;
`
	row := r.pool.QueryRow(ctx, q,
		order.UserID,
		order.OrderRef,
		order.ProductCode,
		order.Amount,
		order.Fee,
		order.Status,
		metaParam,
	)

	var inserted Order
	var metaJSON []byte
	if err := row.Scan(&inserted.ID, &inserted.UserID, &inserted.OrderRef, &inserted.ProductCode, &inserted.Amount, &inserted.Fee, &inserted.Status, &metaJSON, &inserted.CreatedAt, &inserted.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert order: %w", err)
	}
	inserted.Metadata = fromJSON(metaJSON)
	return &inserted, nil
}

// UpdateOrderStatus updates order metadata/status.
func (r *PostgresRepository) UpdateOrderStatus(ctx context.Context, orderRef, status string, metadata map[string]any) error {
	meta, err := toJSON(metadata)
	if err != nil {
		return err
	}
	metaParam := jsonParam(meta)
	const q = `
UPDATE orders
SET status = $2,
    metadata = COALESCE($3, metadata),
    updated_at = NOW()
WHERE order_ref = $1;
`
	_, err = r.pool.Exec(ctx, q, orderRef, status, metaParam)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}
	return nil
}

// GetOrderByRef retrieves an order by reference.
func (r *PostgresRepository) GetOrderByRef(ctx context.Context, ref string) (*Order, error) {
	const q = `
SELECT id, user_id, order_ref, product_code, amount, fee, status, metadata, created_at, updated_at
FROM orders
WHERE order_ref = $1
LIMIT 1;
`
	row := r.pool.QueryRow(ctx, q, ref)
	var order Order
	var metaJSON []byte
	if err := row.Scan(&order.ID, &order.UserID, &order.OrderRef, &order.ProductCode, &order.Amount, &order.Fee, &order.Status, &metaJSON, &order.CreatedAt, &order.UpdatedAt); err != nil {
		return nil, fmt.Errorf("get order by ref: %w", err)
	}
	order.Metadata = fromJSON(metaJSON)
	return &order, nil
}

// InsertDeposit stores a new deposit record.
func (r *PostgresRepository) InsertDeposit(ctx context.Context, dep Deposit) (*Deposit, error) {
	meta, err := toJSON(dep.Metadata)
	if err != nil {
		return nil, err
	}
	metaLog := ""
	if meta != nil {
		metaLog = string(meta)
	}
	metaParam := jsonParam(meta)
	r.logger.Debug("insert deposit payload", "deposit_ref", dep.DepositRef, "metadata", metaLog)
	const q = `
INSERT INTO deposits (user_id, deposit_ref, method, amount, status, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, user_id, deposit_ref, method, amount, status, metadata, created_at, updated_at;
`
	row := r.pool.QueryRow(ctx, q,
		dep.UserID,
		dep.DepositRef,
		dep.Method,
		dep.Amount,
		dep.Status,
		metaParam,
	)

	var inserted Deposit
	var metaJSON []byte
	if err := row.Scan(&inserted.ID, &inserted.UserID, &inserted.DepositRef, &inserted.Method, &inserted.Amount, &inserted.Status, &metaJSON, &inserted.CreatedAt, &inserted.UpdatedAt); err != nil {
		return nil, fmt.Errorf("insert deposit: %w", err)
	}
	inserted.Metadata = fromJSON(metaJSON)
	return &inserted, nil
}

// UpdateDepositStatus updates deposit meta/status.
func (r *PostgresRepository) UpdateDepositStatus(ctx context.Context, ref, status string, metadata map[string]any) error {
	meta, err := toJSON(metadata)
	if err != nil {
		return err
	}
	metaParam := jsonParam(meta)
	const q = `
UPDATE deposits
SET status = $2,
    metadata = COALESCE($3, metadata),
    updated_at = NOW()
WHERE deposit_ref = $1;
`
	_, err = r.pool.Exec(ctx, q, ref, status, metaParam)
	if err != nil {
		return fmt.Errorf("update deposit status: %w", err)
	}
	return nil
}

// GetDepositByRef retrieves deposit by reference.
func (r *PostgresRepository) GetDepositByRef(ctx context.Context, ref string) (*Deposit, error) {
	const q = `
SELECT id, user_id, deposit_ref, method, amount, status, metadata, created_at, updated_at
FROM deposits
WHERE deposit_ref = $1
LIMIT 1;
`
	row := r.pool.QueryRow(ctx, q, ref)
	var dep Deposit
	var metaJSON []byte
	if err := row.Scan(&dep.ID, &dep.UserID, &dep.DepositRef, &dep.Method, &dep.Amount, &dep.Status, &metaJSON, &dep.CreatedAt, &dep.UpdatedAt); err != nil {
		return nil, fmt.Errorf("get deposit by ref: %w", err)
	}
	dep.Metadata = fromJSON(metaJSON)
	return &dep, nil
}

// ListOrdersAwaitingDeposit returns orders waiting for the specified deposit.
func (r *PostgresRepository) ListOrdersAwaitingDeposit(ctx context.Context, depositRef string) ([]Order, error) {
	const q = `
SELECT id, user_id, order_ref, product_code, amount, fee, status, metadata, created_at, updated_at
FROM orders
WHERE metadata ->> 'deposit_ref' = $1
  AND status = 'awaiting_payment'
ORDER BY created_at ASC;
`
	rows, err := r.pool.Query(ctx, q, depositRef)
	if err != nil {
		return nil, fmt.Errorf("list orders awaiting deposit: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		var metaJSON []byte
		if err := rows.Scan(&order.ID, &order.UserID, &order.OrderRef, &order.ProductCode, &order.Amount, &order.Fee, &order.Status, &metaJSON, &order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan order awaiting deposit: %w", err)
		}
		order.Metadata = fromJSON(metaJSON)
		orders = append(orders, order)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orders awaiting deposit: %w", err)
	}
	return orders, nil
}

func toJSON(val map[string]any) ([]byte, error) {
	if val == nil {
		return nil, nil
	}
	data, err := json.Marshal(val)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	return data, nil
}

func fromJSON(data []byte) map[string]any {
	if len(data) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]any{"_raw": string(data)}
	}
	return m
}

func jsonParam(data []byte) any {
	if data == nil {
		return nil
	}
	return string(data)
}
