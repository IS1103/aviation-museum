package logger

import (
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var globalLogger *zap.Logger
var sugar *zap.SugaredLogger

// gateLogger 專用於 Gate 的 logger（不顯示 caller path）
var gateLogger *zap.Logger
var gateSugar *zap.SugaredLogger

// Config 日誌配置
type Config struct {
	Level      string // debug, info, warn, error
	Output     string // stdout, stderr, file
	FilePath   string // 日誌文件路徑（當 Output = file 時使用）
	JSON       bool   // 是否使用 JSON 格式
	CallerSkip int    // 調用棧跳過層數
}

// DefaultConfig 默認配置
func DefaultConfig() Config {
	return Config{
		Level:      "info",
		Output:     "stdout",
		JSON:       false,
		CallerSkip: 0,
	}
}

// Init 初始化日誌系統
func Init(cfg Config) error {
	// 解析日誌級別
	level := zapcore.InfoLevel
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	// 配置編碼器
	var encoderConfig zapcore.EncoderConfig
	if cfg.JSON {
		encoderConfig = zap.NewProductionEncoderConfig()
	} else {
		encoderConfig = zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05.000")

	// 選擇編碼器
	var encoder zapcore.Encoder
	if cfg.JSON {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 配置輸出
	var writeSyncer zapcore.WriteSyncer
	switch cfg.Output {
	case "stderr":
		writeSyncer = zapcore.AddSync(os.Stderr)
	case "file":
		if cfg.FilePath != "" {
			file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return err
			}
			writeSyncer = zapcore.AddSync(file)
		} else {
			writeSyncer = zapcore.AddSync(os.Stdout)
		}
	default:
		writeSyncer = zapcore.AddSync(os.Stdout)
	}

	// 創建 core
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// 創建 logger（帶 caller）
	globalLogger = zap.New(core,
		zap.AddCaller(),
		zap.AddCallerSkip(1+cfg.CallerSkip),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	sugar = globalLogger.Sugar()

	// 創建 Gate 專用 logger（不帶 caller、不帶 stack trace，只顯示時間和訊息）
	// Gate 的 error 只顯示簡化訊息，詳細 stack trace 應該在源頭服務顯示
	gateLogger = zap.New(core)
	gateSugar = gateLogger.Sugar()

	return nil
}

// InitFromEnv 從環境變量初始化日誌系統
func InitFromEnv() error {
	cfg := Config{
		Level:      getEnvOrDefault("LOG_LEVEL", "info"),
		Output:     getEnvOrDefault("LOG_OUTPUT", "stdout"),
		FilePath:   getEnvOrDefault("LOG_FILE", ""),
		JSON:       getEnvOrDefault("LOG_JSON", "false") == "true",
		CallerSkip: 0,
	}
	return Init(cfg)
}

// MustInit 初始化日誌系統，失敗則 panic
func MustInit(cfg Config) {
	if err := Init(cfg); err != nil {
		panic(err)
	}
}

// MustInitFromEnv 從環境變量初始化日誌系統，失敗則 panic
func MustInitFromEnv() {
	if err := InitFromEnv(); err != nil {
		panic(err)
	}
}

// GetLogger 獲取 zap.Logger 實例
func GetLogger() *zap.Logger {
	if globalLogger == nil {
		// 如果沒有初始化，使用默認配置
		MustInit(DefaultConfig())
	}
	return globalLogger
}

// GetSugar 獲取 zap.SugaredLogger 實例（支持格式化字符串）
func GetSugar() *zap.SugaredLogger {
	if sugar == nil {
		// 如果沒有初始化，使用默認配置
		MustInit(DefaultConfig())
	}
	return sugar
}

// Sync 刷新緩衝區（程序退出前應該調用）
func Sync() {
	if globalLogger != nil {
		_ = globalLogger.Sync()
	}
}

// 便捷方法 - Debug
func Debug(msg string, fields ...zap.Field) {
	GetLogger().Debug(msg, fields...)
}

// 便捷方法 - Info
func Info(msg string, fields ...zap.Field) {
	GetLogger().Info(msg, fields...)
}

// 便捷方法 - Warn
func Warn(msg string, fields ...zap.Field) {
	GetLogger().Warn(msg, fields...)
}

// 便捷方法 - Error
func Error(msg string, fields ...zap.Field) {
	GetLogger().Error(msg, fields...)
}

// 便捷方法 - Fatal
func Fatal(msg string, fields ...zap.Field) {
	GetLogger().Fatal(msg, fields...)
}

// 便捷方法 - Panic
func Panic(msg string, fields ...zap.Field) {
	GetLogger().Panic(msg, fields...)
}

// 格式化字符串便捷方法 - Debugf
func Debugf(template string, args ...interface{}) {
	GetSugar().Debugf(template, args...)
}

// 格式化字符串便捷方法 - Infof
func Infof(template string, args ...interface{}) {
	GetSugar().Infof(template, args...)
}

// 格式化字符串便捷方法 - Warnf
func Warnf(template string, args ...interface{}) {
	GetSugar().Warnf(template, args...)
}

// 格式化字符串便捷方法 - Errorf
func Errorf(template string, args ...interface{}) {
	GetSugar().Errorf(template, args...)
}

// 格式化字符串便捷方法 - Fatalf
func Fatalf(template string, args ...interface{}) {
	GetSugar().Fatalf(template, args...)
}

// 格式化字符串便捷方法 - Panicf
func Panicf(template string, args ...interface{}) {
	GetSugar().Panicf(template, args...)
}

// WithFields 創建帶字段的子 logger
func WithFields(fields ...zap.Field) *zap.Logger {
	return GetLogger().With(fields...)
}

// WithContext 創建帶上下文信息的子 logger
func WithContext(reqID int32, route string) *zap.Logger {
	return GetLogger().With(
		zap.Int32("reqID", reqID),
		zap.String("route", route),
		zap.Time("timestamp", time.Now()),
	)
}

// GetGateLogger 獲取 Gate 專用 logger（不顯示 caller path）
func GetGateLogger() *zap.Logger {
	if gateLogger == nil {
		MustInit(DefaultConfig())
	}
	return gateLogger
}

// GetGateSugar 獲取 Gate 專用 SugaredLogger
func GetGateSugar() *zap.SugaredLogger {
	if gateSugar == nil {
		MustInit(DefaultConfig())
	}
	return gateSugar
}

// GateLogPrefix 產生統一的請求日誌前綴 [uid:xx | traceID:xx]，便於各 service 追蹤錯誤。
// uid 可為 uint32（WS）或 string（gRPC metadata）；traceID 為字串，空時顯示 "-"。
func GateLogPrefix(uid interface{}, traceID string) string {
	uidStr := "-"
	switch v := uid.(type) {
	case uint32:
		if v != 0 {
			uidStr = fmt.Sprintf("%d", v)
		}
	case string:
		if v != "" {
			uidStr = v
		}
	}
	traceStr := traceID
	if traceStr == "" {
		traceStr = "-"
	}
	return fmt.Sprintf("[uid:%s | traceID:%s]", uidStr, traceStr)
}

// Gate 專用便捷方法 - GateInfo（不顯示 caller path）
func GateInfo(msg string, fields ...zap.Field) {
	GetGateLogger().Info(msg, fields...)
}

// Gate 專用便捷方法 - GateWarn（不顯示 caller path）
func GateWarn(msg string, fields ...zap.Field) {
	GetGateLogger().Warn(msg, fields...)
}

// Gate 專用便捷方法 - GateError（不顯示 caller path）
func GateError(msg string, fields ...zap.Field) {
	GetGateLogger().Error(msg, fields...)
}

// Gate 專用便捷方法 - GateDebug（不顯示 caller path）
func GateDebug(msg string, fields ...zap.Field) {
	GetGateLogger().Debug(msg, fields...)
}

// Gate 專用格式化便捷方法 - GateInfof（不顯示 caller path）
func GateInfof(template string, args ...interface{}) {
	GetGateSugar().Infof(template, args...)
}

// Gate 專用格式化便捷方法 - GateWarnf（不顯示 caller path）
func GateWarnf(template string, args ...interface{}) {
	GetGateSugar().Warnf(template, args...)
}

// Gate 專用格式化便捷方法 - GateErrorf（不顯示 caller path）
func GateErrorf(template string, args ...interface{}) {
	GetGateSugar().Errorf(template, args...)
}

// Gate 專用格式化便捷方法 - GateFatalf（不顯示 caller path）
func GateFatalf(template string, args ...interface{}) {
	GetGateSugar().Fatalf(template, args...)
}

// ServiceInitConfig 服務初始化日誌配置
type ServiceInitConfig struct {
	ServiceName string            // 服務名稱（顯示用）
	Fields      map[string]string // 要顯示的欄位（key -> value）
}

// LogServiceInit 輸出統一格式的服務初始化日誌
func LogServiceInit(cfg ServiceInitConfig) {
	// 計算最大 key 和 value 長度（用於對齊）
	maxKeyLen := 15
	maxValLen := 16

	// 構建表格
	var lines []string
	lines = append(lines, fmt.Sprintf("%s 服務初始化", cfg.ServiceName))
	lines = append(lines, "┌─────────────────┬──────────────────┐")
	lines = append(lines, fmt.Sprintf("│ %-*s │ %-*s │", maxKeyLen, "Key", maxValLen, "Value"))
	lines = append(lines, "├─────────────────┼──────────────────┤")

	// 按照常見的順序輸出欄位
	orderedKeys := []string{"svt", "sid", "gRPC", "WS", "IP", "pingInterval"}
	printed := make(map[string]bool)

	for _, key := range orderedKeys {
		if val, ok := cfg.Fields[key]; ok {
			lines = append(lines, fmt.Sprintf("│ %-*s │ %-*s │", maxKeyLen, key, maxValLen, val))
			printed[key] = true
		}
	}

	// 輸出剩餘的欄位
	for key, val := range cfg.Fields {
		if !printed[key] {
			lines = append(lines, fmt.Sprintf("│ %-*s │ %-*s │", maxKeyLen, key, maxValLen, val))
		}
	}

	lines = append(lines, "└─────────────────┴──────────────────┘")

	// 輸出日誌
	for i, line := range lines {
		if i == 0 {
			GateInfo(line)
		} else {
			// 直接輸出表格行（不帶時間戳）
			fmt.Println(line)
		}
	}
}

// getEnvOrDefault 獲取環境變量，如果不存在則返回默認值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
