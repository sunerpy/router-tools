#!/bin/sh
set -eu

REPO="sunerpy/router-tools"
BIN="routerctl"
CHECKSUM_FILE="checksums.txt"

err() {
	printf 'error: %s\n' "$1" >&2
	exit 1
}

info() {
	printf '%s\n' "$1" >&2
}

if command -v curl >/dev/null 2>&1; then
	download() { curl -fsSL "$1" -o "$2"; }
	fetch() { curl -fsSL "$1"; }
elif command -v wget >/dev/null 2>&1; then
	download() { wget -qO "$2" "$1"; }
	fetch() { wget -qO - "$1"; }
else
	err "curl or wget is required"
fi

command -v tar >/dev/null 2>&1 || err "tar is required"

case "$(uname -s)" in
Linux) os_part="linux" ;;
Darwin) os_part="darwin" ;;
*) err "unsupported OS: $(uname -s)" ;;
esac

case "$(uname -m)" in
x86_64 | amd64) arch_part="amd64" ;;
arm64 | aarch64) arch_part="arm64" ;;
*) err "unsupported architecture: $(uname -m)" ;;
esac

if [ -n "${TOOL_VERSION:-}" ]; then
	version=$(printf '%s' "$TOOL_VERSION" | sed 's/^v//')
else
	info "Resolving latest release..."
	api="https://api.github.com/repos/${REPO}/releases/latest"
	tag=$(fetch "$api" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
	[ -n "$tag" ] || err "could not resolve latest release"
	version=$(printf '%s' "$tag" | sed 's/^v//')
fi

asset="${BIN}_${version}_${os_part}_${arch_part}.tar.gz"
base_url="https://github.com/${REPO}/releases/download/v${version}"
install_dir="${TOOL_INSTALL_DIR:-$HOME/.local/bin}"

tmp=$(mktemp -d 2>/dev/null || mktemp -d -t "$BIN")
trap 'rm -rf "$tmp"' EXIT INT TERM

info "Downloading ${asset}"
download "${base_url}/${asset}" "$tmp/$asset"
download "${base_url}/${CHECKSUM_FILE}" "$tmp/$CHECKSUM_FILE"

expected=$(awk -v name="$asset" '{
	file=$2
	sub(/^\*/, "", file)
	if (file == name) { print $1; exit }
}' "$tmp/$CHECKSUM_FILE")
[ -n "$expected" ] || err "${asset} is missing from ${CHECKSUM_FILE}"

if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$tmp/$asset" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
else
	err "sha256sum or shasum is required"
fi
[ "$actual" = "$expected" ] || err "checksum mismatch for ${asset}"

tar -xzf "$tmp/$asset" -C "$tmp"
mkdir -p "$install_dir"
mv "$tmp/$BIN" "$install_dir/$BIN"
chmod +x "$install_dir/$BIN"

info "Installed ${BIN} to ${install_dir}/${BIN}"
case ":$PATH:" in
*":$install_dir:"*) ;;
*) info "Add ${install_dir} to PATH" ;;
esac
