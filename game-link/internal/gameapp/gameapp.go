package gameapp

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	grpcapp "internal/grpc/app"
	"internal/rule"

	profilepb "internal.proto/pb/profile"

	"google.golang.org/grpc"
)

// Config 定義遊戲服務啟動配置：僅從本地 playerSetting/gameSetting 載入，不呼叫 GetGameInfo（GetGameInfo 供 web-dashboard 等取得遊戲基本資料用）。
type Config struct {
	RuntimeConfig   grpcapp.RuntimeConfig
	Register        func(s *grpc.Server)
	SetRulecache    func(*rule.Cache)   // 由各遊戲傳入自己的 rulecache.Set
	InitGameModule  func(*profilepb.GetGameInfo_Resp) error // 可為 nil；傳入由本地檔組裝的 resp，供遊戲從 game_settings 等組裝 config
	OnShutdown      func()

	// 必填：遊戲服務僅從本地檔案載入 rule 與 game_settings，未設定或缺檔則啟動失敗。
	LocalPlayerSettingPath string // 例：config/playerSetting.json（player_modes / rule JSON）
	LocalGameSettingPath   string // 例：config/gameSetting.json（game_settings JSON）
}

// readLocalFile 依候選路徑讀取檔案，回傳第一個存在且可讀的內容；皆不存在則回傳 ""。
func readLocalFile(candidates []string) string {
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data)
		}
	}
	return ""
}

// RunGame 啟動遊戲 gRPC 服務：BeforeListen 內僅從本地 playerSetting/gameSetting 載入 → SetRulecache → InitGameModule。不呼叫 GetGameInfo（該 API 供 web-dashboard 取得遊戲基本資料用）。
func RunGame(cfg Config) error {
	if cfg.SetRulecache == nil {
		return fmt.Errorf("gameapp: SetRulecache 不能為空")
	}
	if cfg.LocalPlayerSettingPath == "" {
		return fmt.Errorf("gameapp: LocalPlayerSettingPath 必填（例：config/playerSetting.json）")
	}
	if cfg.LocalGameSettingPath == "" {
		return fmt.Errorf("gameapp: LocalGameSettingPath 必填（例：config/gameSetting.json）")
	}
	beforeListen := func() error {
		svt := cfg.RuntimeConfig.ServiceName
		playerPath := cfg.LocalPlayerSettingPath
		gamePath := cfg.LocalGameSettingPath

		playerCandidates := []string{playerPath}
		if filepath.Dir(playerPath) != "." {
			playerCandidates = append(playerCandidates, filepath.Join("..", playerPath), filepath.Base(playerPath))
		}
		gameCandidates := []string{gamePath}
		if filepath.Dir(gamePath) != "." {
			gameCandidates = append(gameCandidates, filepath.Join("..", gamePath), filepath.Base(gamePath))
		}

		ruleJSON := readLocalFile(playerCandidates)
		gameSettingsJSON := readLocalFile(gameCandidates)
		if ruleJSON == "" {
			return fmt.Errorf("找不到 playerSetting 檔案（已嘗試 %v）", playerCandidates)
		}
		if gameSettingsJSON == "" {
			return fmt.Errorf("找不到 gameSetting 檔案（已嘗試 %v）", gameCandidates)
		}

		resp := &profilepb.GetGameInfo_Resp{
			Svt:                    svt,
			Rule:                   ruleJSON,
			GameSettingsJson:       gameSettingsJSON,
			GameSettingsSchemaJson: "{}",
		}
		log.Printf("[%s] 使用本地 config: playerSetting + gameSetting", svt)

		cache, err := rule.NewCache(resp.GetRule())
		if err != nil {
			return err
		}
		cfg.SetRulecache(cache)
		if cfg.InitGameModule != nil {
			if err := cfg.InitGameModule(resp); err != nil {
				return err
			}
		}
		return nil
	}
	return grpcapp.Run(grpcapp.Config{
		RuntimeConfig: cfg.RuntimeConfig,
		Register:      cfg.Register,
		BeforeListen:  beforeListen,
		OnShutdown:    cfg.OnShutdown,
	})
}
