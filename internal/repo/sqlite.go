package repo

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"

	_ "modernc.org/sqlite"
)

// SQLiteRepository provides access to a local SQLite database.
type SQLiteRepository struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewSQLite opens a new connection to the SQLite database.
func NewSQLite(ctx context.Context, databasePath string, logger *slog.Logger) (*SQLiteRepository, error) {
	path := strings.TrimSpace(databasePath)
	if path == "" {
		return nil, fmt.Errorf("sqlite database path is empty")
	}
	// Busy timeout and WAL mode are recommended for SQLite concurrency.
	dsn := path
	if !strings.HasPrefix(dsn, "file:") {
		dsn = "file:" + dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	dsn = fmt.Sprintf("%s%s_pragma=busy_timeout=10000&_pragma=journal_mode=WAL&_pragma=foreign_keys=ON", dsn, sep)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	r := &SQLiteRepository{
		db:     db,
		logger: logger.With("component", "repo_sqlite"),
	}

	return r, nil
}

// Close releases the database connection.
func (r *SQLiteRepository) Close() {
	if r.db != nil {
		r.db.Close()
	}
}

// Ping ensures the database is reachable.
func (r *SQLiteRepository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// RunMigrations applies schema migrations on the connected database.
func (r *SQLiteRepository) RunMigrations(ctx context.Context, filesystem fs.FS) error {
	// We need a separate migration runner for database/sql
	// vs pgxpool. The existing ApplyMigrations likely uses pgxpool.
	// I will need to implement a simple migration runner for sql.DB here or adapt the existing one.
	// For now, let's assume we can implement a simple one here.

	// Actually, let's just read the file and exec it since we only have one migration for now.
	// Or reuse the logic if possible.

	// Simplest: Read 001_init.sql from fs and Exec.
	sqlContent, err := fs.ReadFile(filesystem, "sqlite/001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, string(sqlContent)); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}

	return nil
}
