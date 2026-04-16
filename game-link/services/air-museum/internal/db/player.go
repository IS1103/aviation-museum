package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"air-museum/config"

	"internal/logger"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const playerTable = "player"

var (
	once sync.Once
	db   *sql.DB
)

// 僅允許資料庫名稱含英數字與底線，避免 CREATE DATABASE 時注入
var safeDBNameRe = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// Init 連線 Postgres；啟動前若目標資料庫不存在會先連到 postgres 建庫，再建立所需資料表。DSN 為空則不連線。
// 第二個回傳值於「有跑自動建庫檢查且成功」時為 true 表示該資料庫在啟動前就已存在；其餘情況為 false。
func Init(ctx context.Context) (bool, error) {
	var existedAtEnsure bool
	var err error
	once.Do(func() {
		dsn := config.GetPostgresDSN()
		if dsn == "" {
			return
		}
		bootstrapDSN, targetDB := config.PostgresEnsureParams()
		if bootstrapDSN != "" && targetDB != "" && targetDB != "postgres" && safeDBNameRe.MatchString(targetDB) {
			logger.GateInfo(fmt.Sprintf("[postgres] ensuring database %q exists...", targetDB))
			existed, errCreate := ensureDatabaseExists(ctx, bootstrapDSN, targetDB)
			if errCreate != nil {
				logger.GateWarnf("[postgres] auto-create database failed (will try connect anyway): %v", errCreate)
			} else {
				existedAtEnsure = existed
				if !existed {
					logger.GateInfo(fmt.Sprintf("[postgres] database %q created", targetDB))
				}
			}
		}
		db, err = sql.Open("pgx", dsn)
		if err != nil {
			return
		}
		if err = db.PingContext(ctx); err != nil {
			return
		}
		err = ensureTable(ctx)
	})
	return existedAtEnsure, err
}

// ensureDatabaseExists 連到 bootstrap（postgres），若 targetDB 不存在則建立。連線帳號需具備 CREATEDB 權限。
// alreadyExisted 為 true 表示查詢時資料庫已存在，未執行 CREATE DATABASE。
func ensureDatabaseExists(ctx context.Context, bootstrapDSN, targetDB string) (alreadyExisted bool, err error) {
	conn, err := sql.Open("pgx", bootstrapDSN)
	if err != nil {
		return false, fmt.Errorf("bootstrap connect: %w", err)
	}
	defer conn.Close()
	if err = conn.PingContext(ctx); err != nil {
		return false, fmt.Errorf("bootstrap ping: %w", err)
	}
	var exists int
	err = conn.QueryRowContext(ctx, "SELECT 1 FROM pg_database WHERE datname = $1", targetDB).Scan(&exists)
	if err == nil {
		return true, nil
	}
	if err != sql.ErrNoRows {
		return false, fmt.Errorf("check db: %w", err)
	}
	_, err = conn.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", quoteIdent(targetDB)))
	if err != nil {
		return false, fmt.Errorf("CREATE DATABASE: %w", err)
	}
	return false, nil
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func ensureTable(ctx context.Context) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL DEFAULT '',
			age INT NOT NULL DEFAULT 0,
			sex INT NOT NULL DEFAULT 0,
			avatar JSONB NOT NULL DEFAULT '{}'::jsonb,
			score INT NOT NULL DEFAULT 0
		);
	`, quoteIdent(playerTable)))
	return err
}

// Close 關閉 DB 連線
func Close() error {
	if db == nil {
		return nil
	}
	return db.Close()
}
