#!/bin/sh
# 統一升級 grpc、protobuf、genproto 到最高版本
set -e
cd "$(dirname "$0")/.."

MODULES="internal proto utils
  services/auth services/baccarat services/bigtwo services/bingo services/chat
  services/coin-pusher services/coop-kitchen services/fish services/gate
  services/gomoku services/history services/holdem services/horse-racing
  services/match services/mj services/monopoly services/niuniu services/paigow
  services/profile services/roulette services/sanguo services/thirteen
  services/three-heroes services/werewolf services/zombie
  services/werewolf/coop-kitchen services/paigow/coop-kitchen"

for mod in $MODULES; do
  if [ -f "$mod/go.mod" ]; then
    echo "=== Upgrading $mod ==="
    (cd "$mod" && go get google.golang.org/grpc@v1.71.0 google.golang.org/protobuf@v1.36.11 && go mod tidy)
  fi
done

echo ""
echo "=== Syncing go.work ==="
go work sync

echo ""
echo "=== Done ==="
