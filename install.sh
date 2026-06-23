#!/usr/bin/env bash
# ecs CLI를 빌드해 /usr/local/bin/ecs 로 설치한다.
set -euo pipefail

cd "$(dirname "$0")"

BINARY="ecs"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
TARGET="$INSTALL_DIR/$BINARY"

# git describe 로 버전을 만들어 바이너리에 주입 (git 정보 없으면 dev)
VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"

echo "==> building $BINARY ($VERSION)"
go build -ldflags "-X github.com/heebin2/ecsctl/internal/cli.version=$VERSION" -o "$BINARY" ./cmd/ecs

echo "==> installing to $TARGET"
if [ -w "$INSTALL_DIR" ]; then
	install -m 0755 "$BINARY" "$TARGET"
else
	# /usr/local/bin 쓰기 권한이 없으면 sudo 로 설치
	sudo install -m 0755 "$BINARY" "$TARGET"
fi

rm -f "$BINARY"
echo "==> installed: $(command -v $BINARY)"
"$TARGET" --version 2>/dev/null || true
