package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func RunMigrations(db *sql.DB, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, file := range files {
		if filepath.Base(file) == "003_add_recycle_root_id.sql" {
			if err := ensureRecycleRootIDColumn(db); err != nil {
				return fmt.Errorf("run migration %s failed: %w", filepath.Base(file), err)
			}
			continue
		}

		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}

		statements := strings.Split(string(content), ";")
		for _, statement := range statements {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if _, err := db.Exec(statement); err != nil {
				return fmt.Errorf("run migration %s failed: %w", filepath.Base(file), err)
			}
		}
	}

	return nil
}

func ensureRecycleRootIDColumn(db *sql.DB) error {
	exists, err := columnExists(db, "matter", "recycle_root_id")
	if err != nil {
		return err
	}
	if !exists {
		if _, err := db.Exec("ALTER TABLE matter ADD COLUMN recycle_root_id BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'recycle root matter id'"); err != nil {
			return err
		}
	}

	hasIndex, err := indexExists(db, "matter", "idx_user_recycle_root")
	if err != nil {
		return err
	}
	if !hasIndex {
		if _, err := db.Exec("CREATE INDEX idx_user_recycle_root ON matter (user_id, recycle_root_id)"); err != nil {
			return err
		}
	}

	return backfillRecycleRootID(db)
}

func columnExists(db *sql.DB, tableName, columnName string) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = ?
  AND COLUMN_NAME = ?`,
		tableName, columnName,
	).Scan(&count)
	return count > 0, err
}

func indexExists(db *sql.DB, tableName, indexName string) (bool, error) {
	var count int
	err := db.QueryRow(
		`SELECT COUNT(*)
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = ?
  AND INDEX_NAME = ?`,
		tableName, indexName,
	).Scan(&count)
	return count > 0, err
}

type recycledMatterNode struct {
	id       uint64
	parentID uint64
}

func backfillRecycleRootID(db *sql.DB) error {
	rows, err := db.Query("SELECT id, parent_id FROM matter WHERE status = 2 AND recycle_root_id = 0")
	if err != nil {
		return err
	}
	defer rows.Close()

	nodes := make(map[uint64]recycledMatterNode)
	for rows.Next() {
		var node recycledMatterNode
		if err := rows.Scan(&node.id, &node.parentID); err != nil {
			return err
		}
		nodes[node.id] = node
	}
	if err := rows.Err(); err != nil {
		return err
	}

	rootCache := make(map[uint64]uint64)
	for id := range nodes {
		rootID := recycleRootForNode(id, nodes, rootCache)
		if _, err := db.Exec(
			"UPDATE matter SET recycle_root_id = ? WHERE id = ? AND status = 2 AND recycle_root_id = 0",
			rootID, id,
		); err != nil {
			return err
		}
	}

	return nil
}

func recycleRootForNode(id uint64, nodes map[uint64]recycledMatterNode, rootCache map[uint64]uint64) uint64 {
	if rootID, ok := rootCache[id]; ok {
		return rootID
	}

	node := nodes[id]
	if _, ok := nodes[node.parentID]; !ok {
		rootCache[id] = id
		return id
	}

	rootID := recycleRootForNode(node.parentID, nodes, rootCache)
	rootCache[id] = rootID
	return rootID
}
