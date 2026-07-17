#!/usr/bin/env bash
# =============================================================================
# Aniya Studio 一次性启动脚本
# -----------------------------------------------------------------------------
# 用法:
#   ./start.sh                  默认 (智能) 启动: 后端/Agent 用 Docker, 前端本地
#   ./start.sh docker           强制 Docker 模式启动后端 + Agent
#   ./start.sh dev              强制源码模式: 三个进程全部本地运行
#   ./start.sh stop             停止所有服务
#   ./start.sh restart          重启所有服务
#   ./start.sh status           查看运行状态
#   ./start.sh logs [name]      查看日志 (name: agentgo|backend|frontend|all)
#   ./start.sh clean            清理日志、pid 文件、停止服务 (不删除容器/卷)
#   ./start.sh help             显示帮助
#
# 端口约定:
#   Frontend (Vite/React):  5173
#   Backend  (FastAPI):     8000
#   AgentGo  (Go Agent):    8080
# =============================================================================

set -euo pipefail

# ---------- 常量 ----------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENTGO_DIR="${SCRIPT_DIR}/agentgo"
BACKEND_DIR="${SCRIPT_DIR}/htmlslide/backend"
FRONTEND_DIR="${SCRIPT_DIR}/htmlslide/frontend"
COMPOSE_DIR="${SCRIPT_DIR}/htmlslide"
LOG_DIR="${SCRIPT_DIR}/logs"
PID_DIR="${SCRIPT_DIR}/.run"

# 端口
PORT_FRONTEND="${PORT_FRONTEND:-5173}"
PORT_BACKEND="${PORT_BACKEND:-8000}"
PORT_AGENTGO="${PORT_AGENTGO:-8080}"

# 颜色
if [[ -t 1 ]]; then
  C_RED='\033[0;31m'
  C_GREEN='\033[0;32m'
  C_YELLOW='\033[0;33m'
  C_BLUE='\033[0;34m'
  C_MAGENTA='\033[0;35m'
  C_CYAN='\033[0;36m'
  C_BOLD='\033[1m'
  C_RESET='\033[0m'
else
  C_RED=''; C_GREEN=''; C_YELLOW=''; C_BLUE=''; C_MAGENTA=''; C_CYAN=''; C_BOLD=''; C_RESET=''
fi

# ---------- 工具函数 ----------
log()    { printf "${C_BLUE}[$(date '+%H:%M:%S')]${C_RESET} %s\n" "$*"; }
ok()     { printf "${C_GREEN}✅ %s${C_RESET}\n" "$*"; }
warn()   { printf "${C_YELLOW}⚠️  %s${C_RESET}\n" "$*"; }
err()    { printf "${C_RED}❌ %s${C_RESET}\n" "$*" >&2; }
banner() { printf "\n${C_BOLD}${C_MAGENTA}🚀 %s${C_RESET}\n" "$*"; }

have()   { command -v "$1" >/dev/null 2>&1; }
die()    { err "$*"; exit 1; }

ensure_dirs() {
  mkdir -p "${LOG_DIR}" "${PID_DIR}"
}

# 检查端口是否被占用
port_in_use() {
  local port="$1"
  if have lsof; then
    lsof -nP -iTCP:"${port}" -sTCP:LISTEN >/dev/null 2>&1
  else
    # macOS 上若没有 lsof, 退化用 netstat
    netstat -an 2>/dev/null | grep -q "\.${port}.*LISTEN"
  fi
}

# 等端口可用
wait_port_free() {
  local port="$1" name="$2" max="${3:-15}"
  local i=0
  while port_in_use "${port}"; do
    i=$((i+1))
    if (( i >= max )); then
      err "${name} 端口 ${port} 持续被占用 ${max}s。请检查占用或运行 ./start.sh stop"
      return 1
    fi
    sleep 1
  done
}

require_port_free() {
  local port="$1" name="$2"
  if port_in_use "${port}"; then
    err "${name} 端口 ${port} 已被占用。请先运行 ./start.sh stop，或手动释放该端口后再启动"
    if have lsof; then
      lsof -nP -iTCP:"${port}" -sTCP:LISTEN || true
    fi
    exit 1
  fi
}

ensure_started_pid_alive() {
  local pid="$1" name="$2" log_file="$3"
  sleep 1
  if ! kill -0 "${pid}" 2>/dev/null; then
    err "${name} 启动后已退出，请查看日志: ${log_file}"
    tail -n 40 "${log_file}" 2>/dev/null || true
    exit 1
  fi
}

stop_port_listener() {
  local port="$1" name="$2"
  have lsof || return 0
  local pids
  pids="$(lsof -tiTCP:"${port}" -sTCP:LISTEN 2>/dev/null || true)"
  [[ -z "${pids}" ]] && return 0

  for pid in ${pids}; do
    if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
      log "停止占用 ${name} 端口 ${port} 的进程 (pid=${pid})"
      pkill -P "${pid}" 2>/dev/null || true
      kill "${pid}" 2>/dev/null || true
    fi
  done
  sleep 1
  for pid in ${pids}; do
    if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
      kill -9 "${pid}" 2>/dev/null || true
    fi
  done
}

# 等端口开始监听
wait_port_listen() {
  local port="$1" name="$2" timeout="${3:-90}"
  local i=0
  while ! port_in_use "${port}"; do
    i=$((i+1))
    if (( i >= timeout )); then
      err "${name} 在 ${timeout}s 内未监听端口 ${port}"
      return 1
    fi
    sleep 1
  done
}

# 等 url 返回 2xx (使用 curl)
wait_http_ok() {
  local url="$1" name="$2" timeout="${3:-90}"
  local i=0
  while true; do
    if curl -fsS --max-time 2 -o /dev/null "${url}" 2>/dev/null; then
      return 0
    fi
    i=$((i+1))
    if (( i >= timeout )); then
      err "${name} 健康检查未通过: ${url} (等待 ${timeout}s)"
      return 1
    fi
    sleep 1
  done
}

# ---------- 环境检查 ----------
preflight() {
  banner "环境检查"

  local missing=()
  have curl || missing+=("curl")

  case "${MODE}" in
    docker|hybrid)
      have docker || missing+=("docker")
      if (( ${#missing[@]} > 0 )); then
        err "缺少必要命令: ${missing[*]}"
        err "请先安装 Docker Desktop 与 curl 后再运行本脚本"
        exit 1
      fi
      if ! docker info >/dev/null 2>&1; then
        die "Docker 未运行。请先启动 Docker Desktop，或使用 ./start.sh dev 纯本地启动"
      fi
      ok "Docker 可用"
      ;;
    dev)
      for cmd in go python3 node npm uv; do
        if ! have "${cmd}"; then
          missing+=("${cmd}")
        fi
      done
      if (( ${#missing[@]} > 0 )); then
        die "dev 模式缺少必要命令: ${missing[*]}"
      fi
      have go      && ok "Go:     $(go version | awk '{print $3}')"
      have python3 && ok "Python: $(python3 --version | awk '{print $2}')"
      have node    && ok "Node:   $(node --version)"
      have uv      && ok "uv:     $(uv --version | awk '{print $2}')"
      ;;
  esac
}

# ---------- .env 处理 ----------
ensure_htmlslide_env() {
  banner "检查 htmlslide/.env"
  local env_file="${COMPOSE_DIR}/.env"
  if [[ -f "${env_file}" ]]; then
    ok "已存在 ${env_file}"
    return 0
  fi

  warn "未找到 ${env_file}, 将创建模板"
  if [[ ! -f "${COMPOSE_DIR}/.env.example" ]]; then
    # 从 README 文档与 compose 中总结的默认模板
    cat > "${env_file}" <<'EOF'
# Docker Compose 启动时从此处读取以下变量:
DEEPSEEK_API_KEY=
DEEPSEEK_MODEL=deepseek-v4-flash
DEEPSEEK_BASE_URL=https://api.deepseek.com
PROVIDER_TYPE=openai
EOF
  else
    cp "${COMPOSE_DIR}/.env.example" "${env_file}"
  fi

  # 如果 key 为空, 提示输入
  if grep -qE '^DEEPSEEK_API_KEY= *$' "${env_file}" || ! grep -q '^DEEPSEEK_API_KEY=' "${env_file}"; then
    echo
    read -rp "请输入 DEEPSEEK_API_KEY (留空可稍后手动填): " api_key
    if [[ -n "${api_key}" ]]; then
      # 替换或追加
      if grep -q '^DEEPSEEK_API_KEY=' "${env_file}"; then
        # macOS/BSD 兼容写法
        sed -i.bak "s|^DEEPSEEK_API_KEY=.*$|DEEPSEEK_API_KEY=${api_key}|" "${env_file}" && rm -f "${env_file}.bak"
      else
        echo "DEEPSEEK_API_KEY=${api_key}" >> "${env_file}"
      fi
      ok "已写入 DEEPSEEK_API_KEY"
    else
      warn "DEEPSEEK_API_KEY 仍为空, Agent 实际调用会失败, 但服务可以先启动"
    fi
  fi
}

ensure_agentgo_env() {
  banner "检查 agentgo/.env (仅 dev 模式需要)"
  local env_file="${AGENTGO_DIR}/.env"
  if [[ -f "${env_file}" ]]; then
    ok "已存在 ${env_file}"
    return 0
  fi
  if [[ -f "${AGENTGO_DIR}/.env.example" ]]; then
    cp "${AGENTGO_DIR}/.env.example" "${env_file}"
    ok "已从 .env.example 复制"
  fi
}

ensure_backend_env() {
  banner "检查 backend/.env (仅独立 dev 调试需要, docker 模式不读)"
  local env_file="${BACKEND_DIR}/.env"
  if [[ -f "${env_file}" ]]; then
    return 0
  fi
  if [[ -f "${BACKEND_DIR}/.env.example" ]]; then
    cp "${BACKEND_DIR}/.env.example" "${env_file}"
    ok "已从 .env.example 复制 backend/.env"
  fi
}

# ---------- 启动: docker / hybrid 模式 ----------
start_docker_backend() {
  banner "启动 AgentGo + Backend (Docker Compose)"
  ensure_htmlslide_env

  pushd "${COMPOSE_DIR}" >/dev/null
  # 强制重建, 确保使用最新的 Dockerfile
  docker compose up -d --build
  popd >/dev/null

  # 等 AgentGo 健康
  log "等待 AgentGo 健康检查 (http://localhost:${PORT_AGENTGO}/health) ..."
  # 容器内 wget 可以, 主机用 curl
  wait_http_ok "http://localhost:${PORT_AGENTGO}/health" "AgentGo" 120 \
    && ok "AgentGo 健康 (端口 ${PORT_AGENTGO})"

  # 等 Backend 起来 (容器里 uvicorn 启动较慢)
  log "等待 Backend 端口 ${PORT_BACKEND} 监听 ..."
  wait_port_listen "${PORT_BACKEND}" "Backend" 60 \
    && ok "Backend 已监听端口 ${PORT_BACKEND}"
}

# ---------- 启动: dev 模式 ----------
start_agentgo_dev() {
  banner "启动 AgentGo (源码模式, 端口 ${PORT_AGENTGO})"
  ensure_agentgo_env

  pushd "${AGENTGO_DIR}" >/dev/null
  # shellcheck disable=SC1090
  [[ -f .env ]] && set -a && . ./.env && set +a

  export AGENTGO_SKILLS_DIR="${AGENTGO_SKILLS_DIR:-${AGENTGO_DIR}/skills}"
  export AGENTGO_PROJECT_SKILLS_DIR="${AGENTGO_PROJECT_SKILLS_DIR:-${AGENTGO_DIR}/project-skills}"

  # vendor 模式构建; 若失败回退到普通 go build
  nohup go run -mod=vendor ./cmd/server \
    > "${LOG_DIR}/agentgo.log" 2>&1 &
  echo $! > "${PID_DIR}/agentgo.pid"
  local pid="$!"
  popd >/dev/null

  ensure_started_pid_alive "${pid}" "AgentGo" "${LOG_DIR}/agentgo.log"
  log "PID: $(cat "${PID_DIR}/agentgo.pid"), 等待端口 ${PORT_AGENTGO} 监听 ..."
  wait_port_listen "${PORT_AGENTGO}" "AgentGo" 90 \
    && ok "AgentGo 已监听端口 ${PORT_AGENTGO}"
}

start_backend_dev() {
  banner "启动 Backend (源码模式, 端口 ${PORT_BACKEND})"
  ensure_backend_env

  if [[ ! -d "${BACKEND_DIR}/.venv" ]]; then
    log "首次启动, 同步 Python 依赖 (uv sync) ..."
    pushd "${BACKEND_DIR}" >/dev/null
    uv sync
    popd >/dev/null
    ok "依赖同步完成"
  fi

  # 设置 Backend 指向本机 agentgo
  export AGENT_URL="http://localhost:${PORT_AGENTGO}"
  # 避免系统代理 (Clash/Surge) 劫持 Backend→AgentGo
  export NO_PROXY="${NO_PROXY:-localhost,127.0.0.1,::1,agentgo}"
  export no_proxy="${no_proxy:-${NO_PROXY}}"
  export WORKSPACE_PATH="${WORKSPACE_PATH:-${SCRIPT_DIR}/workspace}"
  mkdir -p "${WORKSPACE_PATH}"

  pushd "${BACKEND_DIR}" >/dev/null
  nohup uv run uvicorn src.main:app --host 0.0.0.0 --port "${PORT_BACKEND}" \
    > "${LOG_DIR}/backend.log" 2>&1 &
  echo $! > "${PID_DIR}/backend.pid"
  local pid="$!"
  popd >/dev/null

  ensure_started_pid_alive "${pid}" "Backend" "${LOG_DIR}/backend.log"
  log "PID: $(cat "${PID_DIR}/backend.pid"), 等待端口 ${PORT_BACKEND} 监听 ..."
  wait_port_listen "${PORT_BACKEND}" "Backend" 120 \
    && ok "Backend 已监听端口 ${PORT_BACKEND}"
}

start_frontend_dev() {
  banner "启动 Frontend (Vite, 端口 ${PORT_FRONTEND})"

  if [[ ! -d "${FRONTEND_DIR}/node_modules" ]]; then
    log "首次启动, 安装前端依赖 (npm install) ..."
    pushd "${FRONTEND_DIR}" >/dev/null
    # 若锁文件是 npm 锁就用 npm, 否则尝试 pnpm 兜底
    if [[ -f package-lock.json ]] && have npm; then
      npm install --silent
    elif [[ -f pnpm-lock.yaml ]] && have pnpm; then
      pnpm install --silent
    else
      npm install --silent
    fi
    popd >/dev/null
    ok "前端依赖已安装"
  fi

  pushd "${FRONTEND_DIR}" >/dev/null
  PORT="${PORT_FRONTEND}" nohup npm run dev -- --host 0.0.0.0 \
    --strictPort \
    > "${LOG_DIR}/frontend.log" 2>&1 &
  echo $! > "${PID_DIR}/frontend.pid"
  local pid="$!"
  popd >/dev/null

  ensure_started_pid_alive "${pid}" "Frontend" "${LOG_DIR}/frontend.log"
  log "PID: $(cat "${PID_DIR}/frontend.pid"), 等待端口 ${PORT_FRONTEND} 监听 ..."
  wait_port_listen "${PORT_FRONTEND}" "Frontend" 60 \
    && ok "Frontend 已监听端口 ${PORT_FRONTEND}"
}

# ---------- 主流程 ----------
cmd_start() {
  preflight
  ensure_dirs

  if [[ "${MODE}" == "docker" ]]; then
    # 纯 Docker: 仅启动后端 + agent, 前端必须本地
    require_port_free "${PORT_AGENTGO}" "AgentGo"
    require_port_free "${PORT_BACKEND}" "Backend"
    require_port_free "${PORT_FRONTEND}" "Frontend"
    start_docker_backend
    start_frontend_dev
  elif [[ "${MODE}" == "dev" ]]; then
    # 纯源码
    require_port_free "${PORT_AGENTGO}" "AgentGo"
    require_port_free "${PORT_BACKEND}" "Backend"
    require_port_free "${PORT_FRONTEND}" "Frontend"
    start_agentgo_dev
    start_backend_dev
    start_frontend_dev
  else
    # hybrid (默认): 等同 docker + frontend
    require_port_free "${PORT_AGENTGO}" "AgentGo"
    require_port_free "${PORT_BACKEND}" "Backend"
    require_port_free "${PORT_FRONTEND}" "Frontend"
    start_docker_backend
    start_frontend_dev
  fi

  save_runtime_mode
  print_summary
}

save_runtime_mode() {
  echo "${MODE}" > "${PID_DIR}/mode"
}

cmd_stop() {
  banner "停止所有服务"

  # 1. dev 模式: kill pid 记录的进程
  for name in agentgo backend frontend; do
    local pid_file="${PID_DIR}/${name}.pid"
    if [[ -f "${pid_file}" ]]; then
      local pid
      pid="$(cat "${pid_file}" || true)"
      if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
        log "停止 ${name} (pid=${pid})"
        # 杀掉进程组 (nohup 启动)
        pkill -P "${pid}" 2>/dev/null || true
        kill "${pid}" 2>/dev/null || true
        sleep 1
        kill -9 "${pid}" 2>/dev/null || true
      fi
      rm -f "${pid_file}"
    fi
  done

  # 如果 pid 文件被失败启动覆盖，仍按项目固定端口兜底清理旧监听进程。
  stop_port_listener "${PORT_AGENTGO}" "AgentGo"
  stop_port_listener "${PORT_BACKEND}" "Backend"
  stop_port_listener "${PORT_FRONTEND}" "Frontend"

  # 2. docker compose down (如果之前是 docker/hybrid)
  if [[ -f "${COMPOSE_DIR}/docker-compose.yml" ]] && have docker; then
    if docker compose -f "${COMPOSE_DIR}/docker-compose.yml" ps --services 2>/dev/null | grep -q .; then
      log "停止 docker compose 服务"
      pushd "${COMPOSE_DIR}" >/dev/null
      docker compose down 2>/dev/null || true
      popd >/dev/null
    fi
  fi

  ok "已停止"
}

cmd_status() {
  banner "服务状态"
  local mode
  mode="$(cat "${PID_DIR}/mode" 2>/dev/null || echo unknown)"
  echo "运行模式: ${mode}"
  echo

  printf "%-10s %-8s %-25s %s\n" "服务" "端口" "状态" "地址"
  printf "%-10s %-8s %-25s %s\n" "----" "----" "----" "----"

  # frontend
  if port_in_use "${PORT_FRONTEND}"; then
    printf "${C_GREEN}%-10s %-8s %-25s %s${C_RESET}\n" "Frontend" "${PORT_FRONTEND}" "运行中" "http://localhost:${PORT_FRONTEND}"
  else
    printf "${C_RED}%-10s %-8s %-25s %s${C_RESET}\n" "Frontend" "${PORT_FRONTEND}" "未运行" "-"
  fi

  # backend
  if port_in_use "${PORT_BACKEND}"; then
    printf "${C_GREEN}%-10s %-8s %-25s %s${C_RESET}\n" "Backend" "${PORT_BACKEND}" "运行中" "http://localhost:${PORT_BACKEND}/docs"
  else
    printf "${C_RED}%-10s %-8s %-25s %s${C_RESET}\n" "Backend" "${PORT_BACKEND}" "未运行" "-"
  fi

  # agentgo
  if port_in_use "${PORT_AGENTGO}"; then
    local health="运行中"
    if curl -fsS --max-time 2 "http://localhost:${PORT_AGENTGO}/health" >/dev/null 2>&1; then
      health="${health} (健康)"
    else
      health="${health} (无响应)"
    fi
    printf "${C_GREEN}%-10s %-8s %-25s %s${C_RESET}\n" "AgentGo" "${PORT_AGENTGO}" "${health}" "http://localhost:${PORT_AGENTGO}/health"
  else
    printf "${C_RED}%-10s %-8s %-25s %s${C_RESET}\n" "AgentGo" "${PORT_AGENTGO}" "未运行" "-"
  fi

  echo
  echo "日志目录: ${LOG_DIR}"
  ls -lh "${LOG_DIR}" 2>/dev/null | tail -n +2 || true
}

cmd_logs() {
  local target="${1:-all}"
  case "${target}" in
    agentgo|backend|frontend)
      tail -n 200 -f "${LOG_DIR}/${target}.log"
      ;;
    all|"")
      if have multitail || have lnav; then
        if have multitail; then
          multitail -s 2 \
            "${LOG_DIR}/agentgo.log" \
            "${LOG_DIR}/backend.log" \
            "${LOG_DIR}/frontend.log"
        else
          lnav "${LOG_DIR}"/*.log
        fi
      else
        echo "=== agentgo ==="
        tail -n 50 "${LOG_DIR}/agentgo.log" 2>/dev/null || echo "(无)"
        echo
        echo "=== backend ==="
        tail -n 50 "${LOG_DIR}/backend.log" 2>/dev/null || echo "(无)"
        echo
        echo "=== frontend ==="
        tail -n 50 "${LOG_DIR}/frontend.log" 2>/dev/null || echo "(无)"
        echo
        echo "提示: 安装 multitail 或 lnav 可获得分屏实时日志体验"
      fi
      ;;
    *)
      err "未知日志名: ${target} (可用: agentgo|backend|frontend|all)"
      exit 1
      ;;
  esac
}

cmd_clean() {
  banner "清理"
  cmd_stop
  rm -rf "${LOG_DIR}" "${PID_DIR}"
  ok "已清理日志与 pid 文件"
}

cmd_restart() {
  cmd_stop
  sleep 2
  cmd_start
}

print_summary() {
  banner "全部启动完成 ✨"
  echo -e "${C_BOLD}访问地址${C_RESET}"
  echo -e "  • ${C_CYAN}前端编辑器${C_RESET}:    http://localhost:${PORT_FRONTEND}"
  echo -e "  • ${C_CYAN}Backend API 文档${C_RESET}: http://localhost:${PORT_BACKEND}/docs"
  echo -e "  • ${C_CYAN}Agent 健康检查${C_RESET}:  http://localhost:${PORT_AGENTGO}/health"
  echo
  echo -e "${C_BOLD}日志与控制${C_RESET}"
  echo -e "  • ${C_CYAN}日志目录${C_RESET}: ${LOG_DIR}/  (agentgo.log / backend.log / frontend.log)"
  echo -e "  • ${C_CYAN}查看状态${C_RESET}: ./start.sh status"
  echo -e "  • ${C_CYAN}查看日志${C_RESET}: ./start.sh logs [agentgo|backend|frontend|all]"
  echo -e "  • ${C_CYAN}停止服务${C_RESET}: ./start.sh stop"
  echo -e "  • ${C_CYAN}重启服务${C_RESET}: ./start.sh restart"
  echo
  echo -e "${C_BOLD}当前模式${C_RESET}: ${MODE}  (hybrid: docker backend + 本地 frontend / docker: 全部 docker / dev: 全部源码)"
}

print_help() {
  cat <<EOF
${C_BOLD}Aniya Studio 一次性启动脚本${C_RESET}

${C_BOLD}用法${C_RESET}
  ./start.sh [子命令]

${C_BOLD}子命令${C_RESET}
  start     启动 (默认行为, 等同于直接运行 ./start.sh)
            - hybrid: 后端/Agent 用 Docker, 前端本地 (推荐)
  docker    强制使用 docker compose 启动 (后端 + Agent), 前端仍本地
  dev       强制源码模式: 三个进程全部本地运行 (go / uv / npm)
  stop      停止所有服务 (含 docker compose down)
  restart   重启
  status    查看运行状态与端口
  logs      跟踪日志 (参数: agentgo|backend|frontend|all, 默认 all)
  clean     停止并清理日志和 pid 文件 (不删除容器/卷)
  help      显示本帮助

${C_BOLD}环境变量 (可选)${C_RESET}
  PORT_FRONTEND  默认 5173
  PORT_BACKEND   默认 8000
  PORT_AGENTGO   默认 8080
  WORKSPACE_PATH 默认 \${SCRIPT_DIR}/workspace (仅 dev 模式使用)

${C_BOLD}首次运行${C_RESET}
  1. 脚本会创建 \${COMPOSE_DIR}/.env (如不存在), 提示填入 DEEPSEEK_API_KEY
  2. dev 模式会创建 agentgo/.env 与 backend/.env
  3. 默认 start/hybrid 需要 Docker Desktop；纯本地请显式运行 ./start.sh dev
  4. Docker 模式首次会构建镜像 (耗时较长, 后续秒启)

EOF
}

# ---------- 入口 ----------
MODE="hybrid"
SUBCMD="${1:-start}"

case "${SUBCMD}" in
  start|"")
    MODE="${2:-hybrid}"
    cmd_start
    ;;
  docker)  MODE="docker";  cmd_start ;;
  dev)     MODE="dev";     cmd_start ;;
  stop)    cmd_stop ;;
  restart) cmd_restart ;;
  status)  cmd_status ;;
  logs)    shift; cmd_logs "${@:-all}" ;;
  clean)   cmd_clean ;;
  help|-h|--help) print_help ;;
  *) err "未知子命令: ${SUBCMD}"; print_help; exit 1 ;;
esac
