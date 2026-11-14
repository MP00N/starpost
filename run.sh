#!/usr/bin/env bash
#
# run.sh — 一条命令把站点跑起来，用来试玩、调设置、对接上游。
#
# 用法：
#   ./run.sh                   # 编译（必要时）+ 启动，默认 3000 端口
#   ./run.sh --port 8080       # 换端口
#   ./run.sh --dev             # 后端 + 前端热更新（改代码即时生效）
#   ./run.sh --rebuild         # 强制重编前端和后端
#   ./run.sh --fresh           # 清空本地数据库重新开始
#   ./run.sh --no-setup        # 跳过自动建管理员（你自己在网页里建）
#
# 说明：
#   数据全部放在 ./data，删掉这个目录就是全新环境。
#   默认用 SQLite，不依赖 MySQL / Redis，开箱即用。
#   这个脚本会占用当前终端（前台运行），Ctrl-C 停止——这样进程不会被
#   容器的退出机制顺手杀掉。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

PORT=3000
MODE="prod"
REBUILD=0
FRESH=0
AUTO_SETUP=1
ADMIN_USER="admin"
ADMIN_PASS=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --port) PORT="$2"; shift 2 ;;
    --dev) MODE="dev"; shift ;;
    --rebuild) REBUILD=1; shift ;;
    --fresh) FRESH=1; shift ;;
    --no-setup) AUTO_SETUP=0; shift ;;
    -h|--help) sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "未知参数: $1（--help 查看用法）" >&2; exit 1 ;;
  esac
done

c_info() { printf '\033[32m▸\033[0m %s\n' "$*"; }
c_warn() { printf '\033[33m!\033[0m %s\n' "$*"; }
c_err()  { printf '\033[31m✗\033[0m %s\n' "$*" >&2; }
die()    { c_err "$*"; exit 1; }

# ── 环境检查 ────────────────────────────────────────────────────────────────
# 机器上可能同时存在多个 Go（系统包管理器装的和手动装的）。go.mod 要求
# 1.26+，用错的那个会直接编译失败，所以按版本挑最高的，而不是按 PATH 顺序。
pick_go() {
  local best="" best_score=0 cand ver major minor score
  for cand in /usr/local/go/bin/go "$(command -v go 2>/dev/null || true)"; do
    [[ -n "$cand" && -x "$cand" ]] || continue
    # GOTOOLCHAIN=local + timeout: without them `go version` may try to fetch a
    # matching toolchain over the network and hang for a minute before this
    # script prints anything at all.
    ver="$(GOTOOLCHAIN=local timeout 5 "$cand" version 2>/dev/null \
      | sed -n 's/.*go\([0-9]*\.\([0-9]*\)\).*/\1/p')"
    [[ -z "$ver" ]] && continue
    major="${ver%%.*}"
    minor="${ver#*.}"
    score=$(( major * 1000 + minor ))
    if (( score > best_score )); then
      best_score=$score
      best="$cand"
    fi
  done
  printf '%s' "$best"
}
GO_BIN="$(pick_go)"

BUN_BIN="$(command -v bun 2>/dev/null || true)"

DATA_DIR="$ROOT/data"
DB_FILE="$DATA_DIR/one-api.db"
# 初始管理员凭据（明文只用于本地试玩，不要提交到仓库）
CRED_FILE="$DATA_DIR/admin-credentials.txt"
BIN="$ROOT/new-api"
DIST="$ROOT/web/dist"

mkdir -p "$DATA_DIR"

if [[ $FRESH -eq 1 ]]; then
  # 删库是不可逆的：渠道、用户、令牌、兑换码全没了，要重新配一遍。
  # 所以要二次确认，并且明确列出会丢什么。
  if [[ -f "$DB_FILE" ]]; then
    c_warn "即将删除数据库：$DB_FILE"
    c_warn "这会清空：管理员账号、渠道配置、用户、令牌、兑换码、日志"
    if [[ -t 0 ]]; then
      read -r -p "确认删除？输入 yes 继续: " reply
      if [[ "$reply" != "yes" ]]; then
        c_info "已取消，数据保留。"
        exit 0
      fi
    else
      c_err "非交互环境下 --fresh 被拒绝（避免误删）。确定要删请加 --yes"
      exit 1
    fi
  fi
  rm -f "$DB_FILE" "$DB_FILE"-shm "$DB_FILE"-wal
fi

# ── 前端 ────────────────────────────────────────────────────────────────────
build_web() {
  [[ -z "$BUN_BIN" ]] && die "需要 bun 来编译前端，但没找到。装一个：https://bun.sh"
  c_info "安装前端依赖…"
  ( cd "$ROOT/web" && "$BUN_BIN" install --backend=copyfile )
  c_info "编译前端…"
  ( cd "$ROOT/web" && "$BUN_BIN" run build )
}

if [[ $MODE == "prod" ]]; then
  if [[ $REBUILD -eq 1 || ! -f "$DIST/index.html" ]]; then
    build_web
  else
    c_info "前端已有产物，跳过编译（要重编加 --rebuild）"
  fi
fi

# ── 后端 ────────────────────────────────────────────────────────────────────
build_api() {
  [[ -z "$GO_BIN" ]] && die "需要 Go 来编译后端，但没找到。装一个：https://go.dev/dl/"
  c_info "编译后端…"
  export PATH="$(dirname "$GO_BIN"):$PATH"
  export GOTOOLCHAIN=local
  # 不强制设置 GOPROXY：依赖已经在本地模块缓存里，强行指到官方代理反而会
  # 让 go 去访问被墙的域名，编译直接卡死。需要代理的人自己 export GOPROXY。
  # 带上版本号，否则页头会显示 "Unknown version"
  ( cd "$ROOT" && go build \
      -ldflags "-X 'github.com/QuantumNous/new-api/common.Version=${VERSION:-dev}'" \
      -o "$BIN" . )
}

if [[ $MODE == "prod" ]]; then
  if [[ $REBUILD -eq 1 || ! -x "$BIN" ]]; then
    build_api
  else
    c_info "后端已有可执行文件，跳过编译（要重编加 --rebuild）"
  fi
fi

# ── 启动 ────────────────────────────────────────────────────────────────────
export PORT
export TZ="${TZ:-Asia/Shanghai}"
# 绝对路径。相对路径会让"从不同目录启动"连到不同的库，表现为"数据没了"，
# 实际是换了个空库——这个坑排查起来很费时间。
export SQLITE_PATH="${SQLITE_PATH:-$DB_FILE}"
case "$SQLITE_PATH" in
  /*) : ;;
  *) SQLITE_PATH="$(cd "$(dirname "$SQLITE_PATH")" 2>/dev/null && pwd)/$(basename "$SQLITE_PATH")" ;;
esac
export ERROR_LOG_ENABLED="${ERROR_LOG_ENABLED:-true}"
export SESSION_COOKIE_SECURE="${SESSION_COOKIE_SECURE:-false}"

LAN_IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
[[ -z "$LAN_IP" ]] && LAN_IP="$(ip route get 1 2>/dev/null | awk '{print $7; exit}')"

cleanup() {
  local code=$?
  echo
  c_info "已停止（退出码 $code）"
}
trap cleanup EXIT

echo
echo "──────────────────────────────────────────────"
c_info "端口      $PORT"
c_info "数据库    $SQLITE_PATH"
if [[ -f "$SQLITE_PATH" ]]; then
  c_info "数据      沿用已有库（渠道与账号都在），要清空请用 --fresh"
else
  c_info "数据      新建数据库（首次启动）"
fi
c_info "模式      $([[ $MODE == dev ]] && echo '开发（热更新）' || echo '生产')"
echo "──────────────────────────────────────────────"
echo

if [[ $MODE == "dev" ]]; then
  [[ -z "$BUN_BIN" ]] && die "需要 bun 才能跑前端开发服务器"
  c_info "启动后端（后台）+ 前端开发服务器（前台）"
  ( cd "$ROOT" && "$BIN" --port "$PORT" --log-dir "$DATA_DIR/logs" ) &
  API_PID=$!
  trap 'kill $API_PID 2>/dev/null || true; cleanup' EXIT
  sleep 2
  ( cd "$ROOT/web" && "$BUN_BIN" run dev -- --port 5173 )
  exit 0
fi

# 生产模式：前台跑，Ctrl-C 停止
c_info "启动中… 首次启动会建表，稍等几秒"
"$BIN" --port "$PORT" --log-dir "$DATA_DIR/logs" &
API_PID=$!
trap 'kill $API_PID 2>/dev/null || true; cleanup' EXIT

# 等站点就绪
READY=0
for _ in $(seq 1 60); do
  if curl -s -m 2 -o /dev/null "http://127.0.0.1:$PORT/api/status"; then READY=1; break; fi
  sleep 1
done
[[ $READY -eq 1 ]] || die "服务没起来，看日志：$DATA_DIR/logs"

# ── 首次进入：自动建管理员 ──────────────────────────────────────────────────
# 只认 status 一个字段。已初始化的站再查 /api/setup 时，root_init 恒为
# false（后端提前返回时没填这个字段），拿它判断会误伤所有第二次启动。
SETUP_STATUS="$(curl -s -m 5 "http://127.0.0.1:$PORT/api/setup" \
  | sed -n 's/.*"status":\(true\|false\).*/\1/p')"

if [[ "$SETUP_STATUS" == "false" && $AUTO_SETUP -eq 1 ]]; then
  ADMIN_PASS="$(head -c 24 /dev/urandom | base64 | tr -d '\n/+=' | head -c 14)"
  c_info "初始化站点，创建管理员…"
  if curl -s -m 30 -X POST "http://127.0.0.1:$PORT/api/setup" \
    -H 'Content-Type: application/json' \
    -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\",\"confirmPassword\":\"$ADMIN_PASS\",\"SelfUseModeEnabled\":false,\"DemoSiteEnabled\":false}" \
    >/dev/null; then
    # 密码只在这时是明文（之后库里是哈希），落一份到 data/ 以便每次启动都能打印。
    # 权限 600：只有当前用户能读。
    umask 077
    printf '%s\n%s\n' "$ADMIN_USER" "$ADMIN_PASS" > "$CRED_FILE"
    chmod 600 "$CRED_FILE" 2>/dev/null || true
  else
    c_warn "自动初始化失败，你可以打开网页手动创建"
  fi
fi

# ── 品牌默认值：直接写数据库，不必进后台逐个改名 ─────────────────────────
# 项目 AGENTS.md 把源码里的品牌字符串列为受保护内容，所以不去改 constants.go，
# 而是在安装时把「站点选项」写进数据库——这正是后台「系统设置 → 站点」做的事。
#
# 退出码 10 表示「写入了新值」：/api/status 读的是启动时载入内存的选项，
# 写完不重启，接口返回的还是旧名称。
# 这里不能写成 `if cmd; then` —— $? 会被 if 自己清掉，必须先取值再判断。
BRAND_APPLIED=0
if [[ -f "$SQLITE_PATH" ]]; then
  # 先取退出码再判断。写成 `cmd; [[ $? -eq 10 ]] && X` 在 set -e 下是个陷阱：
  # 条件不成立时 [[ ]] 返回 1，而它是 && 列表的最后一条，整个表达式失败会
  # 直接终止脚本——表现为「创建完管理员就退出」。
  set +e
  python3 "$ROOT/tools/apply-branding.py" "$SQLITE_PATH" "/starpost-mark.svg"
  brand_code=$?
  set -e
  if [[ "$brand_code" -eq 10 ]]; then
    BRAND_APPLIED=1
  fi
fi

if [[ "${BRAND_APPLIED:-0}" == "1" ]]; then
  c_info "应用站点名称与主题，重启一次…"
  kill $API_PID 2>/dev/null || true
  wait $API_PID 2>/dev/null || true
  "$BIN" --port "$PORT" --log-dir "$DATA_DIR/logs" &
  API_PID=$!
  trap 'kill $API_PID 2>/dev/null || true; cleanup' EXIT
  for _ in $(seq 1 40); do
    curl -s -m 2 -o /dev/null "http://127.0.0.1:$PORT/api/status" && break
    sleep 1
  done
fi

echo
echo "══════════════════════════════════════════════"
c_info "站点已就绪"
echo
echo "   本机     http://127.0.0.1:$PORT"
[[ -n "$LAN_IP" ]] && echo "   局域网   http://$LAN_IP:$PORT"
echo
# 每次启动都打印初始凭据：调试时最常问的就是「密码是多少」，翻日志很烦。
# 若在后台改过密码，这里显示的是初始值，会与当前密码不一致——所以标注清楚。
if [[ -f "$CRED_FILE" ]]; then
  CRED_USER="$(sed -n '1p' "$CRED_FILE")"
  CRED_PASS="$(sed -n '2p' "$CRED_FILE")"
  echo "   管理员   $CRED_USER / $CRED_PASS"
  echo "            （初始密码；若你已在后台改过，以你改的为准）"
  echo
elif [[ -n "$ADMIN_PASS" ]]; then
  echo "   管理员   $ADMIN_USER / $ADMIN_PASS"
  echo
fi
echo "   建议接下来做："
echo "     1. 右上角 系统设置 → 站点：改站点名称、Logo"
echo "     2. 渠道：添加上游（填 base_url 和 key），然后测试渠道"
echo "     3. 用户：给自己发一个 Key，用 /v1/chat/completions 试一次"
echo
echo "   日志  $DATA_DIR/logs"
echo "   停止  Ctrl-C"
echo "══════════════════════════════════════════════"
echo

wait $API_PID
