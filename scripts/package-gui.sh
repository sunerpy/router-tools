#!/usr/bin/env bash
# Build and package the Router Tools desktop GUI for one release platform.
#
# Usage:
#   scripts/package-gui.sh (windows|darwin) [--version X.Y.Z] [--out DIR]
#
# windows  cross-compiled on Linux or macOS; needs wails, pnpm, node, makensis, 7z or 7zz, zip, unzip
#   router-tools_<v>_windows_amd64_setup.exe     NSIS installer
#   router-tools_<v>_windows_amd64_portable.zip  "Router Tools.exe", LICENSE, THIRD_PARTY_NOTICES.md
# darwin   native on macOS; needs wails, pnpm, node, ditto, lipo, plutil, unzip
#   router-tools_<v>_darwin_universal.zip        "Router Tools.app" (x86_64 + arm64), licence files
#
# <v> is cmd/routergui/wails.json info.productVersion, which release-please updates. With
# --version the script fails unless that value equals the release version (release builds);
# without it the wails.json value is used as-is (CI smoke). Asset names never contain spaces
# because GitHub rewrites spaces in release asset names. The script writes Wails outputs to
# cmd/routergui/build/bin and the packaged assets to DIR (default: dist-gui).
set -euo pipefail

usage() {
	sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'
	exit 2
}

die() {
	printf 'package-gui: %s\n' "$1" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || die "$1 is required: $2"
}

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

[ "$#" -ge 1 ] || usage
platform=$1
shift
release_version=""
out_dir=""
while [ "$#" -gt 0 ]; do
	case "$1" in
	--version)
		[ "$#" -ge 2 ] || usage
		release_version=${2#v}
		shift 2
		;;
	--out)
		[ "$#" -ge 2 ] || usage
		out_dir=$2
		shift 2
		;;
	*) usage ;;
	esac
done
case "$platform" in
windows | darwin) ;;
*) usage ;;
esac

root=$(cd "$(dirname "$0")/.." && pwd)
project="$root/cmd/routergui"
bin_dir="$project/build/bin"

need go "install Go 1.27 (https://go.dev/dl)"
need node "install Node.js 24 (https://nodejs.org)"
need pnpm "corepack enable, or npm install --global pnpm@9.15.9"
need wails "go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1"
need unzip "install unzip"
if [ "$platform" = windows ]; then
	need makensis "install NSIS (apt-get install nsis, brew install makensis)"
	sevenzip=$(command -v 7z || command -v 7zz || true)
	[ -n "$sevenzip" ] || die "7z or 7zz is required: install 7-Zip (apt-get install p7zip-full, brew install sevenzip)"
	need zip "install zip"
else
	[ "$(uname -s)" = Darwin ] || die "darwin packages must be built on macOS"
	need ditto "run on macOS"
	need lipo "install the Xcode command line tools"
	need plutil "run on macOS"
fi

version=$(node -e 'const j = require(process.argv[1]); process.stdout.write(String((j.info && j.info.productVersion) || ""))' "$project/wails.json")
printf '%s' "$version" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$' ||
	die "wails.json info.productVersion is not a SemVer version: '$version'"
if [ -n "$release_version" ] && [ "$version" != "$release_version" ]; then
	die "wails.json info.productVersion is $version but the release version is $release_version"
fi

# The desktop binary carries the same build metadata as routerctl (internal/buildinfo): its MCP service
# reports this version, and backup manifests created by the window or by an agent record it.
buildinfo=github.com/sunerpy/router-tools/internal/buildinfo
commit=$(git -C "$root" rev-parse --short HEAD 2>/dev/null || echo unknown)
build_time=$(date -u +%Y-%m-%dT%H:%M:%SZ)
ldflags="-X $buildinfo.Version=v$version -X $buildinfo.CommitID=$commit -X $buildinfo.BuildTime=$build_time"

# check_buildinfo BINARY fails unless the Go build info of BINARY records the injected version. It reads the
# -ldflags that Go records in the build info, which Go omits under -trimpath; adding -trimpath to the builds
# below needs another check.
check_buildinfo() {
	local info recorded
	info=$(go version -m "$1") || die "go version -m cannot read $1"
	recorded=$(printf '%s\n' "$info" | sed -n 's/.*buildinfo\.Version=\([^ "]*\).*/\1/p')
	[ "$recorded" = "v$version" ] || die "$1 records buildinfo version '${recorded:-none}', expected v$version"
}

out_dir=${out_dir:-$root/dist-gui}
mkdir -p "$out_dir"
out_dir=$(cd "$out_dir" && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

licence_files=("$root/LICENSE" "$root/THIRD_PARTY_NOTICES.md")
for file in "${licence_files[@]}"; do
	[ -f "$file" ] || die "missing $file"
done

if [ "$platform" = windows ]; then
	exe="$bin_dir/routergui.exe"
	installer="$bin_dir/Router Tools-amd64-installer.exe"
	rm -f "$exe" "$installer"
	(cd "$project" && wails build -platform windows/amd64 -nsis -skipbindings -ldflags "$ldflags" -o routergui.exe)
	[ -f "$exe" ] || die "wails did not produce $exe"
	[ -f "$installer" ] || die "wails did not produce $installer"
	check_buildinfo "$exe"

	# The installer must carry exactly the executable this run built.
	"$sevenzip" x -y -o"$work/installer" "$installer" "Router Tools.exe" >/dev/null
	[ -f "$work/installer/Router Tools.exe" ] || die "installer does not contain Router Tools.exe"
	[ "$(sha256 "$work/installer/Router Tools.exe")" = "$(sha256 "$exe")" ] ||
		die "installer executable differs from the built executable"

	setup_name="router-tools_${version}_windows_amd64_setup.exe"
	portable_name="router-tools_${version}_windows_amd64_portable.zip"
	rm -f "$out_dir/$setup_name" "$out_dir/$portable_name"
	cp "$installer" "$out_dir/$setup_name"
	mkdir "$work/portable"
	cp "$exe" "$work/portable/Router Tools.exe"
	cp "${licence_files[@]}" "$work/portable/"
	(cd "$work/portable" && zip -q -X "$out_dir/$portable_name" "Router Tools.exe" LICENSE THIRD_PARTY_NOTICES.md)
	unzip -Z1 "$out_dir/$portable_name" | grep -qx "Router Tools.exe" || die "portable zip lacks Router Tools.exe"
	produced=("$setup_name" "$portable_name")
else
	app="$bin_dir/Router Tools.app"
	rm -rf "$app"
	(cd "$project" && wails build -platform darwin/universal -skipbindings -ldflags "$ldflags")
	[ -d "$app" ] || die "wails did not produce $app"

	plist="$app/Contents/Info.plist"
	executable=$(plutil -extract CFBundleExecutable raw -o - "$plist")
	bundle_version=$(plutil -extract CFBundleShortVersionString raw -o - "$plist")
	[ "$bundle_version" = "$version" ] || die "bundle version $bundle_version differs from $version"
	archs=$(lipo -archs "$app/Contents/MacOS/$executable")
	case " $archs " in *" x86_64 "*) ;; *) die "app lacks x86_64 ($archs)" ;; esac
	case " $archs " in *" arm64 "*) ;; *) die "app lacks arm64 ($archs)" ;; esac
	for arch in x86_64 arm64; do
		lipo -thin "$arch" -output "$work/$executable-$arch" "$app/Contents/MacOS/$executable"
		check_buildinfo "$work/$executable-$arch"
	done

	zip_name="router-tools_${version}_darwin_universal.zip"
	rm -f "$out_dir/$zip_name"
	mkdir "$work/stage"
	ditto "$app" "$work/stage/Router Tools.app"
	cp "${licence_files[@]}" "$work/stage/"
	ditto -c -k --sequesterRsrc "$work/stage" "$out_dir/$zip_name"
	unzip -Z1 "$out_dir/$zip_name" | grep -qx "Router Tools.app/Contents/Info.plist" ||
		die "zip lacks Router Tools.app/Contents/Info.plist"
	produced=("$zip_name")
fi

for name in "${produced[@]}"; do
	printf '%s  %s\n' "$(sha256 "$out_dir/$name")" "$name"
done
