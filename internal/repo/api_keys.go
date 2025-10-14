package repo

import (
	"context"
	"fmt"
	"time"
)

const providerGemini = "gemini"

// SyncGeminiKeys ensures provided keys exist in database with matching priority.
func (r *Repository) SyncGeminiKeys(ctx context.Context, keys []string) error {
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

func (r *Repository) upsertAPIKey(ctx context.Context, provider, value string, priority int) error {
	const q = `
INSERT INTO api_keys (provider, value, priority)
VALUES ($1, $2, $3)
ON CONFLICT (provider, value) DO UPDATE
SET priority = EXCLUDED.priority,
    updated_at = NOW();`
	_, err := r.pool.Exec(ctx, q, provider, value, priority)
	if err != nil {
		return fmt.Errorf("upsert api key: %w", err)
	}
	return nil
}

// ListActiveGeminiKeys returns Gemini API keys ordered by priority.
func (r *Repository) ListActiveGeminiKeys(ctx context.Context) ([]APIKey, error) {
	const q = `
SELECT id, provider, value, priority, cooldown_until, created_at, updated_at
FROM api_keys
WHERE provider = $1
ORDER BY priority ASC;
`
	rows, err := r.pool.Query(ctx, q, providerGemini)
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list api keys rows: %w", err)
	}
	return res, nil
}

// ClearCooldown resets cooldown for a key.
func (r *Repository) ClearCooldown(ctx context.Context, id string) error {
	const q = `UPDATE api_keys SET cooldown_until = NULL, updated_at = NOW() WHERE id = $1`
	ct, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("clear cooldown: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("api key not found: %s", id)
	}
	return nil
}

// SetCooldownUntil updates cooldown until specific time.
func (r *Repository) SetCooldownUntil(ctx context.Context, id string, until time.Time) error {
	const q = `UPDATE api_keys SET cooldown_until = $2, updated_at = NOW() WHERE id = $1`
	ct, err := r.pool.Exec(ctx, q, id, until)
	if err != nil {
		return fmt.Errorf("set cooldown: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("api key not found: %s", id)
	}
	return nil
}
