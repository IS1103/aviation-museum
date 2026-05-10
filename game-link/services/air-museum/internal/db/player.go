package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"air-museum/config"

	pb "air-museum/proto/pb"

	"internal/logger"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	schemaName   = "air_museum"
	playerTable  = "player"
	missionTable = "mission"
	logTable     = "log"

	// MaxReservedFixtureUID：player 表內建之固定終端種子列 uid（1～4），與 SERIAL 新列分離（新玩家自 5 起）。
	MaxReservedFixtureUID uint32 = 4
	// PlayerRegisterScreenUID air_museum/add_player 推播目標之 WS 連線 uid（種子）。
	PlayerRegisterScreenUID uint32 = 2
)

// Fixture 種子列使用的 name，與 GetFixtureAuthUID 驗證一致；請勿當一般玩家顯示。
const (
	fixtureNameGameScreen           = "__fixture.gamescreen"
	fixtureNamePlayerRegisterScreen = "__fixture.playerregisterscreen"
	fixtureNameAirplaneScreen       = "__fixture.airplanescreen"
	fixtureNamePlayerGameEndScreen  = "__fixture.playergameendscreen"
)

// IsReservedFixtureUID：true 表示為固定終端種子列，device=player 之 token 不可用此 uid 續連。
func IsReservedFixtureUID(uid uint32) bool {
	return uid >= 1 && uid <= MaxReservedFixtureUID
}

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

	if err := ensureFixturePlayerRows(ctx); err != nil {
		return err
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

	if err := ensurePlayerUIDSequence(ctx); err != nil {
		return err
	}

	return nil
}

// ensurePlayerUIDSequence 將 player.uid 之 SERIAL 下一個值拉到 ≥5（1～4 已預留為固定終端種子列）。
func ensurePlayerUIDSequence(ctx context.Context) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf(`
		SELECT setval(
			pg_get_serial_sequence('%s.%s', 'uid'),
			GREATEST(4, COALESCE((SELECT MAX(uid) FROM %s), 0))
		)
	`, schemaName, playerTable, qualifiedTable(playerTable)))
	if err != nil {
		return fmt.Errorf("set player uid sequence: %w", err)
	}
	return nil
}

// ensureFixturePlayerRows 寫入 uid 1～4 之固定終端列（與 auth 裝置對照）；既有列以 ON CONFLICT 略過。
func ensureFixturePlayerRows(ctx context.Context) error {
	_, err := db.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (uid, name, age, sex) VALUES
			(1, $1, 0, 0),
			(2, $2, 0, 0),
			(3, $3, 0, 0),
			(4, $4, 0, 0)
		ON CONFLICT (uid) DO NOTHING
	`, qualifiedTable(playerTable)),
		fixtureNameGameScreen,
		fixtureNamePlayerRegisterScreen,
		fixtureNameAirplaneScreen,
		fixtureNamePlayerGameEndScreen,
	)
	if err != nil {
		return fmt.Errorf("seed fixture %s: %w", playerTable, err)
	}
	return nil
}

// IsFixtureDevice 表示已 norm（小寫）之 ValidateReq.device 對應 player 種子終端（1～4）。
func IsFixtureDevice(normDevice string) bool {
	_, _, ok := fixtureUIDAndName(normDevice)
	return ok
}

// fixtureUIDAndName 將已 norm 之 device（小寫無空白）對應到種子 uid 與預期 name。projector 視同 gamescreen。
func fixtureUIDAndName(normDevice string) (uid uint32, name string, ok bool) {
	switch normDevice {
	case "gamescreen", "projector":
		return 1, fixtureNameGameScreen, true
	case "playerregisterscreen":
		return 2, fixtureNamePlayerRegisterScreen, true
	case "airplanescreen":
		return 3, fixtureNameAirplaneScreen, true
	case "playergameendscreen":
		return 4, fixtureNamePlayerGameEndScreen, true
	default:
		return 0, "", false
	}
}

// GetFixtureAuthUID 依裝置別從 player 表取出種子列 uid；該列須存在且 name 與建表種子一致。
func GetFixtureAuthUID(ctx context.Context, normDevice string) (uint32, error) {
	if db == nil {
		return 0, fmt.Errorf("db not initialized")
	}
	expectUID, expectName, ok := fixtureUIDAndName(normDevice)
	if !ok {
		return 0, fmt.Errorf("device %q has no fixture uid", normDevice)
	}
	var gotName string
	err := db.QueryRowContext(ctx, fmt.Sprintf(
		`SELECT name FROM %s WHERE uid = $1 LIMIT 1`,
		qualifiedTable(playerTable),
	), expectUID).Scan(&gotName)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("fixture player row missing for uid=%d (re-run service ensureTable)", expectUID)
	}
	if err != nil {
		return 0, err
	}
	if gotName != expectName {
		return 0, fmt.Errorf("fixture uid=%d name mismatch: db=%q expected=%q", expectUID, gotName, expectName)
	}
	return expectUID, nil
}

// PlayerExists 查詢 player 表是否已有該 uid（供續連驗證；含 1～4 種子列）。
func PlayerExists(ctx context.Context, uid uint32) (bool, error) {
	if db == nil {
		return false, fmt.Errorf("db not initialized")
	}
	var dummy int
	err := db.QueryRowContext(ctx, fmt.Sprintf(
		`SELECT 1 FROM %s WHERE uid = $1 LIMIT 1`,
		qualifiedTable(playerTable),
	), uid).Scan(&dummy)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Close 關閉 DB 連線
func Close() error {
	if db == nil {
		return nil
	}
	return db.Close()
}

// LoadAddPlayerNotify 依 uid 讀 player 一整列並填 AddPlayerNotify；無列或非預期掃描則錯。
func LoadAddPlayerNotify(ctx context.Context, uid uint32) (*pb.AddPlayerNotify, error) {
	if db == nil {
		return nil, fmt.Errorf("db not initialized")
	}
	var (
		gotUID                                uint32
		mission                               int32
		name                                  string
		age                                   int32
		sex                                   int32
		eyes, eyebrow, mouth, glasses, helmet int32
		gameScore, landingScore, ranking      int32
		creatUnix                             int64
	)
	q := fmt.Sprintf(`
		SELECT uid, mission, name, age, sex,
			avatar_eyes, avatar_eyebrow, avatar_mouth, avatar_glasses, avatar_helmet,
			game_score, landing_score, ranking,
			COALESCE(EXTRACT(EPOCH FROM creattime)::bigint, 0)
		FROM %s WHERE uid = $1
		LIMIT 1
	`, qualifiedTable(playerTable))
	err := db.QueryRowContext(ctx, q, uid).Scan(
		&gotUID, &mission, &name, &age, &sex,
		&eyes, &eyebrow, &mouth, &glasses, &helmet,
		&gameScore, &landingScore, &ranking,
		&creatUnix,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("player uid=%d not found", uid)
	}
	if err != nil {
		return nil, err
	}
	return &pb.AddPlayerNotify{
		Uid:                  gotUID,
		Name:                 name,
		Age:                  uint32(age),
		Sex:                  sex,
		AvatarEyes:           eyes,
		AvatarEyebrow:        eyebrow,
		AvatarMouth:          mouth,
		AvatarGlasses:        glasses,
		AvatarHelmet:         helmet,
		Mission:              mission,
		GameScore:            gameScore,
		LandingScore:         landingScore,
		Ranking:              ranking,
		CreattimeUnixSeconds: creatUnix,
	}, nil
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
