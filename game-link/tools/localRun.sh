#!/bin/bash

# Local development environment startup script
# Features:
# 1. Start docker-compose infrastructure (etcd, redis, postgres)
# 2. Wait for health checks
# 3. Start services using Air + tmux (auth, chat, history, gate, match, profile)
#    - Base services run in a split window layout
#      * Left side: gate, auth, history, web-games
#      * Right side: profile, match, chat, web-dashboard
#    - Other services run in separate windows
#    - Services persist after terminal closes
#    - Easy to view/switch between service logs

set -e

# Get project root (parent of tools/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$SCRIPT_DIR"

# Color definitions
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if Air is installed
check_air() {
    # Ensure GOPATH/bin is in PATH
    if [ -n "$GOPATH" ]; then
        export PATH="$PATH:$GOPATH/bin"
    else
        # Try to get GOPATH from go env
        local go_path=$(go env GOPATH 2>/dev/null)
        if [ -n "$go_path" ]; then
            export PATH="$PATH:$go_path/bin"
        fi
    fi
    
    if ! command -v air &> /dev/null; then
        echo -e "${YELLOW}⚠️  Air not installed, installing...${NC}"
        go install github.com/air-verse/air@latest
        if [ $? -ne 0 ]; then
            echo -e "${RED}❌ Air installation failed, please install manually: go install github.com/air-verse/air@latest${NC}"
            exit 1
        fi
        # Update PATH after installation
        local go_path=$(go env GOPATH 2>/dev/null)
        if [ -n "$go_path" ]; then
            export PATH="$PATH:$go_path/bin"
        fi
        echo -e "${GREEN}✅ Air installed successfully${NC}"
    else
        echo -e "${GREEN}✅ Air is already installed${NC}"
    fi
}

# Verify Docker can actually run containers (not just docker info)
# WSL2 已知問題：docker info 可能成功但容器無法啟動，直到開啟 Docker Desktop 介面
verify_docker_ready() {
    if command -v timeout &> /dev/null; then
        timeout 15 docker run --rm alpine echo "ok" &> /dev/null
    elif command -v gtimeout &> /dev/null; then
        gtimeout 15 docker run --rm alpine echo "ok" &> /dev/null
    else
        docker run --rm alpine echo "ok" &> /dev/null
    fi
}

# Check if Docker is running, and try to start it if not
check_docker() {
    if docker info &> /dev/null; then
        echo -e "${GREEN}✅ Docker is already running${NC}"
        # WSL2 額外驗證：確保能實際執行容器（避免「假運行」狀態）
        if verify_docker_ready; then
            return 0
        fi
        echo -e "${YELLOW}⚠️  Docker 回應異常，嘗試開啟 Docker Desktop 以完成初始化...${NC}"
        if [ -n "$WSL_DISTRO_NAME" ] || [ -n "$WSLENV" ] || [ -n "$MSYSTEM" ]; then
            local c_drive="/mnt/c"
            [ -d "/c/Program Files" ] && c_drive="/c"
            local docker_exe="$c_drive/Program Files/Docker/Docker/Docker Desktop.exe"
            [ ! -f "$docker_exe" ] && docker_exe="$c_drive/Program Files/Docker/Docker/frontend/Docker Desktop.exe"
            if [ -f "$docker_exe" ]; then
                local win_path=$(wslpath -w "$docker_exe" 2>/dev/null)
                [ -z "$win_path" ] && win_path=$(echo "$docker_exe" | sed 's|^/c/|C:/|' | sed 's|^/mnt/c/|C:/|' | sed 's|/|\\|g')
                [ -n "$win_path" ] && cmd.exe /c start "" "$win_path" 2>/dev/null || true
            fi
        fi
        echo -e "${YELLOW}⏳ 等待 Docker 完全就緒（請確認 Docker Desktop 視窗已開啟）...${NC}"
        local max_wait=60
        local waited=0
        while [ $waited -lt $max_wait ]; do
            if verify_docker_ready; then
                echo -e " ${GREEN}✅${NC}"
                return 0
            fi
            echo -n "."
            sleep 2
            waited=$((waited + 2))
        done
        echo ""
        echo -e "${RED}❌ Docker 無法正常執行容器${NC}"
        echo -e "${YELLOW}   請手動開啟 Docker Desktop 視窗，等待完全啟動後再執行此腳本${NC}"
        exit 1
    fi
    
    echo -e "${YELLOW}⚠️  Docker is not running, attempting to start...${NC}"
    
    # Try to start Docker based on environment
    local started=false
    
    if [ -n "$WSL_DISTRO_NAME" ] || [ -n "$WSLENV" ] || [ -n "$MSYSTEM" ]; then
        # WSL or Git Bash (MSYS2) - launch Docker Desktop via Windows
        local c_drive="/mnt/c"
        [ -d "/c/Program Files" ] && c_drive="/c"  # Git Bash uses /c/
        local docker_paths=(
            "$c_drive/Program Files/Docker/Docker/Docker Desktop.exe"
            "$c_drive/Program Files/Docker/Docker/frontend/Docker Desktop.exe"
        )
        for docker_exe in "${docker_paths[@]}"; do
            if [ -f "$docker_exe" ]; then
                echo -e "${BLUE}   Starting Docker Desktop...${NC}"
                local win_path
                win_path=$(wslpath -w "$docker_exe" 2>/dev/null)
                if [ -n "$win_path" ]; then
                    cmd.exe /c start "" "$win_path" 2>/dev/null || true
                else
                    # wslpath not available (e.g. Git Bash), convert path manually
                    win_path=$(echo "$docker_exe" | sed 's|^/c/|C:/|' | sed 's|^/mnt/c/|C:/|' | sed 's|/|\\|g')
                    if [ -n "$win_path" ]; then
                        cmd.exe /c start "" "$win_path" 2>/dev/null || true
                    else
                        powershell.exe -Command "Start-Process 'C:\\Program Files\\Docker\\Docker\\Docker Desktop.exe'" 2>/dev/null || true
                    fi
                fi
                started=true
                break
            fi
        done
    elif [ "$(uname)" = "Darwin" ]; then
        # macOS - open Docker Desktop app
        if [ -d "/Applications/Docker.app" ]; then
            echo -e "${BLUE}   Starting Docker Desktop...${NC}"
            open -a Docker
            started=true
        fi
    elif [ "$(uname)" = "Linux" ]; then
        # Native Linux - try systemd or service
        if command -v systemctl &> /dev/null; then
            echo -e "${BLUE}   Starting Docker service...${NC}"
            sudo systemctl start docker 2>/dev/null && started=true
        elif command -v service &> /dev/null; then
            echo -e "${BLUE}   Starting Docker service...${NC}"
            sudo service docker start 2>/dev/null && started=true
        fi
    fi
    
    if [ "$started" = true ]; then
        echo -e "${YELLOW}⏳ Waiting for Docker to be ready (this may take 30-60 seconds)...${NC}"
        local max_wait=90
        local waited=0
        while [ $waited -lt $max_wait ]; do
            if docker info &> /dev/null; then
                echo -e "${GREEN}✅ Docker started successfully${NC}"
                return 0
            fi
            echo -n "."
            sleep 2
            waited=$((waited + 2))
        done
        echo ""
        echo -e "${RED}❌ Docker failed to start within ${max_wait} seconds${NC}"
        echo -e "${YELLOW}   Please start Docker Desktop manually and try again${NC}"
        exit 1
    else
        echo -e "${RED}❌ Could not start Docker automatically${NC}"
        echo -e "${YELLOW}   Please start Docker Desktop manually and try again${NC}"
        exit 1
    fi
}

# Check if tmux is installed
check_tmux() {
    if ! command -v tmux &> /dev/null; then
        echo -e "${YELLOW}⚠️  tmux not installed, installing...${NC}"
        # Try to install tmux (platform dependent)
        if command -v apt-get &> /dev/null; then
            sudo apt-get update && sudo apt-get install -y tmux
        elif command -v yum &> /dev/null; then
            sudo yum install -y tmux
        elif command -v brew &> /dev/null; then
            brew install tmux
        else
            echo -e "${RED}❌ Cannot auto-install tmux. Please install manually:${NC}"
            echo -e "   Ubuntu/Debian: sudo apt-get install tmux"
            echo -e "   CentOS/RHEL: sudo yum install tmux"
            echo -e "   macOS: brew install tmux"
            exit 1
        fi
        echo -e "${GREEN}✅ tmux installed successfully${NC}"
    else
        echo -e "${GREEN}✅ tmux is already installed${NC}"
    fi
}

# Clean up duplicate windows in tmux session
cleanup_duplicate_windows() {
    local session_name=$(get_tmux_session)
    
    if ! tmux has-session -t "$session_name" 2>/dev/null; then
        return 0
    fi
    
    # List of windows that should only exist once
    local windows_to_check=("base-services" "web-apps" "web-dashboard" "web-games" "profile" "gate" "match" "chat" "auth" "history" "baccarat" "holdem")
    
    for window_name in "${windows_to_check[@]}"; do
        # Find all windows with this name
        local window_indices=$(tmux list-windows -t "$session_name" -F "#{window_index}:#{window_name}" 2>/dev/null | grep ":${window_name}$" | cut -d':' -f1)
        
        if [ -n "$window_indices" ]; then
            # Count how many windows with this name exist
            local count=$(echo "$window_indices" | wc -l)
            
            if [ "$count" -gt 1 ]; then
                echo -e "${YELLOW}⚠️  Found $count windows named '$window_name', cleaning up duplicates...${NC}"
                # Keep the first one, kill the rest
                local first=true
                for win_idx in $window_indices; do
                    if [ "$first" = true ]; then
                        first=false
                        continue
                    fi
                    echo -e "${BLUE}   Removing duplicate window: $win_idx${NC}"
                    tmux kill-window -t "$session_name:$win_idx" 2>/dev/null || true
                done
            fi
        fi
    done
}

# Setup tmux session
setup_tmux_session() {
    local session_name="game-link-dev"
    
    # remain-on-exit: Ctrl+C 停止執行緒時，pane 不消失，可再執行 npm run dev 等重新啟動
    tmux set -g remain-on-exit on 2>/dev/null || true
    echo -e "${BLUE}✓ remain-on-exit enabled (Ctrl+C 只停止執行緒，pane 保留)${NC}"
    
    # Enable mouse mode for tmux (for mouse scrolling support)
    # Check tmux version to use correct syntax
    local tmux_version=$(tmux -V 2>/dev/null | sed 's/[^0-9.]//g' | cut -d'.' -f1-2)
    if [ -n "$tmux_version" ]; then
        # For tmux 2.1+, use: set -g mouse on
        # For tmux < 2.1, use: set -g mode-mouse on
        local major_version=$(echo "$tmux_version" | cut -d'.' -f1)
        local minor_version=$(echo "$tmux_version" | cut -d'.' -f2)
        
        if [ "$major_version" -ge 3 ] || ([ "$major_version" -eq 2 ] && [ "${minor_version:-0}" -ge 1 ]); then
            # tmux 2.1+
            tmux set -g mouse on 2>/dev/null || true
            echo -e "${BLUE}✓ Mouse mode enabled (tmux 2.1+)${NC}"
        else
            # tmux < 2.1
            tmux set -g mode-mouse on 2>/dev/null || true
            tmux set -g mouse-resize-pane on 2>/dev/null || true
            tmux set -g mouse-select-pane on 2>/dev/null || true
            tmux set -g mouse-select-window on 2>/dev/null || true
            echo -e "${BLUE}✓ Mouse mode enabled (tmux < 2.1)${NC}"
        fi
    else
        # Try modern syntax first, fallback to old syntax
        tmux set -g mouse on 2>/dev/null || {
            tmux set -g mode-mouse on 2>/dev/null || true
            tmux set -g mouse-resize-pane on 2>/dev/null || true
            tmux set -g mouse-select-pane on 2>/dev/null || true
            tmux set -g mouse-select-window on 2>/dev/null || true
        }
        echo -e "${BLUE}✓ Mouse mode enabled${NC}"
    fi
    
    # Load project tmux config if it exists
    local tmux_config="$SCRIPT_DIR/.tmux.conf"
    if [ -f "$tmux_config" ]; then
        echo -e "${BLUE}Loading tmux configuration from project...${NC}"
        tmux source-file "$tmux_config" 2>/dev/null || true
    fi
    
    # Check if session already exists
    if tmux has-session -t "$session_name" 2>/dev/null; then
        echo -e "${YELLOW}⚠️  tmux session '$session_name' already exists${NC}"
        echo -e "${BLUE}Connecting to existing session...${NC}"
        echo -e "${YELLOW}To attach later, run: tmux attach -t $session_name${NC}"
        echo ""
        
        # Clean up any duplicate windows
        cleanup_duplicate_windows
        
        # Reload config for existing session
        if [ -f "$tmux_config" ]; then
            tmux source-file "$tmux_config" 2>/dev/null || true
        fi
        
        # Ensure mouse mode and remain-on-exit for existing session
        tmux set -g mouse on 2>/dev/null || true
        tmux set -g remain-on-exit on 2>/dev/null || true
        
        return 0
    fi
    
    # Create new tmux session (detached)
    tmux new-session -d -s "$session_name" -n "main"
    
    # Load config for new session
    if [ -f "$tmux_config" ]; then
        tmux source-file "$tmux_config" 2>/dev/null || true
    fi
    
    # Enable mouse mode and remain-on-exit for the new session
    tmux set -g mouse on 2>/dev/null || true
    tmux set -g remain-on-exit on 2>/dev/null || true
    
    echo -e "${GREEN}✅ Created tmux session: $session_name${NC}"
    echo -e "${GREEN}✅ Mouse mode enabled for scrolling${NC}"
    echo -e "${YELLOW}To attach later, run: tmux attach -t $session_name${NC}"
    echo ""
}

# Get tmux session name
get_tmux_session() {
    echo "game-link-dev"
}

# Open new terminal window with tmux session
open_tmux_in_new_terminal() {
    local session_name=$(get_tmux_session)
    
    # Detect available terminal emulator
    if command -v gnome-terminal &> /dev/null; then
        # GNOME Terminal (Linux)
        echo -e "${BLUE}📺 Opening new GNOME Terminal window...${NC}"
        gnome-terminal -- bash -c "tmux attach -t $session_name || tmux new -s $session_name" 2>/dev/null &
    elif command -v xterm &> /dev/null; then
        # xterm (Linux/Unix)
        echo -e "${BLUE}📺 Opening new xterm window...${NC}"
        xterm -e "tmux attach -t $session_name || tmux new -s $session_name" 2>/dev/null &
    elif command -v konsole &> /dev/null; then
        # Konsole (KDE)
        echo -e "${BLUE}📺 Opening new Konsole window...${NC}"
        konsole -e bash -c "tmux attach -t $session_name || tmux new -s $session_name" 2>/dev/null &
    elif command -v terminator &> /dev/null; then
        # Terminator
        echo -e "${BLUE}📺 Opening new Terminator window...${NC}"
        terminator -e "tmux attach -t $session_name || tmux new -s $session_name" 2>/dev/null &
    elif [ -n "$WSL_DISTRO_NAME" ] || [ -n "$WSLENV" ]; then
        # WSL environment - try to use Windows Terminal or default terminal
        echo -e "${BLUE}📺 Opening new terminal window (WSL)...${NC}"
        
        # Try to find Windows Terminal (wt.exe)
        local wt_path=""
        if [ -f "/mnt/c/Users/$USER/AppData/Local/Microsoft/WindowsApps/wt.exe" ]; then
            wt_path="/mnt/c/Users/$USER/AppData/Local/Microsoft/WindowsApps/wt.exe"
        elif [ -f "/mnt/c/Program Files/WindowsApps/Microsoft.WindowsTerminal_*/wt.exe" ]; then
            wt_path=$(find /mnt/c/Program\ Files/WindowsApps/Microsoft.WindowsTerminal_* -name "wt.exe" 2>/dev/null | head -n 1)
        fi
        
        if [ -n "$wt_path" ]; then
            # Use Windows Terminal
            # Get WSL distro name, default to current if not set
            local distro_name="${WSL_DISTRO_NAME:-$(cat /etc/os-release 2>/dev/null | grep '^ID=' | cut -d'=' -f2 | tr -d '"' || echo 'Ubuntu')}"
            "$wt_path" wsl -d "$distro_name" bash -c "tmux attach -t $session_name || tmux new -s $session_name" 2>/dev/null &
        elif command -v cmd.exe &> /dev/null || [ -f /mnt/c/Windows/System32/cmd.exe ]; then
            # Fallback: use cmd.exe to open default terminal
            /mnt/c/Windows/System32/cmd.exe /c start "WSL" wsl bash -c "tmux attach -t $session_name || tmux new -s $session_name" 2>/dev/null &
        else
            # Last resort: try to use PowerShell
            if command -v powershell.exe &> /dev/null || [ -f /mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe ]; then
                /mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe -Command "Start-Process wsl -ArgumentList 'bash', '-c', 'tmux attach -t $session_name || tmux new -s $session_name'" 2>/dev/null &
            else
                echo -e "${YELLOW}⚠️  Cannot auto-open new terminal in WSL.${NC}"
                echo -e "${YELLOW}   Please manually run: ${BLUE}tmux attach -t $session_name${NC}"
                return 1
            fi
        fi
    elif [ "$(uname)" = "Darwin" ]; then
        # macOS - use Terminal.app or iTerm2
        if command -v osascript &> /dev/null; then
            echo -e "${BLUE}📺 Opening new Terminal window (macOS)...${NC}"
            osascript -e "tell application \"Terminal\" to do script \"tmux attach -t $session_name || tmux new -s $session_name\"" 2>/dev/null &
        elif command -v open &> /dev/null; then
            # Try iTerm2 if available
            if [ -d "/Applications/iTerm.app" ]; then
                open -a iTerm
                sleep 1
                osascript -e "tell application \"iTerm\" to tell current window to tell current session to write text \"tmux attach -t $session_name || tmux new -s $session_name\"" 2>/dev/null &
            fi
        fi
    else
        # Unknown system, just print instructions
        echo -e "${YELLOW}⚠️  Cannot auto-detect terminal emulator.${NC}"
        echo -e "${YELLOW}   Please manually run: ${BLUE}tmux attach -t $session_name${NC}"
        return 1
    fi
    
    sleep 2
    echo -e "${GREEN}✅ New terminal window opened${NC}"
    return 0
}

# Check if infrastructure is already running
check_infrastructure_running() {
    local etcd_running=$(docker ps --filter "name=game-link-etcd" --filter "status=running" --format "{{.Names}}" | wc -l)
    local redis_running=$(docker ps --filter "name=game-link-redis" --filter "status=running" --format "{{.Names}}" | wc -l)
    local metadata_postgres_running=$(docker ps --filter "name=metadata-postgres" --filter "status=running" --format "{{.Names}}" | wc -l)
    local history_postgres_running=$(docker ps --filter "name=history-postgres" --filter "status=running" --format "{{.Names}}" | wc -l)
    
    if [ "$etcd_running" -eq 1 ] && [ "$redis_running" -eq 1 ] && [ "$metadata_postgres_running" -eq 1 ] && [ "$history_postgres_running" -eq 1 ]; then
        return 0  # All running
    else
        return 1  # Not all running
    fi
}

# Start infrastructure
start_infrastructure() {
    local was_running=false
    
    # Check if infrastructure is already running
    if check_infrastructure_running; then
        echo -e "${GREEN}✅ Infrastructure already running, skipping startup...${NC}"
        echo -e "${YELLOW}⏳ Verifying infrastructure health...${NC}"
        was_running=true
    else
        echo -e "${BLUE}📦 Starting infrastructure (etcd, redis, metadata-postgres, history-postgres)...${NC}"
        # Try to start infrastructure, handle network recreation error（含 history 用 DB）
        local output=$(docker compose up -d etcd redis metadata-postgres history-postgres 2>&1)
        local exit_code=$?
        
        if [ $exit_code -ne 0 ]; then
            # Check if error is about network recreation
            if echo "$output" | grep -q "needs to be recreated"; then
                echo -e "${YELLOW}⚠️  Network needs to be recreated, fixing...${NC}"
                docker compose down
                echo -e "${BLUE}📦 Restarting infrastructure...${NC}"
                docker compose up -d etcd redis metadata-postgres history-postgres
                if [ $? -ne 0 ]; then
                    echo -e "${RED}❌ Failed to start infrastructure after network recreation${NC}"
                    exit 1
                fi
            else
                echo "$output"
                echo -e "${RED}❌ Failed to start infrastructure${NC}"
                exit 1
            fi
        fi
        echo -e "${YELLOW}⏳ Waiting for infrastructure to be ready...${NC}"
    fi
    
    # Set timeout based on whether containers were already running
    local etcd_timeout=30
    local redis_timeout=30
    local metadata_postgres_timeout=60
    local history_postgres_timeout=60
    if [ "$was_running" = true ]; then
        # If already running, use shorter timeout (they should be ready quickly)
        etcd_timeout=5
        redis_timeout=5
        metadata_postgres_timeout=10
        history_postgres_timeout=10
    fi
    
    # Wait for etcd to be ready
    echo -n "Waiting for etcd..."
    for ((i=1; i<=$etcd_timeout; i++)); do
        if docker exec game-link-etcd etcdctl endpoint health &> /dev/null; then
            echo -e " ${GREEN}✅${NC}"
            break
        fi
        if [ $i -eq $etcd_timeout ]; then
            echo -e " ${RED}❌ Timeout${NC}"
            exit 1
        fi
        echo -n "."
        sleep 1
    done
    
    # Wait for redis to be ready
    echo -n "Waiting for redis..."
    for ((i=1; i<=$redis_timeout; i++)); do
        if docker exec game-link-redis redis-cli ping &> /dev/null; then
            echo -e " ${GREEN}✅${NC}"
            break
        fi
        if [ $i -eq $redis_timeout ]; then
            echo -e " ${RED}❌ Timeout${NC}"
            exit 1
        fi
        echo -n "."
        sleep 1
    done
    
    # Wait for metadata postgres to be ready
    echo -n "Waiting for metadata postgres..."
    for ((i=1; i<=$metadata_postgres_timeout; i++)); do
        if docker exec metadata-postgres pg_isready -U metadata -d metadata &> /dev/null; then
            echo -e " ${GREEN}✅${NC}"
            break
        fi
        if [ $i -eq $metadata_postgres_timeout ]; then
            echo -e " ${RED}❌ Timeout${NC}"
            exit 1
        fi
        echo -n "."
        sleep 1
    done
    
    # Wait for history postgres to be ready
    echo -n "Waiting for history postgres..."
    for ((i=1; i<=$history_postgres_timeout; i++)); do
        if docker exec history-postgres pg_isready -U gamehistory -d gamehistory &> /dev/null; then
            echo -e " ${GREEN}✅${NC}"
            break
        fi
        if [ $i -eq $history_postgres_timeout ]; then
            echo -e " ${RED}❌ Timeout${NC}"
            exit 1
        fi
        echo -n "."
        sleep 1
    done
    
    echo -e "${GREEN}✅ All infrastructure is ready${NC}"
}

# Get Air command path
get_air_cmd() {
    if command -v air &> /dev/null; then
        echo "air"
    else
        # Try to find air in GOPATH
        local go_path=$(go env GOPATH 2>/dev/null)
        if [ -n "$go_path" ] && [ -f "$go_path/bin/air" ]; then
            echo "$go_path/bin/air"
        else
            echo ""
        fi
    fi
}

# Check if Node.js and npm are installed
check_node() {
    if ! command -v node &> /dev/null; then
        echo -e "${YELLOW}⚠️  Node.js not found, web applications will be skipped${NC}"
        return 1
    fi
    if ! command -v npm &> /dev/null; then
        echo -e "${YELLOW}⚠️  npm not found, web applications will be skipped${NC}"
        return 1
    fi
    return 0
}

# Start service (using Air in tmux window)
start_service() {
    local service_name=$1
    local service_path="$SCRIPT_DIR/services/$service_name"
    local session_name=$(get_tmux_session)
    
    if [ ! -d "$service_path" ]; then
        echo -e "${RED}❌ Service directory does not exist: $service_path${NC}"
        return 1
    fi
    
    # Get Air command
    local air_cmd=$(get_air_cmd)
    if [ -z "$air_cmd" ]; then
        echo -e "${RED}❌ Air command not found. Please ensure Air is installed and in PATH${NC}"
        return 1
    fi
    
    echo -e "${BLUE}🚀 Starting service: $service_name${NC}"
    
    # Create new tmux window for this service
    # Use absolute path to ensure correct directory
    local abs_service_path=$(cd "$service_path" && pwd)
    
    # Set environment variables for local development
    # Redis address for gate service (and other services that use Redis)
    local env_vars=""
    if [ "$service_name" = "gate" ]; then
        # Use provided REDIS_ADDR or default to localhost:6379 for local dev
        env_vars="REDIS_ADDR=${REDIS_ADDR:-localhost:6379}"
    fi
    
    # Create tmux window and run Air in the service directory
    if [ -n "$env_vars" ]; then
        tmux new-window -t "$session_name" -n "$service_name" \
            "cd '$abs_service_path' && ($env_vars $air_cmd; exec bash)"
    else
        tmux new-window -t "$session_name" -n "$service_name" \
            "cd '$abs_service_path' && ($air_cmd; exec bash)"
    fi
    
    # Save window info
    echo "$service_name" > "/tmp/tmux_${service_name}.window"
    
    sleep 1
    echo -e "${GREEN}✅ $service_name started in tmux window: $service_name${NC}"
    echo -e "${BLUE}   View logs: tmux select-window -t $session_name:$service_name${NC}"
}

# Start base services in split window layout
# Layout: 
#   - Left side: gate, auth, history, web-games
#   - Right side: profile, match, chat, web-dashboard
start_base_services_split() {
    local session_name=$(get_tmux_session)
    local air_cmd=$(get_air_cmd)
    
    if [ -z "$air_cmd" ]; then
        echo -e "${RED}❌ Air command not found. Please ensure Air is installed and in PATH${NC}"
        return 1
    fi
    
    echo -e "${BLUE}🚀 Starting all services in split window layout...${NC}"
    echo -e "${BLUE}   Layout:${NC}"
    echo -e "${BLUE}     - Left side: gate, auth, history, web-games${NC}"
    echo -e "${BLUE}     - Right side: profile, match, chat, web-dashboard${NC}"
    
    # Get absolute paths for Go services
    local gate_path=$(cd "$SCRIPT_DIR/services/gate" && pwd)
    local auth_path=$(cd "$SCRIPT_DIR/services/auth" && pwd)
    local history_path=$(cd "$SCRIPT_DIR/services/history" && pwd)
    local profile_path=$(cd "$SCRIPT_DIR/services/profile" && pwd)
    local match_path=$(cd "$SCRIPT_DIR/services/match" && pwd)
    local chat_path=$(cd "$SCRIPT_DIR/services/chat" && pwd)
    
    # Get paths for web apps (if Node.js available)
    local web_games_path=""
    local web_dashboard_path=""
    if check_node; then
        web_games_path=$(cd "$SCRIPT_DIR/web-games" && pwd 2>/dev/null) || true
        web_dashboard_path=$(cd "$SCRIPT_DIR/web-dashboard" && pwd 2>/dev/null) || true
    fi
    
    # Check if base-services window already exists, and kill it if it does
    if tmux list-windows -t "$session_name" -F "#{window_name}" 2>/dev/null | grep -q "^base-services$"; then
        echo -e "${YELLOW}⚠️  Window 'base-services' already exists, removing it...${NC}"
        tmux kill-window -t "$session_name:base-services" 2>/dev/null || true
        sleep 2
        # Verify it was removed
        if tmux list-windows -t "$session_name" -F "#{window_name}" 2>/dev/null | grep -q "^base-services$"; then
            echo -e "${YELLOW}⚠️  Window still exists, trying force kill...${NC}"
            tmux kill-window -t "$session_name:base-services" 2>/dev/null || true
            sleep 1
        fi
    fi
    
    # Create new window named "base-services" and start gate in the main pane
    echo -e "${BLUE}Creating base-services window...${NC}"
    
    # First, let's check if session exists and is accessible
    if ! tmux has-session -t "$session_name" 2>/dev/null; then
        echo -e "${RED}❌ Session '$session_name' does not exist${NC}"
        return 1
    fi
    
    # Try to create the window - use simpler command first to test
    # The issue might be with the command execution, so let's create the window first, then run the command
    echo -e "${BLUE}   Creating empty window...${NC}"
    if ! tmux new-window -t "$session_name" -n "base-services" 2>&1; then
        echo -e "${RED}❌ Failed to create base-services window${NC}"
        return 1
    fi
    
    sleep 2
    
    # Verify window was created
    if ! tmux list-windows -t "$session_name" -F "#{window_name}" 2>/dev/null | grep -q "^base-services$"; then
        echo -e "${RED}❌ Window 'base-services' was not created successfully${NC}"
        echo -e "${YELLOW}   Available windows:${NC}"
        tmux list-windows -t "$session_name" -F "#{window_index}: #{window_name}" 2>/dev/null || true
        return 1
    fi
    
    # Get window index for more reliable targeting
    local window_index=$(tmux list-windows -t "$session_name" -F "#{window_index}:#{window_name}" 2>/dev/null | grep ":base-services$" | head -n1 | cut -d':' -f1)
    
    if [ -z "$window_index" ]; then
        echo -e "${YELLOW}⚠️  Could not get window index, trying with window name...${NC}"
        window_index="base-services"
    fi
    
    echo -e "${BLUE}   Window created successfully (index: $window_index)${NC}"
    
    # Now send the command to start gate service in the window
    echo -e "${BLUE}   Starting gate service in window...${NC}"
    tmux send-keys -t "$session_name:$window_index" "cd '$gate_path' && (REDIS_ADDR=${REDIS_ADDR:-localhost:6379} $air_cmd; exec bash)" C-m 2>&1 || true
    
    sleep 2
    
    # Select the window first to ensure it's active
    if ! tmux select-window -t "$session_name:$window_index" 2>&1; then
        echo -e "${RED}❌ Failed to select window $window_index${NC}"
        return 1
    fi
    
    sleep 1
    
    # Create layout with all services:
    # Target layout:
    #   Left side: gate, auth, history, web-games
    #   Right side: profile, match, chat, web-dashboard
    #
    # Build strategy:
    #   1. Gate is in pane 0 (full width, already started)
    #   2. Split horizontally: gate (left, pane 0), right area (right, pane 1)
    #   3. In right (pane 1): split vertically - profile (top), match (middle), chat (bottom)
    #   4. Capture chat_pane_id for Step 7 (pane indices change after left splits)
    #   5. In left (pane 0): split vertically - gate, auth, history
    #   6. Step 7: split chat pane to add web-dashboard (right bottom) - BEFORE left split
    #   7. Step 8: split history pane to add web-games (left bottom)
    
    # Step 1: Split horizontally to create left (gate) and right (for profile/match/chat) sides
    echo -e "${BLUE}Step 1: Splitting horizontally (left: gate, right: profile/match/chat)...${NC}"
    if ! tmux select-pane -t "$session_name:$window_index.0" 2>/dev/null; then
            echo -e "${RED}❌ Failed to select gate pane${NC}"
            return 1
        fi
        
        # Split horizontally: gate stays in pane 0 (left), creates pane 1 (right, empty)
        # Explicitly specify pane 0 to ensure we split from the correct pane
        if ! tmux split-window -t "$session_name:$window_index.0" -h 2>&1; then
            echo -e "${RED}❌ Failed to split horizontally${NC}"
            return 1
        fi
        sleep 2
        
        # Step 2: In the right side (pane 1), start profile (top of right column)
        echo -e "${BLUE}Step 2: Starting profile in right side (top)...${NC}"
        if ! tmux select-pane -t "$session_name:$window_index.1" 2>/dev/null; then
            echo -e "${RED}❌ Failed to select right pane${NC}"
            return 1
        fi
        tmux send-keys -t "$session_name:$window_index.1" "cd '$profile_path' && ($air_cmd; exec bash)" C-m 2>&1 || true
        sleep 2
        
        # Step 3: In the right side (pane 1, profile), split vertically for match (middle)
        echo -e "${BLUE}Step 3: Splitting right side vertically for match (middle)...${NC}"
        if ! tmux select-pane -t "$session_name:$window_index.1" 2>/dev/null; then
            echo -e "${YELLOW}⚠️  Warning: Could not select profile pane, trying navigation...${NC}"
            tmux select-pane -t "$session_name:$window_index" -R 2>/dev/null || true
        fi
        
        # Split vertically: profile (pane 1, top), creates pane 2 (bottom, will become match)
        # Explicitly specify pane 1 to ensure we split from the correct pane
        if ! tmux split-window -t "$session_name:$window_index.1" -v \
            "cd '$match_path' && ($air_cmd; exec bash)" 2>&1; then
            echo -e "${RED}❌ Failed to split for match service${NC}"
            return 1
        fi
        sleep 2
        
        # Step 4: In the right side, split vertically again for chat (bottom)
        # After previous split: pane 1 = profile (top), pane 2 = match (bottom)
        # We need to split pane 2 (match) vertically to create chat
        echo -e "${BLUE}Step 4: Splitting right side vertically for chat (bottom)...${NC}"
        # Select pane 2 (match)
        if ! tmux select-pane -t "$session_name:$window_index.2" 2>/dev/null; then
            echo -e "${YELLOW}⚠️  Warning: Could not select match pane, trying navigation...${NC}"
            # Navigate: select profile (pane 1), go down to match (pane 2)
            if tmux select-pane -t "$session_name:$window_index.1" 2>/dev/null; then
                tmux select-pane -t "$session_name:$window_index" -D 2>/dev/null || true
            fi
        fi
        
        # Split vertically: match (pane 2, middle), creates pane 3 (bottom, will become chat)
        # Explicitly specify pane 2 to ensure we split from the correct pane
        local panes_before_step4=$(mktemp)
        tmux list-panes -t "$session_name:$window_index" -F "#{pane_id}" | sort > "$panes_before_step4"
        if ! tmux split-window -t "$session_name:$window_index.2" -v \
            "cd '$chat_path' && ($air_cmd; exec bash)" 2>&1; then
            rm -f "$panes_before_step4"
            echo -e "${RED}❌ Failed to split for chat${NC}"
            return 1
        fi
        sleep 2
        # Capture chat pane ID for Step 8 (pane indices may change after left-side splits)
        local panes_after_step4=$(mktemp)
        tmux list-panes -t "$session_name:$window_index" -F "#{pane_id}" | sort > "$panes_after_step4"
        local chat_pane_id=$(comm -13 "$panes_before_step4" "$panes_after_step4" | head -1)
        rm -f "$panes_before_step4" "$panes_after_step4"
        
        # Step 5: In the left side (pane 0, gate), split vertically for auth (middle)
        echo -e "${BLUE}Step 5: Splitting left side vertically for auth (middle)...${NC}"
        # Debug: List all panes before splitting
        echo -e "${BLUE}   Current panes before Step 5:${NC}"
        tmux list-panes -t "$session_name:$window_index" -F "#{pane_index}: #{pane_title}" 2>/dev/null || true
        
        # Select left side (pane 0, gate) first to ensure it's active
        if ! tmux select-pane -t "$session_name:$window_index.0" 2>/dev/null; then
            echo -e "${YELLOW}⚠️  Warning: Could not select gate pane (pane 0), trying navigation...${NC}"
            # Try to navigate to pane 0 by going left from current pane
            tmux select-pane -t "$session_name:$window_index" -L 2>/dev/null || true
            # Try again after navigation
            if ! tmux select-pane -t "$session_name:$window_index.0" 2>/dev/null; then
                echo -e "${RED}❌ Failed to select pane 0 even after navigation${NC}"
                echo -e "${YELLOW}   Available panes:${NC}"
                tmux list-panes -t "$session_name:$window_index" -F "#{pane_index}" 2>/dev/null || true
                return 1
            fi
        fi
        
        echo -e "${BLUE}   Selected pane 0, splitting vertically for chat...${NC}"
        # Split vertically: gate (pane 0, top), creates new pane (middle, will become chat)
        # Explicitly specify pane 0 to ensure we split from the correct pane
        # Get current pane count and IDs before splitting
        local pane_count_before=$(tmux list-panes -t "$session_name:$window_index" -F "#{pane_index}" | wc -l | tr -d ' ')
        local panes_before_file=$(mktemp)
        tmux list-panes -t "$session_name:$window_index" -F "#{pane_id}" | sort > "$panes_before_file"
        echo -e "${BLUE}   Panes before split: $pane_count_before${NC}"
        
        if ! tmux split-window -t "$session_name:$window_index.0" -v \
            "cd '$auth_path' && ($air_cmd; exec bash)" 2>&1; then
            echo -e "${RED}❌ Failed to split for auth service${NC}"
            echo -e "${YELLOW}   Debug: session=$session_name, window_index=$window_index, pane=0${NC}"
            rm -f "$panes_before_file"
            return 1
        fi
        sleep 2
        
        # Find the newly created pane by comparing pane IDs
        local panes_after_file=$(mktemp)
        tmux list-panes -t "$session_name:$window_index" -F "#{pane_id}" | sort > "$panes_after_file"
        local auth_pane_id=$(comm -13 "$panes_before_file" "$panes_after_file" | head -1)
        rm -f "$panes_before_file" "$panes_after_file"
        
        if [ -z "$auth_pane_id" ]; then
            echo -e "${RED}❌ Could not find newly created auth pane${NC}"
            return 1
        fi
        
        # Get pane index from pane ID
        local auth_pane_index=$(tmux list-panes -t "$session_name:$window_index" -F "#{pane_index}:#{pane_id}" | grep ":$auth_pane_id$" | cut -d':' -f1)
        echo -e "${BLUE}   Created new pane: $auth_pane_index (ID: $auth_pane_id), auth service started${NC}"
        
        # Debug: List all panes after splitting
        echo -e "${BLUE}   Panes after Step 5:${NC}"
        tmux list-panes -t "$session_name:$window_index" -F "#{pane_index}: #{pane_title}" 2>/dev/null || true
        
        # Step 6: In the left side, split auth pane vertically for history (bottom)
        echo -e "${BLUE}Step 6: Splitting left side vertically for history (bottom)...${NC}"
        
        # Select auth pane to split from
        if ! tmux select-pane -t "$auth_pane_id" 2>/dev/null; then
            echo -e "${YELLOW}⚠️  Warning: Could not select auth pane, trying by index...${NC}"
            if ! tmux select-pane -t "$session_name:$window_index.$auth_pane_index" 2>/dev/null; then
                echo -e "${RED}❌ Failed to select auth pane for splitting${NC}"
                return 1
            fi
        fi
        
        echo -e "${BLUE}   Selected auth pane, splitting vertically for history...${NC}"
        # Get current pane count and IDs before splitting
        pane_count_before=$(tmux list-panes -t "$session_name:$window_index" -F "#{pane_index}" | wc -l | tr -d ' ')
        panes_before_file=$(mktemp)
        tmux list-panes -t "$session_name:$window_index" -F "#{pane_id}" | sort > "$panes_before_file"
        echo -e "${BLUE}   Panes before split: $pane_count_before${NC}"
        
        if ! tmux split-window -t "$auth_pane_id" -v 2>&1; then
            echo -e "${RED}❌ Failed to split for history${NC}"
            echo -e "${YELLOW}   Debug: session=$session_name, window_index=$window_index, pane=$auth_pane_id${NC}"
            rm -f "$panes_before_file"
            return 1
        fi
        sleep 1
        
        # Find the newly created pane by comparing pane IDs
        panes_after_file=$(mktemp)
        tmux list-panes -t "$session_name:$window_index" -F "#{pane_id}" | sort > "$panes_after_file"
        local new_pane_id=$(comm -13 "$panes_before_file" "$panes_after_file" | head -1)
        rm -f "$panes_before_file" "$panes_after_file"
        
        if [ -z "$new_pane_id" ]; then
            echo -e "${RED}❌ Could not find newly created pane${NC}"
            return 1
        fi
        
        # Get pane index from pane ID
        local new_pane_index=$(tmux list-panes -t "$session_name:$window_index" -F "#{pane_index}:#{pane_id}" | grep ":$new_pane_id$" | cut -d':' -f1)
        echo -e "${BLUE}   Created new pane: $new_pane_index (ID: $new_pane_id), starting history...${NC}"
        
        # Select the new pane first to ensure it's active
        tmux select-pane -t "$new_pane_id" 2>/dev/null || true
        sleep 0.5
        
        # Clear any existing content in the pane
        tmux send-keys -t "$new_pane_id" C-l 2>/dev/null || true
        sleep 0.3
        
        # Send command to the new pane - start history service with Air
        echo -e "${BLUE}   Sending command to pane $new_pane_index...${NC}"
        tmux send-keys -t "$new_pane_id" "cd '$history_path' && ($air_cmd; exec bash)" C-m 2>&1 || true
        sleep 3
        
        # Verify the command is running
        local pane_cmd=$(tmux list-panes -t "$new_pane_id" -F "#{pane_current_command}" 2>/dev/null)
        echo -e "${BLUE}   Pane $new_pane_index current command: $pane_cmd${NC}"
        
        # Debug: List all panes after splitting
        echo -e "${BLUE}   Panes after Step 6:${NC}"
        tmux list-panes -t "$session_name:$window_index" -F "#{pane_index}: #{pane_title}" 2>/dev/null || true
        
        # Step 7: Split chat pane (right bottom) to add web-dashboard - DO BEFORE Step 8
        # Right after Step 6: pane 3 = chat. Use pane_id (stable) or pane index 3.
        if [ -n "$web_dashboard_path" ] && [ -d "$web_dashboard_path" ]; then
            echo -e "${BLUE}Step 7: Splitting right side for web-dashboard (bottom)...${NC}"
            local dash_ok=false
            if [ -n "$chat_pane_id" ]; then
                if tmux split-window -t "$chat_pane_id" -v "cd '$web_dashboard_path' && (npm run dev; exec bash)" 2>&1; then
                    dash_ok=true
                fi
            fi
            if [ "$dash_ok" = false ]; then
                if tmux split-window -t "$session_name:$window_index.3" -v "cd '$web_dashboard_path' && (npm run dev; exec bash)" 2>&1; then
                    dash_ok=true
                fi
            fi
            if [ "$dash_ok" = true ]; then
                sleep 2
                echo -e "${BLUE}   web-dashboard started (right bottom)${NC}"
            else
                echo -e "${YELLOW}⚠️  Failed to add web-dashboard pane${NC}"
            fi
        else
            echo -e "${YELLOW}⚠️  Skipping web-dashboard (Node.js or web-dashboard dir not found)${NC}"
        fi
        
        # Step 8: Split history pane (left bottom) to add web-games
        if [ -n "$web_games_path" ] && [ -d "$web_games_path" ]; then
            echo -e "${BLUE}Step 8: Splitting left side for web-games (bottom)...${NC}"
            if tmux split-window -t "$new_pane_id" -v "cd '$web_games_path' && (npm run dev; exec bash)" 2>&1; then
                sleep 2
                echo -e "${BLUE}   web-games started (left bottom)${NC}"
            else
                echo -e "${YELLOW}⚠️  Failed to add web-games pane${NC}"
            fi
        else
            echo -e "${YELLOW}⚠️  Skipping web-games (Node.js or web-games dir not found)${NC}"
        fi
    
    # Verify window exists before trying to resize
    if ! tmux list-windows -t "$session_name" -F "#{window_name}" 2>/dev/null | grep -q "^base-services$"; then
        echo -e "${RED}❌ Window 'base-services' not found after creation${NC}"
        return 1
    fi
    
    # Adjust pane sizes for better layout
    echo -e "${BLUE}Adjusting pane sizes for optimal layout...${NC}"
    local window_height=$(tmux display-message -t "$session_name:$window_index" -p "#{window_height}" 2>/dev/null || echo "24")
    
    # Adjust pane sizes for better layout
    # Layout: 4 panes per side, each ~25% height
    #   Left: gate, auth, history, web-games
    #   Right: profile, match, chat, web-dashboard
    
    if [ -n "$window_height" ] && [ "$window_height" -gt 15 ]; then
        echo -e "${BLUE}Adjusting pane sizes for optimal layout...${NC}"
        
        # Left side: gate (pane 0) ~25% height
        local left_row_height=$((window_height / 4))
        if tmux select-pane -t "$session_name:$window_index.0" 2>/dev/null; then
            tmux resize-pane -t "$session_name:$window_index.0" -y "$left_row_height" 2>/dev/null || true
        fi
        
        # Right side: profile (pane 1) ~25% height
        if tmux select-pane -t "$session_name:$window_index.1" 2>/dev/null; then
            tmux resize-pane -t "$session_name:$window_index.1" -y "$left_row_height" 2>/dev/null || true
        fi
        
        # Match (pane 2) ~25% height
        if tmux select-pane -t "$session_name:$window_index.2" 2>/dev/null; then
            local match_height=$((window_height * 2 / 4 / 2))
            tmux resize-pane -t "$session_name:$window_index.2" -y "$match_height" 2>/dev/null || true
        fi
    fi
    
    # Select top pane (gate) to make it active
    tmux select-pane -t "$session_name:$window_index.0" 2>/dev/null || true
    
    echo -e "${GREEN}✅ All services started in split window layout${NC}"
    echo -e "${BLUE}   Window: base-services${NC}"
    echo -e "${BLUE}   Left side: gate, auth, history, web-games${NC}"
    echo -e "${BLUE}   Right side: profile, match, chat, web-dashboard${NC}"
    echo -e "${BLUE}   View: tmux select-window -t $session_name:base-services${NC}"
}

# Start web applications in split window layout
# Layout: web-games on left, web-dashboard on right (two panes side by side)
start_web_apps_split() {
    local session_name=$(get_tmux_session)
    
    # Check if Node.js is available
    if ! check_node; then
        echo -e "${YELLOW}⚠️  Node.js not found, skipping web applications${NC}"
        return 1
    fi
    
    echo -e "${BLUE}🚀 Starting web applications in split window layout...${NC}"
    echo -e "${BLUE}   Layout: web-games (left), web-dashboard (right)${NC}"
    
    # Get absolute paths
    local web_games_path=$(cd "$SCRIPT_DIR/web-games" && pwd)
    local web_dashboard_path=$(cd "$SCRIPT_DIR/web-dashboard" && pwd)
    
    # Check if web-apps window already exists, and kill it if it does
    if tmux list-windows -t "$session_name" -F "#{window_name}" 2>/dev/null | grep -q "^web-apps$"; then
        echo -e "${YELLOW}⚠️  Window 'web-apps' already exists, removing it...${NC}"
        tmux kill-window -t "$session_name:web-apps" 2>/dev/null || true
        sleep 2
        # Verify it was removed
        if tmux list-windows -t "$session_name" -F "#{window_name}" 2>/dev/null | grep -q "^web-apps$"; then
            echo -e "${YELLOW}⚠️  Window still exists, trying force kill...${NC}"
            tmux kill-window -t "$session_name:web-apps" 2>/dev/null || true
            sleep 1
        fi
    fi
    
    # Also clean up individual windows if they exist
    if tmux list-windows -t "$session_name" -F "#{window_name}" 2>/dev/null | grep -q "^web-dashboard$"; then
        tmux kill-window -t "$session_name:web-dashboard" 2>/dev/null || true
    fi
    if tmux list-windows -t "$session_name" -F "#{window_name}" 2>/dev/null | grep -q "^web-games$"; then
        tmux kill-window -t "$session_name:web-games" 2>/dev/null || true
    fi
    sleep 1
    
    # Create new window named "web-apps" and start web-games in the main pane
    echo -e "${BLUE}Creating web-apps window...${NC}"
    echo -e "${BLUE}   Creating empty window...${NC}"
    if ! tmux new-window -t "$session_name" -n "web-apps" 2>&1; then
        echo -e "${RED}❌ Failed to create web-apps window${NC}"
        return 1
    fi
    
    sleep 2
    
    # Verify window was created
    if ! tmux list-windows -t "$session_name" -F "#{window_name}" 2>/dev/null | grep -q "^web-apps$"; then
        echo -e "${RED}❌ Window 'web-apps' was not created successfully${NC}"
        echo -e "${YELLOW}   Available windows:${NC}"
        tmux list-windows -t "$session_name" -F "#{window_index}: #{window_name}" 2>/dev/null || true
        return 1
    fi
    
    # Get window index for more reliable targeting
    local window_index=$(tmux list-windows -t "$session_name" -F "#{window_index}:#{window_name}" 2>/dev/null | grep ":web-apps$" | head -n1 | cut -d':' -f1)
    
    if [ -z "$window_index" ]; then
        echo -e "${YELLOW}⚠️  Could not get window index, trying with window name...${NC}"
        window_index="web-apps"
    fi
    
    echo -e "${BLUE}   Window created successfully (index: $window_index)${NC}"
    
    # Now send the command to start web-games in the window
    echo -e "${BLUE}   Starting web-games in window...${NC}"
    tmux send-keys -t "$session_name:$window_index" "cd '$web_games_path' && (npm run dev; exec bash)" C-m 2>&1 || true
    
    sleep 2
    
    # Select the window first to ensure it's active
    if ! tmux select-window -t "$session_name:$window_index" 2>&1; then
        echo -e "${RED}❌ Failed to select window $window_index${NC}"
        return 1
    fi
    
    sleep 1
    
    # Split window horizontally: create right pane for web-dashboard (left: web-games, right: web-dashboard)
    # Use -h for horizontal split: left 50%, right 50%
    echo -e "${BLUE}Splitting window for web-dashboard (left: web-games, right: web-dashboard)...${NC}"
    if ! tmux split-window -t "$session_name:$window_index" -h \
        "cd '$web_dashboard_path' && (npm run dev; exec bash)" 2>&1; then
        echo -e "${RED}❌ Failed to split window for web-dashboard${NC}"
        echo -e "${YELLOW}   Debug: session=$session_name, window_index=$window_index${NC}"
        echo -e "${YELLOW}   Listing windows:${NC}"
        tmux list-windows -t "$session_name" -F "#{window_index}: #{window_name}" 2>/dev/null || true
        return 1
    fi
    
    sleep 2
    
    # Final layout:
    # - Pane 0: web-games (left, 50% width)
    # - Pane 1: web-dashboard (right, 50% width)
    
    # Verify window exists before trying to resize
    if ! tmux list-windows -t "$session_name" -F "#{window_name}" 2>/dev/null | grep -q "^web-apps$"; then
        echo -e "${RED}❌ Window 'web-apps' not found after creation${NC}"
        return 1
    fi
    
    # Select left pane (web-games) to make it active
    if tmux select-pane -t "$session_name:$window_index.0" 2>/dev/null; then
        # Optionally resize panes if needed (they should be 50/50 by default)
        echo -e "${BLUE}   Layout set: web-games (left), web-dashboard (right)${NC}"
        tmux select-pane -t "$session_name:$window_index.0" 2>/dev/null || true
    else
        echo -e "${YELLOW}⚠️  Warning: Could not select pane 0 for web-apps${NC}"
    fi
    
    echo -e "${GREEN}✅ Web applications started in split window layout${NC}"
    echo -e "${BLUE}   Window: web-apps${NC}"
    echo -e "${BLUE}   - Left pane: web-games (npm run dev)${NC}"
    echo -e "${BLUE}   - Right pane: web-dashboard (npm run dev)${NC}"
    echo -e "${BLUE}   View: tmux select-window -t $session_name:web-apps${NC}"
}

# Start web application (using npm in tmux window) - DEPRECATED, use start_web_apps_split instead
start_web_app() {
    local app_name=$1
    local app_path="$SCRIPT_DIR/$app_name"
    local session_name=$(get_tmux_session)
    
    if [ ! -d "$app_path" ]; then
        echo -e "${RED}❌ Web app directory does not exist: $app_path${NC}"
        return 1
    fi
    
    if [ ! -f "$app_path/package.json" ]; then
        echo -e "${RED}❌ package.json not found in: $app_path${NC}"
        return 1
    fi
    
    echo -e "${BLUE}🚀 Starting web app: $app_name${NC}"
    
    # Check if window already exists, and kill it if it does
    if tmux list-windows -t "$session_name" -F "#{window_name}" 2>/dev/null | grep -q "^${app_name}$"; then
        echo -e "${YELLOW}⚠️  Window '$app_name' already exists, removing it...${NC}"
        tmux kill-window -t "$session_name:$app_name" 2>/dev/null || true
        sleep 1
    fi
    
    # Create new tmux window for this web app
    # Use absolute path to ensure correct directory
    local abs_app_path=$(cd "$app_path" && pwd)
    
    # Create tmux window and run npm dev in the app directory
    if ! tmux new-window -t "$session_name" -n "$app_name" \
        "cd '$abs_app_path' && (npm run dev; exec bash)" 2>/dev/null; then
        echo -e "${RED}❌ Failed to create window for $app_name${NC}"
        return 1
    fi
    
    # Save window info
    echo "$app_name" > "/tmp/tmux_${app_name}.window"
    
    sleep 1
    echo -e "${GREEN}✅ $app_name started in tmux window: $app_name${NC}"
    echo -e "${BLUE}   View logs: tmux select-window -t $session_name:$app_name${NC}"
}

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}🛑 Stopping services...${NC}"
    
    local session_name=$(get_tmux_session)
    
    # Stop services in tmux windows
    # Handle base-services split window specially
    if tmux list-windows -t "$session_name" -F "#{window_name}" 2>/dev/null | grep -q "^base-services$"; then
        echo -e "${BLUE}Stopping all services in base-services window (gate, auth, history, profile, match, chat)...${NC}"
        # Send Ctrl+C to all panes in the base-services window
        tmux list-panes -t "$session_name:base-services" -F "#{pane_id}" | while read pane_id; do
            tmux send-keys -t "$pane_id" C-c 2>/dev/null || true
        done
        sleep 2
        tmux kill-window -t "$session_name:base-services" 2>/dev/null || true
    fi
    
    # Web apps are now in base-services window, so they will be handled above
    # But check for standalone web-apps window in case it exists from previous runs
    if tmux list-windows -t "$session_name" -F "#{window_name}" 2>/dev/null | grep -q "^web-apps$"; then
        echo -e "${BLUE}Stopping standalone web-apps window...${NC}"
        # Send Ctrl+C to all panes in the web-apps window
        tmux list-panes -t "$session_name:web-apps" -F "#{pane_id}" | while read pane_id; do
            tmux send-keys -t "$pane_id" C-c 2>/dev/null || true
        done
        sleep 2
        tmux kill-window -t "$session_name:web-apps" 2>/dev/null || true
    fi
    
    # Stop other services in separate windows
    # Kill all windows with these names (in case there are duplicates)
    local services=("baccarat" "holdem" "web-dashboard" "web-games")
    for service_name in "${services[@]}"; do
        # Find all windows with this name (there might be duplicates)
        local windows=$(tmux list-windows -t "$session_name" -F "#{window_index}:#{window_name}" 2>/dev/null | grep ":${service_name}$" | cut -d':' -f1)
        if [ -n "$windows" ]; then
            echo -e "${BLUE}Stopping service: $service_name${NC}"
            # Kill all windows with this name
            for win_idx in $windows; do
                # Send Ctrl+C to gracefully stop
                tmux send-keys -t "$session_name:$win_idx" C-c 2>/dev/null || true
                sleep 1
                # Kill the window
                tmux kill-window -t "$session_name:$win_idx" 2>/dev/null || true
            done
        fi
        rm -f "/tmp/tmux_${service_name}.window"
    done
    
    # Stop infrastructure (optional, commented out to keep running)
    # echo -e "${BLUE}Stopping infrastructure...${NC}"
    # docker-compose down
    
    echo -e "${GREEN}✅ Cleanup completed${NC}"
    echo -e "${YELLOW}Note: tmux session '$session_name' is still running${NC}"
    echo -e "${YELLOW}To kill session: tmux kill-session -t $session_name${NC}"
    echo ""
    echo -e "${BLUE}視窗保持開啟，可再次執行 bash localRun.sh 重新啟動${NC}"
    # 使用 exec 啟動新 shell，避免視窗關閉（Ctrl+C 後仍可繼續使用）
    exec ${SHELL:-bash}
}

# Register cleanup function
trap cleanup SIGINT SIGTERM

# Main flow
main() {
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}  Local Development Environment Startup${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    
    # Check dependencies
    check_docker
    check_air
    check_tmux
    
    # Setup tmux session
    setup_tmux_session
    
    # Start infrastructure
    start_infrastructure
    
    # Set default Redis address for local development
    export REDIS_ADDR="${REDIS_ADDR:-localhost:6379}"
    
    echo ""
    echo -e "${BLUE}🚀 Starting business services in tmux...${NC}"
    
    # Start base services (gate, auth, history, web-games | profile, match, chat, web-dashboard)
    start_base_services_split
    sleep 3
    
    echo ""
    echo -e "${GREEN}========================================${NC}"
    echo -e "${GREEN}✅ All services started!${NC}"
    echo -e "${GREEN}========================================${NC}"
    echo ""
    local session_name=$(get_tmux_session)
    echo -e "${YELLOW}📋 tmux Session: $session_name${NC}"
    echo -e "${YELLOW}========================================${NC}"
    echo ""
    
    # Display quick reference
    echo -e "${CYAN}Quick Reference:${NC}"
    echo -e "  ${BLUE}Ctrl+B then 'd'${NC} - Detach from tmux (services keep running)"
    echo -e "  ${BLUE}Ctrl+B then 'o'${NC} - Switch between panes in current window"
    echo -e "  ${BLUE}Ctrl+B then arrow keys${NC} - Navigate between panes"
    echo -e "  ${BLUE}Ctrl+B then 'w'${NC} - List all windows"
    echo ""
    
    # Ensure mouse mode is enabled before attaching
    echo -e "${BLUE}🔗 Preparing to show base-services window...${NC}"
    tmux set -g mouse on 2>/dev/null || true
    
    # Verify base-services window exists
    if tmux list-windows -t "$session_name" -F "#{window_name}" 2>/dev/null | grep -q "^base-services$"; then
        # Switch to base-services window
        tmux select-window -t "$session_name:base-services" 2>/dev/null || true
        sleep 1
        
        # Select the first pane (gate) to ensure we're viewing the split layout
        tmux select-pane -t "$session_name:base-services.0" 2>/dev/null || true
        sleep 1
        
        echo -e "${GREEN}✅ Ready! Entering tmux session with base-services window...${NC}"
        echo -e "${YELLOW}💡 Layout:${NC}"
        echo -e "${YELLOW}   Left: gate, auth, history, web-games${NC}"
        echo -e "${YELLOW}   Right: profile, match, chat, web-dashboard${NC}"
        echo -e "${YELLOW}💡 Mouse scrolling is enabled - use mouse wheel to scroll${NC}"
        echo -e "${YELLOW}💡 Press Ctrl+B then 'd' to detach and exit${NC}"
        echo ""
        sleep 1
        
        # Attach to tmux session with base-services window already selected
        # Ensure mouse mode is enabled when attaching
        # 不使用 exec，讓 tmux 結束後啟動新 shell，保持視窗開啟（只關閉執行緒，不關閉視窗）
        tmux attach -t "$session_name" \; select-window -t base-services \; select-pane -t 0 \; set -g mouse on || true
    else
        echo -e "${RED}❌ base-services window not found!${NC}"
        echo -e "${YELLOW}   Attaching to tmux anyway...${NC}"
        # Enable mouse mode before attaching
        tmux attach -t "$session_name" \; set -g mouse on || true
    fi
    # tmux 結束後（detach 或 exit）啟動新 shell，保持視窗開啟
    exec ${SHELL:-bash}
}

# Execute main flow
main
