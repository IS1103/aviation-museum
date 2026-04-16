package config

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/jackc/pgx/v5"
	"gopkg.in/yaml.v3"
)

var (
	configOnce sync.Once
	cfg        *Config
)

// ServiceSection 服務識別（對應 YAML 頂層 service）
type ServiceSection struct {
	Svt        string `yaml:"svt"`
	ServiceID  string `yaml:"service_id"`
	WSPort     int    `yaml:"ws_port"`
	HTTPPort   int    `yaml:"http_port"`   // Gin HTTP 端口（預留擴充）
	MaxPlayers int    `yaml:"max_players"` // 單房人數上限
}

// PostgresSection Postgres 連線設定
type PostgresSection struct {
	DSN string `yaml:"dsn"` // 例：postgres://user:pass@localhost:5432/air_museum?sslmode=disable
}

// Config air-museum 直連服務配置（僅 WS，無 gRPC/Redis/etcd）
type Config struct {
	Service  ServiceSection  `yaml:"service"`
	Postgres PostgresSection `yaml:"postgres"`
}

// Load 載入配置檔
func Load() {
	configOnce.Do(func() {
		candidates := []string{
			"config.local.yaml",
			"config/config.local.yaml",
			"config.yaml",
			"config/config.yaml",
			"config/config.yaml.example",
		}

		var path string
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				path = p
				break
			}
		}

		if path == "" {
			log.Println("未找到 air-museum 配置檔，使用預設配置")
			cfg = defaultConfig()
			return
		}

		data, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("讀取配置檔失敗: %v", err)
		}

		c := &Config{}
		if err := yaml.Unmarshal(data, c); err != nil {
			log.Fatalf("解析配置檔失敗: %v", err)
		}

		applyDefaults(c)
		cfg = c
	})
}

func defaultConfig() *Config {
	return &Config{
		Service: ServiceSection{WSPort: 8770, Svt: "air_museum", ServiceID: "air-museum-local"},
	}
}

func applyDefaults(c *Config) {
	if c.Service.WSPort == 0 {
		c.Service.WSPort = 8770
	}
	if c.Service.Svt == "" {
		c.Service.Svt = "air_museum"
	}
	if c.Service.ServiceID == "" {
		c.Service.ServiceID = "air-museum-local"
	}
	if c.Service.MaxPlayers <= 0 {
		c.Service.MaxPlayers = 10
	}
	if c.Service.HTTPPort <= 0 {
		c.Service.HTTPPort = 8771
	}
}

// GetWSPort 取得 WebSocket 端口
func GetWSPort() int {
	if cfg == nil {
		return defaultConfig().Service.WSPort
	}
	return cfg.Service.WSPort
}

// GetSvt 取得服務類型
func GetSvt() string {
	if cfg == nil {
		return "air_museum"
	}
	return cfg.Service.Svt
}

// GetServiceID 取得服務實例 ID
func GetServiceID() string {
	Load()
	if cfg == nil {
		return defaultConfig().Service.ServiceID
	}
	return cfg.Service.ServiceID
}

// GetMaxPlayers 取得單房人數上限
func GetMaxPlayers() int {
	Load()
	if cfg == nil {
		return 10
	}
	if cfg.Service.MaxPlayers <= 0 {
		return 10
	}
	return cfg.Service.MaxPlayers
}

// GetHTTPPort 取得 HTTP API 端口
func GetHTTPPort() int {
	Load()
	if cfg == nil {
		return 8771
	}
	if cfg.Service.HTTPPort <= 0 {
		return 8771
	}
	return cfg.Service.HTTPPort
}

// GetPostgresDSN 取得 Postgres DSN；優先使用 YAML postgres.dsn，否則依 META_POSTGRES_*（host/port/user/password）組裝，資料庫名為 air_museum。
// 空字串表示未設定。
func GetPostgresDSN() string {
	Load()
	if cfg != nil && cfg.Postgres.DSN != "" {
		return cfg.Postgres.DSN
	}
	return postgresDSNFromEnv()
}

// postgresDSNFromEnv 依 META_POSTGRES_* 取得主機／帳密，資料庫名稱為 air_museum（可選 AIR_MUSEUM_POSTGRES_DB 覆寫）。
func postgresDSNFromEnv() string {
	return postgresDSNFromEnvWithDB(getAirMuseumDBNameFromEnv())
}

func getAirMuseumDBNameFromEnv() string {
	dbname := os.Getenv("AIR_MUSEUM_POSTGRES_DB")
	if dbname == "" {
		return "air_museum"
	}
	return dbname
}

// postgresDSNFromEnvWithDB 依 META_POSTGRES_* 組出 DSN，dbname 由參數指定。
func postgresDSNFromEnvWithDB(dbname string) string {
	host := os.Getenv("META_POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("META_POSTGRES_PORT")
	if port == "" {
		port = "5433"
	}
	user := os.Getenv("META_POSTGRES_USER")
	if user == "" {
		user = "metadata"
	}
	password := os.Getenv("META_POSTGRES_PASSWORD")
	if password == "" {
		password = "metadata"
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
}

// GetPostgresBootstrapDSN 取得連往預設庫 postgres 的 DSN，僅在「依環境變數組 DSN」時有值，用於啟動時自動建庫。
// 若使用 YAML postgres.dsn 則回傳空字串，不自動建庫。
func GetPostgresBootstrapDSN() string {
	Load()
	if cfg != nil && cfg.Postgres.DSN != "" {
		return ""
	}
	return postgresDSNFromEnvWithDB("postgres")
}

// GetAirMuseumDBName 取得目標資料庫名稱（僅在依環境變數組 DSN 時有值，供自動建庫用）。
func GetAirMuseumDBName() string {
	Load()
	if cfg != nil && cfg.Postgres.DSN != "" {
		return ""
	}
	return getAirMuseumDBNameFromEnv()
}

// PostgresEnsureParams 回傳連到預設庫 postgres 的 bootstrap DSN 與「主 DSN」中的目標庫名，供啟動時若目標庫不存在則 CREATE DATABASE。
// 若未設定主 DSN、或目標庫為 postgres／無法解析，則回傳空字串（略過自動建庫）。
func PostgresEnsureParams() (bootstrapDSN, targetDB string) {
	Load()
	mainDSN := GetPostgresDSN()
	if mainDSN == "" {
		return "", ""
	}
	if cfg != nil && cfg.Postgres.DSN != "" {
		b, t, err := bootstrapDSNFromTargetDSN(cfg.Postgres.DSN)
		if err != nil {
			log.Printf("[postgres] 解析 dsn 以供自動建庫: %v", err)
			return "", ""
		}
		return b, t
	}
	return postgresDSNFromEnvWithDB("postgres"), getAirMuseumDBNameFromEnv()
}

// bootstrapDSNFromTargetDSN 由連線字串複製連線參數，將 dbname 改為 postgres，並回傳原 dbname。
func bootstrapDSNFromTargetDSN(targetDSN string) (bootstrapDSN, targetDB string, err error) {
	pc, err := pgx.ParseConfig(targetDSN)
	if err != nil {
		return "", "", err
	}
	targetDB = pc.Database
	if targetDB == "" || targetDB == "postgres" {
		return "", targetDB, nil
	}
	bc := pc.Copy()
	bc.Database = "postgres"
	return bc.ConnString(), targetDB, nil
}
