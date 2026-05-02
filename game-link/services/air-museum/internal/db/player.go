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

const (
	schemaName   = "air_museum"
	playerTable  = "player"
	missionTable = "mission"
	logTable     = "log"
)

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

// qualifiedTable 回傳 "schema"."table" 形式的 schema-qualified 識別子，
// 避免連線帳號於 public schema 無 CREATE 權限時失敗（PG 15+ 常見）。
func qualifiedTable(table string) string {
	return quoteIdent(schemaName) + "." + quoteIdent(table)
}

func ensureTable(ctx context.Context) error {
	// 先建立專屬 schema，避開 public 權限問題（SQLSTATE 3F000）。
	if _, err := db.ExecContext(ctx, fmt.Sprintf(
		`CREATE SCHEMA IF NOT EXISTS %s`, quoteIdent(schemaName),
	)); err != nil {
		return fmt.Errorf("create schema %s: %w", schemaName, err)
	}

	if _, err := db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			uid SERIAL PRIMARY KEY,
			mission INT NOT NULL DEFAULT 0,
			name TEXT NOT NULL DEFAULT '',
			age INT NOT NULL DEFAULT 0,
			sex INT NOT NULL DEFAULT 0,
			avatar_glasses INT NOT NULL DEFAULT 0,
			avatar_helmet INT NOT NULL DEFAULT 0,
			avatar_eyes INT NOT NULL DEFAULT 0,
			avatar_eyebrow INT NOT NULL DEFAULT 0,
			avatar_mouth INT NOT NULL DEFAULT 0,
			game_score INT NOT NULL DEFAULT 0,
			landing_score INT NOT NULL DEFAULT 0,
			ranking INT NOT NULL DEFAULT 0,
			creattime TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`, qualifiedTable(playerTable))); err != nil {
		return fmt.Errorf("create %s: %w", playerTable, err)
	}

	if _, err := db.ExecContext(ctx, fmt.Sprintf(
		`ALTER TABLE %s ADD COLUMN IF NOT EXISTS avatar_eyebrow INT NOT NULL DEFAULT 0`,
		qualifiedTable(playerTable),
	)); err != nil {
		return fmt.Errorf("alter %s avatar_eyebrow: %w", playerTable, err)
	}

	if _, err := db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			mission SERIAL PRIMARY KEY,
			mission_num INT NOT NULL DEFAULT 0,
			landing_type INT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`, qualifiedTable(missionTable))); err != nil {
		return fmt.Errorf("create %s: %w", missionTable, err)
	}

	if _, err := db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id BIGSERIAL PRIMARY KEY,
			type INT NOT NULL DEFAULT 0,
			msg JSONB NOT NULL DEFAULT '{}'::JSONB,
			creattime TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`, qualifiedTable(logTable))); err != nil {
		return fmt.Errorf("create %s: %w", logTable, err)
	}

	return nil
}

// Close 關閉 DB 連線
func Close() error {
	if db == nil {
		return nil
	}
	return db.Close()
}

// CreatePlayer 新增一筆玩家資料（name/age/sex）到 player 表，回傳自動產生的 uid。
// 其餘欄位使用 DB 預設值（avatar_*、game_score、landing_score、ranking、mission 皆為 0；creattime 為 NOW()）。
func CreatePlayer(ctx context.Context, name string, age, sex int) (uint32, error) {
	if db == nil {
		return 0, fmt.Errorf("db not initialized")
	}
	var uid uint32
	query := fmt.Sprintf(`
		INSERT INTO %s (name, age, sex)
		VALUES ($1, $2, $3)
		RETURNING uid
	`, qualifiedTable(playerTable))
	if err := db.QueryRowContext(ctx, query, name, age, sex).Scan(&uid); err != nil {
		return 0, fmt.Errorf("insert player: %w", err)
	}
	return uid, nil
}

// UpdatePlayerProfile 依 uid 更新玩家姓名、年齡、性別與裝扮索引（含眼鏡 -1 未戴）。
func UpdatePlayerProfile(ctx context.Context, uid uint32, name string, age, sex, eyes, eyebrow, mouth, glasses, helmet int) error {
	if db == nil {
		return fmt.Errorf("db not initialized")
	}
	query := fmt.Sprintf(`
		UPDATE %s SET
			name = $2,
			age = $3,
			sex = $4,
			avatar_eyes = $5,
			avatar_eyebrow = $6,
			avatar_mouth = $7,
			avatar_glasses = $8,
			avatar_helmet = $9
		WHERE uid = $1
	`, qualifiedTable(playerTable))
	res, err := db.ExecContext(ctx, query, uid, name, age, sex, eyes, eyebrow, mouth, glasses, helmet)
	if err != nil {
		return fmt.Errorf("update player: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no player row for uid=%d", uid)
	}
	return nil
}
