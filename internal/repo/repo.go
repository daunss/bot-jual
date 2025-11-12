package repo

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository provides typed access to Supabase (Postgres) resources.
type Repository struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
	schema string
}

// New opens a new connection pool to the database with the desired search_path.
func New(ctx context.Context, databaseURL, schema string, logger *slog.Logger) (*Repository, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	if schema != "" {
		cfg.ConnConfig.RuntimeParams["search_path"] = schema
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	r := &Repository{
		pool:   pool,
		logger: logger.With("component", "repo"),
		schema: schema,
	}

	if err := r.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return r, nil
}

// Close releases the connection pool.
func (r *Repository) Close() {
	if r.pool != nil {
		r.pool.Close()
	}
}

// Ping ensures the database is reachable.
func (r *Repository) Ping(ctx context.Context) error {
	return r.pool.Ping(ctx)
}

// WithTx executes fn within a database transaction.
func (r *Repository) WithTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		return fn(tx)
	})
}

// UpsertUserByWA stores or updates the user profile based on WhatsApp ID.
func (r *Repository) UpsertUserByWA(ctx context.Context, profile UserProfile) (*User, error) {
	const q = `
INSERT INTO users (wa_id, wa_jid, display_name, phone_number, language_preference, timezone, updated_at)
VALUES ($1, $2, $3, $4, COALESCE($5, 'id-ID'), COALESCE($6, 'Asia/Jakarta'), NOW())
ON CONFLICT (wa_id) DO UPDATE SET
    wa_jid = EXCLUDED.wa_jid,
    display_name = COALESCE(EXCLUDED.display_name, users.display_name),
    phone_number = COALESCE(EXCLUDED.phone_number, users.phone_number),
    language_preference = COALESCE(EXCLUDED.language_preference, users.language_preference),
    timezone = COALESCE(EXCLUDED.timezone, users.timezone),
    updated_at = NOW()
RETURNING id, wa_id, wa_jid, display_name, phone_number, language_preference, timezone, created_at, updated_at;
`
	row := r.pool.QueryRow(ctx, q,
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

// InsertMessage stores a message record for auditing purposes.
func (r *Repository) InsertMessage(ctx context.Context, msg MessageRecord) error {
	const q = `
INSERT INTO messages (user_id, direction, message_type, content, media_url, raw_payload)
VALUES ($1, $2, $3, $4, $5, $6);
`
	_, err := r.pool.Exec(ctx, q,
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

// ListRecentMessages returns the latest messages exchanged with the user.
func (r *Repository) ListRecentMessages(ctx context.Context, userID string, limit int) ([]MessageRecord, error) {
	if limit <= 0 {
		limit = 10
	}
	const q = `
SELECT direction, message_type, content, created_at
FROM messages
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2;
`
	rows, err := r.pool.Query(ctx, q, userID, limit)
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

// RunMigrations applies schema migrations on the connected database.
func (r *Repository) RunMigrations(ctx context.Context, filesystem fs.FS) error {
	return ApplyMigrations(ctx, r.pool, filesystem)
}

// ListActiveGeminiKeys retrieves active Gemini API keys from the database.
func (r *Repository) ListActiveGeminiKeys(ctx context.Context) ([]APIKey, error) {
	const q = `
SELECT id, value, provider, cooldown_until
FROM api_keys
WHERE provider = 'gemini' AND status = 'active'
ORDER BY last_used_at ASC NULLS FIRST, created_at ASC;
`
	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list active gemini keys: %w", err)
	}
	defer rows.Close()

	var keys []APIKey
	for rows.Next() {
		var k APIKey
		if err := rows.Scan(&k.ID, &k.Value, &k.Provider, &k.CooldownUntil); err != nil {
			return nil, fmt.Errorf("scan active gemini key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active gemini keys: %w", err)
	}
	return keys, nil
}

// SetCooldownUntil sets the cooldown time for a specific API key.
func (r *Repository) SetCooldownUntil(ctx context.Context, id string, until time.Time) error {
	const q = `UPDATE api_keys SET cooldown_until = $2, updated_at = NOW() WHERE id = $1`
	ct, err := r.pool.Exec(ctx, q, id, until)
	if err != nil {
		return fmt.Errorf("update api key cooldown: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("api key not found: %s", id)
	}
	return nil
}

// InsertOrder creates a new order record.
func (r *Repository) InsertOrder(ctx context.Context, order Order) (*Order, error) {
	const q = `
INSERT INTO orders (user_id, order_ref, product_code, amount, status, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at;
`
	var id string
	var createdAt time.Time
	err := r.pool.QueryRow(ctx, q,
		order.UserID,
		order.OrderRef,
		order.ProductCode,
		order.Amount,
		order.Status,
		order.Metadata,
	).Scan(&id, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("insert order: %w", err)
	}
	order.ID = id
	order.CreatedAt = createdAt
	return &order, nil
}

// UpdateOrderStatus updates the status and metadata of an existing order.
func (r *Repository) UpdateOrderStatus(ctx context.Context, orderRef, status string, metadata map[string]any) error {
	const q = `
UPDATE orders
SET status = $2, metadata = metadata || $3, updated_at = NOW()
WHERE order_ref = $1;
`
	ct, err := r.pool.Exec(ctx, q, orderRef, status, metadata)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("order not found: %s", orderRef)
	}
	return nil
}

// InsertDeposit creates a new deposit record.
func (r *Repository) InsertDeposit(ctx context.Context, deposit Deposit) (*Deposit, error) {
	const q = `
INSERT INTO deposits (user_id, deposit_ref, method, amount, status, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, created_at;
`
	var id string
	var createdAt time.Time
	err := r.pool.QueryRow(ctx, q,
		deposit.UserID,
		deposit.DepositRef,
		deposit.Method,
		deposit.Amount,
		deposit.Status,
		deposit.Metadata,
	).Scan(&id, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("insert deposit: %w", err)
	}
	deposit.ID = id
	deposit.CreatedAt = createdAt
	return &deposit, nil
}

// SyncGeminiKeys ensures the provided keys exist in the database.
func (r *Repository) SyncGeminiKeys(ctx context.Context, keys []string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.WithTx(ctx, func(tx pgx.Tx) error {
		for _, key := range keys {
			if _, err := tx.Exec(ctx, `
INSERT INTO api_keys (value, provider, status)
VALUES ($1, 'gemini', 'active')
ON CONFLICT (value, provider) DO NOTHING;
`, key); err != nil {
				return fmt.Errorf("sync gemini key %q: %w", key[:5], err)
			}
		}
		return nil
	})
}
