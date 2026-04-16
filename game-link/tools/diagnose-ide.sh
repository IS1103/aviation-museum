#!/bin/sh
# IDE/gopls 診斷腳本 - 檢查版本差異與環境
set -e
cd "$(dirname "$0")/.."

echo "=== Go 環境 ==="
go version
echo ""

echo "=== gopls 版本 ==="
gopls version 2>/dev/null || echo "gopls 未安裝"
echo ""

echo "=== 關鍵依賴版本差異 (可能導致 IDE 誤報) ==="
echo ""
echo "google.golang.org/grpc:"
find internal proto utils services -name "go.mod" -exec grep -h "google.golang.org/grpc v" {} \; 2>/dev/null | grep -v "// indirect" | sort -u
echo ""
echo "google.golang.org/protobuf:"
find internal proto utils services -name "go.mod" -exec grep -h "google.golang.org/protobuf v" {} \; 2>/dev/null | grep -v "// indirect" | sort -u
echo ""
echo "google.golang.org/genproto/googleapis/api:"
find internal proto utils services -name "go.mod" -exec grep -h "google.golang.org/genproto/googleapis/api" {} \; 2>/dev/null | sort -u
echo ""

echo "=== 驗證編譯 (若成功則為 gopls 誤報) ==="
go build ./internal/... ./proto/... ./utils/... ./services/gate/... 2>&1 && echo "OK: 編譯成功" || echo "FAIL: 編譯失敗"
echo ""

echo "=== 建議 ==="
echo "1. 若編譯成功但 IDE 報錯：多半是 gopls 與 go.work 多模組的已知問題"
echo "2. 版本不一致可能加重 gopls 混淆，可考慮統一各 go.mod 的 grpc/protobuf 版本"
echo "3. 啟用 gopls 日誌：在 .vscode/settings.json 加入 gopls 的 verboseOutput 或設定 logfile"
