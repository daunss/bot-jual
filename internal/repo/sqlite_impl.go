package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// -- Users --

func (r *SQLiteRepository) UpsertUserByWA(ctx context.Context, profile UserProfile) (*User, error) {
	// SQLite supports ON CONFLICT from 3.24+
	// COALESCE works same.
	// $n -> ?
	// NOW() -> CURRENT_TIMESTAMP
	const q = `
INSERT INTO users (id, wa_id, wa_jid, display_name, phone_number, language_preference, timezone, updated_at)
VALUES (?, ?, ?, ?, ?, COALESCE(?, 'id-ID'), COALESCE(?, 'Asia/Jakarta'), CURRENT_TIMESTAMP)
ON CONFLICT (wa_id) DO UPDATE SET
    wa_jid = excluded.wa_jid,
    display_name = COALESCE(excluded.display_name, users.display_name),
    phone_number = COALESCE(excluded.phone_number, users.phone_number),
    language_preference = COALESCE(excluded.language_preference, users.language_preference),
    timezone = COALESCE(excluded.timezone, users.timezone),
    updated_at = CURRENT_TIMESTAMP
RETURNING id, wa_id, wa_jid, display_name, phone_number, language_preference, timezone, created_at, updated_at;
`
	// Need to generate UUID for ID if it's new?
	// The migration says ID is TEXT PRIMARY KEY.
	// Postgres handles gen_random_uuid().
	// SQLite does not auto-generate UUIDs unless using an extension or hex(randomblob(16)).
	// IMPORTANT: usage of hex(randomblob) creates a long hex string, which is fine, but formatting might differ from standard UUID.
	// Better to generate UUID in Go.

	// BUT, Upsert logic: if it exists, we don't need valid ID.
	// Only for INSERT.
	// Helper function for UUID? I can use "github.com/google/uuid".

	id := randomUUID()

	row := r.db.QueryRowContext(ctx, q,
		id,
		profile.WAID,
		profile.WAJID,
		profile.DisplayName,
		profile.PhoneNumber,
		profile.LanguagePreference,
		profile.Timezone,
	)

	var u User
	if err := row.Scan(&u.ID, &u.WAID, &u.WAJID, &u.DisplayName, &u.PhoneNumber, &u.LanguagePreference, &u.Timezone, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return &u, nil
}

func (r *SQLiteRepository) GetUserByID(ctx context.Context, id string) (*User, error) {
	const q = `
SELECT id, wa_id, wa_jid, display_name, phone_number, language_preference, timezone, created_at, updated_at
FROM users
WHERE id = ?
LIMIT 1;
`
	row := r.db.QueryRowContext(ctx, q, id)
	var user User
	if err := row.Scan(&user.ID, &user.WAID, &user.WAJID, &user.DisplayName, &user.PhoneNumber, &user.LanguagePreference, &user.Timezone, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &user, nil
}

// -- Messages --

func (r *SQLiteRepository) InsertMessage(ctx context.Context, msg MessageRecord) error {
	id := randomUUID()
	const q = `
INSERT INTO messages (id, user_id, direction, message_type, content, media_url, raw_payload)
VALUES (?, ?, ?, ?, ?, ?, ?);
`
	_, err := r.db.ExecContext(ctx, q,
		id,
		msg.UserID,
		msg.Direction,
		msg.Type,
		msg.Content,
		msg.MediaURL,
		msg.RawPayload,
	)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) ListRecentMessages(ctx context.Context, userID string, limit int) ([]MessageRecord, error) {
	if limit <= 0 {
		limit = 10
	}
	const q = `
SELECT direction, message_type, content, created_at
FROM messages
WHERE user_id = ?
ORDER BY created_at DESC
LIMIT ?;
`
	rows, err := r.db.QueryContext(ctx, q, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent messages: %w", err)
	}
	defer rows.Close()

	var records []MessageRecord
	for rows.Next() {
		var msg MessageRecord
		if err := rows.Scan(&msg.Direction, &msg.Type, &msg.Content, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan recent message: %w", err)
		}
		msg.UserID = userID
		records = append(records, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent messages: %w", err)
	}
	return records, nil
}

// -- API Keys --

func (r *SQLiteRepository) SyncGeminiKeys(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return fmt.Errorf("no gemini keys provided")
	}

	for idx, key := range keys {
		if err := r.upsertAPIKey(ctx, providerGemini, key, idx); err != nil {
			return err
		}
	}
	return nil
}

func (r *SQLiteRepository) upsertAPIKey(ctx context.Context, provider, value string, priority int) error {
	id := randomUUID()
	const q = `
INSERT INTO api_keys (id, provider, value, priority, cooldown_until)
VALUES (?, ?, ?, ?, NULL)
ON CONFLICT (provider, value) DO UPDATE
SET priority = excluded.priority,
    cooldown_until = NULL,
    updated_at = CURRENT_TIMESTAMP;`
	_, err := r.db.ExecContext(ctx, q, id, provider, value, priority)
	if err != nil {
		return fmt.Errorf("upsert api key: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) ListActiveGeminiKeys(ctx context.Context) ([]APIKey, error) {
	const q = `
SELECT id, provider, value, priority, cooldown_until, created_at, updated_at
FROM api_keys
WHERE provider = ?
ORDER BY priority ASC;
`
	rows, err := r.db.QueryContext(ctx, q, providerGemini)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var res []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.Provider, &k.Value, &k.Priority, &k.CooldownUntil, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		res = append(res, k)
	}
	return res, nil
}

func (r *SQLiteRepository) ClearCooldown(ctx context.Context, id string) error {
	const q = `UPDATE api_keys SET cooldown_until = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	ct, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("clear cooldown: %w", err)
	}
	if n, _ := ct.RowsAffected(); n == 0 {
		return fmt.Errorf("api key not found: %s", id)
	}
	return nil
}

func (r *SQLiteRepository) SetCooldownUntil(ctx context.Context, id string, until time.Time) error {
	const q = `UPDATE api_keys SET cooldown_until = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	ct, err := r.db.ExecContext(ctx, q, until, id)
	if err != nil {
		return fmt.Errorf("set cooldown: %w", err)
	}
	if n, _ := ct.RowsAffected(); n == 0 {
		return fmt.Errorf("api key not found: %s", id)
	}
	return nil
}

func (r *SQLiteRepository) UpdateAPIKeyCooldown(ctx context.Context, id string, until time.Time) error {
	return r.SetCooldownUntil(ctx, id, until)
}

// -- Balances --

func (r *SQLiteRepository) GetUserBalance(ctx context.Context, userID string) (*UserBalance, error) {
	const userQ = `
SELECT wa_id, wa_jid, updated_at
FROM users
WHERE id = ?
LIMIT 1;
`
	var waid string
	var wajid sql.NullString
	var updatedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, userQ, userID).Scan(&waid, &wajid, &updatedAt); err != nil {
		return nil, fmt.Errorf("get user balance user lookup: %w", err)
	}

	ub := &UserBalance{UserID: userID, WAID: waid}
	if wajid.Valid {
		ub.WAJID = &wajid.String
	}
	if updatedAt.Valid {
		ub.UpdatedAt = &updatedAt.Time
	}

	const depQ = `
SELECT
	COALESCE(SUM(CASE WHEN status = 'success' THEN amount ELSE 0 END), 0) AS deposited_confirmed,
	COALESCE(SUM(CASE WHEN status IN ('pending', 'processing') THEN amount ELSE 0 END), 0) AS deposited_pending,
	COALESCE(SUM(amount), 0) AS total_deposited
FROM deposits
WHERE user_id = ?;
`
	var depConfirmed, depPending, depTotal int64
	if err := r.db.QueryRowContext(ctx, depQ, userID).Scan(&depConfirmed, &depPending, &depTotal); err != nil {
		return nil, fmt.Errorf("get user balance deposits: %w", err)
	}

	const ordQ = `
SELECT
	COALESCE(SUM(CASE WHEN status = 'success' THEN amount ELSE 0 END), 0) AS spent_confirmed,
	COALESCE(SUM(CASE WHEN status IN ('pending', 'processing', 'awaiting_payment') THEN amount ELSE 0 END), 0) AS spent_pending,
	COALESCE(SUM(amount), 0) AS total_spent
FROM orders
WHERE user_id = ?;
`
	var spentConfirmed, spentPending, spentTotal int64
	if err := r.db.QueryRowContext(ctx, ordQ, userID).Scan(&spentConfirmed, &spentPending, &spentTotal); err != nil {
		return nil, fmt.Errorf("get user balance orders: %w", err)
	}

	ub.DepositedConfirmed = depConfirmed
	ub.DepositedPending = depPending
	ub.TotalDeposited = depTotal
	ub.SpentConfirmed = spentConfirmed
	ub.SpentPending = spentPending
	ub.TotalSpent = spentTotal
	ub.SaldoConfirmed = depConfirmed - spentConfirmed

	return ub, nil
}

// -- Orders --

func (r *SQLiteRepository) InsertOrder(ctx context.Context, order Order) (*Order, error) {
	id := randomUUID()
	meta, err := toJSON(order.Metadata)
	if err != nil {
		return nil, err
	}
	metaParam := jsonParam(meta)

	const q = `
INSERT INTO orders (id, user_id, order_ref, product_code, amount, fee, status, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, user_id, order_ref, product_code, amount, fee, status, metadata, created_at, updated_at;
`
	row := r.db.QueryRowContext(ctx, q,
		id,
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

func (r *SQLiteRepository) UpdateOrderStatus(ctx context.Context, orderRef, status string, metadata map[string]any) error {
	meta, err := toJSON(metadata)
	if err != nil {
		return err
	}
	metaParam := jsonParam(meta)
	const q = `
UPDATE orders
SET status = ?,
    metadata = COALESCE(?, metadata),
    updated_at = CURRENT_TIMESTAMP
WHERE order_ref = ?;
`
	_, err = r.db.ExecContext(ctx, q, status, metaParam, orderRef)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) GetOrderByRef(ctx context.Context, ref string) (*Order, error) {
	const q = `
SELECT id, user_id, order_ref, product_code, amount, fee, status, metadata, created_at, updated_at
FROM orders
WHERE order_ref = ?
LIMIT 1;
`
	row := r.db.QueryRowContext(ctx, q, ref)
	var order Order
	var metaJSON []byte
	if err := row.Scan(&order.ID, &order.UserID, &order.OrderRef, &order.ProductCode, &order.Amount, &order.Fee, &order.Status, &metaJSON, &order.CreatedAt, &order.UpdatedAt); err != nil {
		return nil, fmt.Errorf("get order by ref: %w", err)
	}
	order.Metadata = fromJSON(metaJSON)
	return &order, nil
}

func (r *SQLiteRepository) ListOrdersAwaitingDeposit(ctx context.Context, depositRef string) ([]Order, error) {
	// SQLite JSON support: json_extract(metadata, '$.deposit_ref')
	const q = `
SELECT id, user_id, order_ref, product_code, amount, fee, status, metadata, created_at, updated_at
FROM orders
WHERE json_extract(metadata, '$.deposit_ref') = ?
  AND status = 'awaiting_payment'
ORDER BY created_at ASC;
`
	rows, err := r.db.QueryContext(ctx, q, depositRef)
	if err != nil {
		return nil, fmt.Errorf("list orders awaiting: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var order Order
		var metaJSON []byte
		if err := rows.Scan(&order.ID, &order.UserID, &order.OrderRef, &order.ProductCode, &order.Amount, &order.Fee, &order.Status, &metaJSON, &order.CreatedAt, &order.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		order.Metadata = fromJSON(metaJSON)
		orders = append(orders, order)
	}
	return orders, nil
}

// -- Deposits --

func (r *SQLiteRepository) InsertDeposit(ctx context.Context, dep Deposit) (*Deposit, error) {
	id := randomUUID()
	meta, err := toJSON(dep.Metadata)
	if err != nil {
		return nil, err
	}
	metaParam := jsonParam(meta)

	const q = `
INSERT INTO deposits (id, user_id, deposit_ref, method, amount, status, metadata)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, user_id, deposit_ref, method, amount, status, metadata, created_at, updated_at;
`
	row := r.db.QueryRowContext(ctx, q,
		id,
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

func (r *SQLiteRepository) UpdateDepositStatus(ctx context.Context, ref, status string, metadata map[string]any) error {
	meta, err := toJSON(metadata)
	if err != nil {
		return err
	}
	metaParam := jsonParam(meta)
	const q = `
UPDATE deposits
SET status = ?,
    metadata = COALESCE(?, metadata),
    updated_at = CURRENT_TIMESTAMP
WHERE deposit_ref = ?;
`
	_, err = r.db.ExecContext(ctx, q, status, metaParam, ref)
	if err != nil {
		return fmt.Errorf("update deposit status: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) GetDepositByRef(ctx context.Context, ref string) (*Deposit, error) {
	const q = `
SELECT id, user_id, deposit_ref, method, amount, status, metadata, created_at, updated_at
FROM deposits
WHERE deposit_ref = ?
LIMIT 1;
`
	row := r.db.QueryRowContext(ctx, q, ref)
	var dep Deposit
	var metaJSON []byte
	if err := row.Scan(&dep.ID, &dep.UserID, &dep.DepositRef, &dep.Method, &dep.Amount, &dep.Status, &metaJSON, &dep.CreatedAt, &dep.UpdatedAt); err != nil {
		return nil, fmt.Errorf("get deposit by ref: %w", err)
	}
	dep.Metadata = fromJSON(metaJSON)
	return &dep, nil
}

// -- Helpers --

func randomUUID() string {
	// Basic UUID v4 generation to avoid external dep complications if possible,
	// but google/uuid is already checking go.mod
	// I should check imports. I'll add the import.
	// For now, I'll use a placeholder if import not added, but better add "github.com/google/uuid".
	return uuidV4()
}

// Minimal UUID v4 implementation
func uuidV4() string {
	return uuid.NewString()
}
