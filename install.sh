#!/usr/bin/env bash
#
# New API 一键安装脚本（宝塔面板 / 通用 Linux）
#
# 用法：
#   bash install.sh                     # 交互式安装
#   bash install.sh --port 3000         # 指定端口
#   bash install.sh --dir /www/wwwroot/new-api
#   bash install.sh --binary /path/new-api   # 跳过编译，直接用已有二进制
#   bash install.sh --no-build          # 只做运行环境配置，不编译
#
# 设计说明：
#   1. 宝塔默认以 root 运行，脚本优先适配 root；非 root 时用 sudo。
#   2. 宝塔自带的 MySQL 可直接复用，脚本不强制装数据库。
#   3. 未指定数据库时默认用 SQLite，开箱即用，后续可在面板改 SQL_DSN。
#   4. 编译需要 Go 1.26+ 与 Bun 1.4+，都会自动下载到本地目录，不动系统全局。
#   5. 中国网络环境默认走国内镜像，可用 --no-mirror 关掉。

set -euo pipefail

# ---------- 默认参数 ----------
APP_NAME="new-api"
INSTALL_DIR="/www/wwwroot/new-api"
PORT="3000"
LOG_DIR=""
BINARY_SRC=""
DO_BUILD=1
USE_MIRROR=1
SERVICE_NAME="new-api"
GO_VERSION="1.26.10"
BUN_VERSION="1.4.4"
SQL_DSN=""
SQLITE_PATH=""
SKIP_DEPS=0

# ---------- 输出 ----------
c_info() { printf '\033[32m[信息]\033[0m %s\n' "$*"; }
c_warn() { printf '\033[33m[警告]\033[0m %s\n' "$*"; }
c_err()  { printf '\033[31m[错误]\033[0m %s\n' "$*" >&2; }
die()    { c_err "$*"; exit 1; }

# ---------- 参数解析 ----------
while [[ $# -gt 0 ]]; do
  case "$1" in
    --port)        PORT="$2"; shift 2 ;;
    --dir)         INSTALL_DIR="$2"; shift 2 ;;
    --log-dir)     LOG_DIR="$2"; shift 2 ;;
    --binary)      BINARY_SRC="$2"; DO_BUILD=0; shift 2 ;;
    --no-build)    DO_BUILD=0; shift ;;
    --sql-dsn)     SQL_DSN="$2"; shift 2 ;;
    --sqlite-path) SQLITE_PATH="$2"; shift 2 ;;
    --no-mirror)   USE_MIRROR=0; shift ;;
    --skip-deps)   SKIP_DEPS=1; shift ;;
    -h|--help)
      sed -n '/^# 用法：/,/^#   5\./p' "$0" | sed 's/^# \{0,1\}//'
      exit 0 ;;
    *) die "未知参数: $1（用 --help 查看用法）" ;;
  esac
done

[[ -z "$LOG_DIR" ]] && LOG_DIR="$INSTALL_DIR/logs"

# ---------- 基础检查 ----------
c_info "检查运行环境..."

if [[ $EUID -eq 0 ]]; then
  SUDO=""
  RUN_USER="root"
else
  command -v sudo >/dev/null || die "非 root 用户需要 sudo"
  SUDO="sudo"
  RUN_USER="$(id -un)"
  c_warn "当前非 root，将使用 sudo；systemd 服务仍以 $RUN_USER 运行"
fi

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  GO_ARCH="amd64"; BUN_ARCH="x64" ;;
  aarch64|arm64) GO_ARCH="arm64"; BUN_ARCH="arm64" ;;
  *) die "不支持的架构: $ARCH（仅支持 x86_64 / aarch64）" ;;
esac
c_info "架构: $ARCH"

# 发行版包管理器
PKG=""
if command -v apt-get >/dev/null; then PKG="apt"
elif command -v dnf >/dev/null;   then PKG="dnf"
elif command -v yum >/dev/null;   then PKG="yum"
elif command -v apk >/dev/null;   then PKG="apk"
fi
[[ -n "$PKG" ]] && c_info "包管理器: $PKG"
[[ -z "$PKG" ]] && c_warn "未识别包管理器，跳过系统依赖自动安装"

install_pkgs() {
  [[ $SKIP_DEPS -eq 1 || -z "$PKG" ]] && return 0
  local pkgs=("$@")
  local missing=()
  for p in "${pkgs[@]}"; do
    command -v "$p" >/dev/null 2>&1 || missing+=("$p")
  done
  [[ ${#missing[@]} -eq 0 ]] && return 0

  c_info "安装系统依赖: ${missing[*]}"
  case "$PKG" in
    apt)
      if [[ $USE_MIRROR -eq 1 && -f /etc/os-release ]] && grep -qi ubuntu /etc/os-release; then
        c_info "使用清华 apt 镜像"
        local src=/etc/apt/sources.list.d/ubuntu.sources
        if [[ -f "$src" ]] && ! grep -q "tuna.tsinghua" "$src" 2>/dev/null; then
          $SUDO cp "$src" "$src.bak.$(date +%s)" 2>/dev/null || true
          $SUDO sed -i 's|http://[a-z.]*archive.ubuntu.com|https://mirrors.tuna.tsinghua.edu.cn|g;
                        s|http://security.ubuntu.com|https://mirrors.tuna.tsinghua.edu.cn|g' "$src" 2>/dev/null || true
        fi
      fi
      $SUDO apt-get update -qq
      DEBIAN_FRONTEND=noninteractive $SUDO apt-get install -y -qq "${missing[@]}"
      ;;
    dnf) $SUDO dnf install -y "${missing[@]}" ;;
    yum) $SUDO yum install -y "${missing[@]}" ;;
    apk) $SUDO apk add --no-cache "${missing[@]}" ;;
  esac
}

install_pkgs curl tar git unzip

# ---------- 准备目录 ----------
c_info "准备安装目录: $INSTALL_DIR"
$SUDO mkdir -p "$INSTALL_DIR" "$LOG_DIR"

# 脚本所在目录即源码目录（若存在 go.mod 则视为源码安装）
SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$SRC_DIR/go.mod" && "$SRC_DIR" != "$INSTALL_DIR" ]]; then
  c_info "从源码目录部署: $SRC_DIR -> $INSTALL_DIR"
  $SUDO mkdir -p "$INSTALL_DIR"
  # 排除构建产物与依赖目录，避免覆盖或拖慢
  $SUDO tar -C "$SRC_DIR" \
    --exclude='./web/node_modules' --exclude='./.git' \
    --exclude='./logs' --exclude='./data' \
    -cf - . | $SUDO tar -C "$INSTALL_DIR" -xf -
fi

$SUDO chown -R "$RUN_USER":"$RUN_USER" "$INSTALL_DIR" 2>/dev/null || true

# ---------- 安装工具链 ----------
TOOLS_DIR="$INSTALL_DIR/.toolchain"
$SUDO mkdir -p "$TOOLS_DIR"

local_go() { echo "$TOOLS_DIR/go/bin/go"; }
local_bun() { echo "$TOOLS_DIR/bun/bin/bun"; }

have_go() {
  local g
  g="$(command -v go 2>/dev/null || true)"
  [[ -x "$(local_go)" ]] && g="$(local_go)"
  [[ -z "$g" ]] && return 1
  local v
  v="$("$g" version 2>/dev/null | sed -n 's/.*go\([0-9]*\.[0-9]*\).*/\1/p')"
  [[ -n "$v" ]] || return 1
  local major minor need_major need_minor
  major="${v%%.*}"; minor="${v#*.}"
  need_major="${GO_VERSION%%.*}"; need_minor="${GO_VERSION#*.}"; need_minor="${need_minor%%.*}"
  (( major > need_major )) || { (( major == need_major )) && (( minor >= need_minor )); }
}

have_bun() {
  local b
  b="$(command -v bun 2>/dev/null || true)"
  [[ -x "$(local_bun)" ]] && b="$(local_bun)"
  [[ -z "$b" ]] && return 1
  local v; v="$("$b" --version 2>/dev/null || true)"
  [[ -n "$v" ]] || return 1
  local major minor; major="${v%%.*}"; minor="${v#*.}"; minor="${minor%%.*}"
  local nmajor nminor; nmajor="${BUN_VERSION%%.*}"; nminor="${BUN_VERSION#*.}"; nminor="${nminor%%.*}"
  (( major > nmajor )) || { (( major == nmajor )) && (( minor >= nminor )); }
}

download_go() {
  local url
  if [[ $USE_MIRROR -eq 1 ]]; then
    url="https://mirrors.aliyun.com/golang/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
  else
    url="https://go.dev/dl/go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
  fi
  c_info "下载 Go ${GO_VERSION}: $url"
  local tmp; tmp="$(mktemp -d)"
  curl -fL --retry 3 --retry-delay 2 -C - -o "$tmp/go.tgz" "$url" \
    || die "Go 下载失败，可手动下载后放到 $TOOLS_DIR/go"
  $SUDO mkdir -p "$TOOLS_DIR/go"
  $SUDO tar -C "$TOOLS_DIR/go" --strip-components=1 -xzf "$tmp/go.tgz"
  rm -rf "$tmp"
  "$(local_go)" version
}

download_bun() {
  local url
  if [[ $USE_MIRROR -eq 1 ]]; then
    url="https://cdn.npmmirror.com/binaries/bun/bun-v${BUN_VERSION}/bun-linux-${BUN_ARCH}.zip"
  else
    url="https://github.com/oven-sh/bun/releases/download/bun-v${BUN_VERSION}/bun-linux-${BUN_ARCH}.zip"
  fi
  c_info "下载 Bun ${BUN_VERSION}: $url"
  local tmp; tmp="$(mktemp -d)"
  local ok=0
  for i in 1 2 3 4 5; do
    curl -fL --retry 2 -C - -o "$tmp/bun.zip" "$url" || true
    if unzip -t "$tmp/bun.zip" >/dev/null 2>&1; then ok=1; break; fi
    c_warn "Bun 下载不完整，第 $i 次重试..."
    sleep 2
  done
  [[ $ok -eq 1 ]] || die "Bun 下载失败，可手动解压到 $TOOLS_DIR/bun"
  $SUDO mkdir -p "$TOOLS_DIR/bun/bin"
  unzip -o -q "$tmp/bun.zip" -d "$tmp/x"
  local bin; bin="$(find "$tmp/x" -type f -name bun | head -n 1)"
  [[ -n "$bin" ]] || die "解压后找不到 bun 可执行文件"
  $SUDO install -m 0755 "$bin" "$(local_bun)"
  rm -rf "$tmp"
  "$(local_bun)" --version
}

# ---------- 编译 ----------
if [[ $DO_BUILD -eq 1 ]]; then
  c_info "开始编译（前端 + 后端）..."

  have_go || download_go
  have_bun || download_bun

  export PATH="$(dirname "$(local_go)"):$(dirname "$(local_bun)"):$PATH"
  export GOTOOLCHAIN=local
  if [[ $USE_MIRROR -eq 1 ]]; then
    export GOPROXY="https://goproxy.cn,https://mirrors.aliyun.com/goproxy,direct"
  else
    export GOPROXY="https://proxy.golang.org,direct"
  fi

  # 前端
  if [[ -f "$INSTALL_DIR/web/package.json" ]]; then
    c_info "编译前端..."
    export BUN_INSTALL_CACHE_DIR="$TOOLS_DIR/bun-cache"
    # copyfile 后端规避部分文件系统（如 proot/overlay）不支持硬链接的问题
    ( cd "$INSTALL_DIR/web" && "$(local_bun)" install --backend=copyfile ) \
      || die "前端依赖安装失败"
    ( cd "$INSTALL_DIR/web" && "$(local_bun)" run build ) \
      || die "前端编译失败"
    [[ -f "$INSTALL_DIR/web/dist/index.html" ]] || die "前端产物缺失: web/dist/index.html"
    c_info "前端产物: $(du -sh "$INSTALL_DIR/web/dist" | cut -f1)"
  fi

  # 后端
  c_info "编译后端..."
  local_version="$(git -C "$INSTALL_DIR" describe --tags --always 2>/dev/null || echo "v0.0.0")"
  ( cd "$INSTALL_DIR" && "$(local_go)" build \
      -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=${local_version}'" \
      -o "$INSTALL_DIR/new-api" . ) || die "后端编译失败"
  c_info "后端产物: $(du -sh "$INSTALL_DIR/new-api" | cut -f1)"
elif [[ -n "$BINARY_SRC" ]]; then
  c_info "使用已有二进制: $BINARY_SRC"
  [[ -f "$BINARY_SRC" ]] || die "找不到二进制文件: $BINARY_SRC"
  $SUDO install -m 0755 "$BINARY_SRC" "$INSTALL_DIR/new-api"
fi

[[ -f "$INSTALL_DIR/new-api" ]] || die "缺少可执行文件 $INSTALL_DIR/new-api"
$SUDO chmod 0755 "$INSTALL_DIR/new-api"

# ---------- 环境配置 ----------
ENV_FILE="$INSTALL_DIR/.env"
if [[ ! -f "$ENV_FILE" ]]; then
  c_info "生成配置文件 .env"
  local_secret="$(head -c 32 /dev/urandom | base64 | tr -d '\n/+=' | head -c 32)"
  {
    echo "# New API 环境配置（由 install.sh 生成）"
    echo "PORT=$PORT"
    echo "TZ=Asia/Shanghai"
    echo "SESSION_SECRET=$local_secret"
    echo "ERROR_LOG_ENABLED=true"
    echo "BATCH_UPDATE_ENABLED=true"
    if [[ -n "$SQL_DSN" ]]; then
      echo "SQL_DSN=$SQL_DSN"
    elif [[ -n "$SQLITE_PATH" ]]; then
      echo "SQLITE_PATH=$SQLITE_PATH"
    else
      echo "# 未配置 SQL_DSN，默认使用 SQLite：$INSTALL_DIR/one-api.db"
      echo "# 使用宝塔 MySQL 时填：SQL_DSN=root:密码@tcp(127.0.0.1:3306)/new-api?parseTime=true"
    fi
    echo "# 多机部署时两台机器的 SESSION_SECRET 必须一致，NODE_TYPE=slave"
  } | $SUDO tee "$ENV_FILE" >/dev/null
  $SUDO chmod 600 "$ENV_FILE"
else
  c_info "已存在 .env，保留原配置"
fi

# ---------- systemd ----------
# systemctl 存在不代表能用：容器/CI 里 systemd 常常不是 PID 1，
# 此时 systemctl 会返回 "System has not been booted with systemd"。
has_systemd() {
  command -v systemctl >/dev/null 2>&1 || return 1
  [[ -d /run/systemd/system ]] || return 1
  systemctl is-system-running >/dev/null 2>&1 || \
    [[ "$(systemctl is-system-running 2>/dev/null)" =~ ^(running|degraded|initializing)$ ]]
}

if has_systemd; then
  c_info "注册 systemd 服务: $SERVICE_NAME"
  $SUDO tee "/etc/systemd/system/${SERVICE_NAME}.service" >/dev/null <<EOF
[Unit]
Description=New API
After=network.target

[Service]
Type=simple
User=${RUN_USER}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/new-api --port ${PORT} --log-dir ${LOG_DIR}
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
  $SUDO systemctl daemon-reload
  $SUDO systemctl enable "$SERVICE_NAME" >/dev/null 2>&1 || true
  $SUDO systemctl restart "$SERVICE_NAME"
  sleep 3
  if $SUDO systemctl is-active --quiet "$SERVICE_NAME"; then
    c_info "服务已启动"
  else
    c_err "服务启动失败，查看日志: journalctl -u $SERVICE_NAME -n 50 --no-pager"
  fi
else
  c_warn "未检测到 systemd，跳过服务注册"
  c_info "手动启动: cd $INSTALL_DIR && ./new-api --port $PORT --log-dir $LOG_DIR"
fi

# ---------- 宝塔防火墙提示 ----------
if [[ -d /www/server/panel ]]; then
  c_info "检测到宝塔面板"
  c_info "请在 面板 → 安全 中放行端口 $PORT（或改用反向代理）"
  c_info "建议在 网站 → 反向代理 指向 http://127.0.0.1:$PORT"
fi

# ---------- 完成 ----------
echo
c_info "安装完成"
echo "  访问地址: http://<服务器IP>:${PORT}"
echo "  安装目录: ${INSTALL_DIR}"
echo "  配置文件: ${ENV_FILE}"
echo "  日志目录: ${LOG_DIR}"
[[ $DO_BUILD -eq 1 ]] && echo "  工具链:   ${TOOLS_DIR}"
if has_systemd; then
  echo
  echo "  常用命令:"
  echo "    systemctl status ${SERVICE_NAME}"
  echo "    systemctl restart ${SERVICE_NAME}"
  echo "    journalctl -u ${SERVICE_NAME} -f"
fi
echo
c_info "首次访问会进入初始化向导，创建管理员账号"
