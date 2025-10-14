package repo

import (
	"context"
	"fmt"
)

// GetUserByID returns user by internal identifier.
func (r *Repository) GetUserByID(ctx context.Context, id string) (*User, error) {
	const q = `
SELECT id, wa_id, wa_jid, display_name, phone_number, language_preference, timezone, created_at, updated_at
FROM users
WHERE id = $1
LIMIT 1;
`
	row := r.pool.QueryRow(ctx, q, id)
	var user User
	if err := row.Scan(&user.ID, &user.WAID, &user.WAJID, &user.DisplayName, &user.PhoneNumber, &user.LanguagePreference, &user.Timezone, &user.CreatedAt, &user.UpdatedAt); err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &user, nil
}
