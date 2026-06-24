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

# 설치 대상이 ~/.local/bin 이 아니면, 거기 남아 있는 옛 ecs 를 정리한다.
# (PATH에서 ~/.local/bin 이 먼저 잡혀 방금 설치한 바이너리를 가리는 문제 방지)
STALE="$HOME/.local/bin/$BINARY"
if [ "$TARGET" != "$STALE" ] && [ -e "$STALE" ]; then
	echo "==> removing stale $STALE (PATH 섀도잉 방지)"
	rm -f "$STALE"
fi

echo "==> installed: $TARGET"

# PATH에서 실제로 잡히는 ecs 가 방금 설치한 것과 다르면 경고한다.
RESOLVED="$(command -v "$BINARY" 2>/dev/null || true)"
if [ -n "$RESOLVED" ] && [ "$RESOLVED" != "$TARGET" ]; then
	echo "!! 경고: PATH에서 '$RESOLVED' 가 먼저 잡힙니다. 방금 설치한 '$TARGET' 가 가려집니다."
	echo "!!       해당 파일을 지우거나 PATH 순서를 조정하세요."
fi

"$TARGET" --version 2>/dev/null || true
