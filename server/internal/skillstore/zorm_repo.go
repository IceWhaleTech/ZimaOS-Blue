// Package skillstore provides skill storage using zorm for optimized database operations.
package skillstore

import (
	"context"
	"database/sql"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
)

// ZormStore wraps zorm operations for skill storage.
type ZormStore struct {
	db *sql.DB
}

// NewZormStore creates a new zorm-based store from existing sql.DB.
func NewZormStore(db *sql.DB) *ZormStore {
	return &ZormStore{db: db}
}

// UpdateReadmeBatchZorm updates readme content for multiple skills using zorm.
// Only updates records where the readme hash has changed.
func (zs *ZormStore) UpdateReadmeBatchZorm(ctx context.Context, updates []readmeUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	tx, err := zs.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := timeutil.NowTime()
	table := z.TableContext(ctx, tx, "skills")

	for _, u := range updates {
		if _, err := table.Update(
			z.V{
				"readme":      u.Readme,
				"readme_hash": u.Hash,
				"updated_at":  now,
			},
			z.Where(
				z.Eq("id", u.ID),
				z.Or(
					z.IsNull("readme_hash"),
					z.Neq("readme_hash", u.Hash),
				),
			),
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateReadmeZorm updates readme for a single skill using zorm.
func (zs *ZormStore) UpdateReadmeZorm(ctx context.Context, id, readme, hash string) (int64, error) {
	t := z.TableContext(ctx, zs.db, "skills")
	n, err := t.Update(
		z.V{
			"readme":      readme,
			"readme_hash": hash,
			"updated_at":  timeutil.NowTime(),
		},
		z.Where(
			z.Eq("id", id),
			z.Or(
				z.IsNull("readme_hash"),
				z.Neq("readme_hash", hash),
			),
		),
	)
	return int64(n), err
}
