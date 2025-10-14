package repo

import (
	"context"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ApplyMigrations executes SQL files against the provided pool in lexicographical order.
func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool, filesystem fs.FS) error {
	entries, err := fs.ReadDir(filesystem, ".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		sqlBytes, err := fs.ReadFile(filesystem, entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}

		if len(sqlBytes) == 0 {
			continue
		}

		if err := executeSQL(ctx, pool, string(sqlBytes)); err != nil {
			return fmt.Errorf("execute migration %s: %w", entry.Name(), err)
		}
	}

	return nil
}

func executeSQL(ctx context.Context, pool *pgxpool.Pool, sql string) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, sql)
		return err
	})
}
