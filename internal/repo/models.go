package repo

import "time"

// User represents the users table row.
type User struct {
	ID                 string
	WAID               string
	WAJID              *string
	DisplayName        *string
	PhoneNumber        *string
	LanguagePreference string
	Timezone           string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// UserProfile carries data used to upsert a user.
type UserProfile struct {
	WAID               string
	WAJID              *string
	DisplayName        *string
	PhoneNumber        *string
	LanguagePreference *string
	Timezone           *string
}

// MessageRecord is used to persist conversation logs.
type MessageRecord struct {
	UserID     string
	Direction  string
	Type       string
	Content    *string
	MediaURL   *string
	RawPayload any
	CreatedAt  time.Time
}

// APIKey represents a record in api_keys table.
type APIKey struct {
	ID            string
	Provider      string
	Value         string
	Priority      int
	CooldownUntil *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Order represents a row in orders table.
type Order struct {
	ID          string
	UserID      string
	OrderRef    string
	ProductCode string
	Amount      int64
	Fee         int64
	Status      string
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Deposit represents a row in deposits table.
type Deposit struct {
	ID         string
	UserID     string
	DepositRef string
	Method     string
	Amount     int64
	Status     string
	Metadata   map[string]any
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
