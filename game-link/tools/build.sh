#!/bin/sh
# Build all Go modules. Root has go.work but no go.mod,
# so each module path must be specified explicitly.
set -e
cd "$(dirname "$0")/.."
go build ./internal/... ./proto/... ./utils/... ./services/auth/... ./services/baccarat/... ./services/bigtwo/... ./services/bingo/... ./services/chat/... ./services/coin-pusher/... ./services/coop-kitchen/... ./services/fish/... ./services/gate/... ./services/gomoku/... ./services/history/... ./services/holdem/... ./services/horse-racing/... ./services/match/... ./services/mj/... ./services/monopoly/... ./services/niuniu/... ./services/paigow/... ./services/profile/... ./services/purchase/... ./services/roulette/... ./services/sanguo/... ./services/thirteen/... ./services/three-heroes/... ./services/werewolf/... ./services/zombie/...
