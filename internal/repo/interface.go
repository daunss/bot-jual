package repo

import (
	"context"
	"io/fs"
	"time"
)

// Repository defines the interface for data persistence.
type Repository interface {
	// Lifecycle
	Close()
	Ping(ctx context.Context) error
	RunMigrations(ctx context.Context, filesystem fs.FS) error

	// Users
	UpsertUserByWA(ctx context.Context, profile UserProfile) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)

	// Messages
	InsertMessage(ctx context.Context, msg MessageRecord) error
	ListRecentMessages(ctx context.Context, userID string, limit int) ([]MessageRecord, error)

	// API Keys
	SyncGeminiKeys(ctx context.Context, keys []string) error
	ListActiveGeminiKeys(ctx context.Context) ([]APIKey, error)
	ClearCooldown(ctx context.Context, id string) error
	SetCooldownUntil(ctx context.Context, id string, until time.Time) error
	UpdateAPIKeyCooldown(ctx context.Context, id string, until time.Time) error

	// Balances
	GetUserBalance(ctx context.Context, userID string) (*UserBalance, error)

	// Orders
	InsertOrder(ctx context.Context, order Order) (*Order, error)
	GetOrderByRef(ctx context.Context, ref string) (*Order, error)
	UpdateOrderStatus(ctx context.Context, orderRef, status string, metadata map[string]any) error
	ListOrdersAwaitingDeposit(ctx context.Context, depositRef string) ([]Order, error)

	// Deposits
	InsertDeposit(ctx context.Context, dep Deposit) (*Deposit, error)
	GetDepositByRef(ctx context.Context, ref string) (*Deposit, error)
	UpdateDepositStatus(ctx context.Context, ref, status string, metadata map[string]any) error
}
