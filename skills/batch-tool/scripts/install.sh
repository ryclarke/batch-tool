#!/bin/sh
# Install, update, or check batch-tool from its GitHub releases.
#
# Usage: install.sh [--version vX.Y.Z] [--dir DIR] [--check] [--force]
#
#   --version TAG  install a specific release tag (default: latest release)
#   --dir DIR      install directory (default: $BATCH_TOOL_INSTALL_DIR or ~/.local/bin)
#   --check        report installed vs latest version and exit without changes
#   --force        reinstall even when the requested version is already installed
#
# --check prints one line whose first word is machine-readable:
#   up-to-date <version> <path>
#   outdated <installed> -> <latest> <path>
#   not-installed -> <latest>
#
# Release archives are verified against the release's SHA-256 checksum file
# before installation. Never uses sudo and never edits shell startup files.

set -eu

REPO="ryclarke/batch-tool"
MODULE="github.com/$REPO"
RELEASES="https://github.com/$REPO/releases"
BIN="batch-tool"

tag=""
dir="${BATCH_TOOL_INSTALL_DIR:-${HOME:?HOME is not set}/.local/bin}"
check=0
force=0
tmp=""

log() { printf '%s\n' "$*" >&2; }
die() { log "error: $*"; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

usage() {
	sed -n '2,17s/^# \{0,1\}//p' "$0"
}

cleanup() { if [ -n "$tmp" ]; then rm -rf "$tmp"; fi; }
trap cleanup EXIT
trap 'exit 130' INT TERM

while [ $# -gt 0 ]; do
	case $1 in
	--version) [ $# -ge 2 ] || die "--version requires a tag"; tag=$2; shift 2 ;;
	--version=*) tag=${1#*=}; shift ;;
	--dir) [ $# -ge 2 ] || die "--dir requires a path"; dir=$2; shift 2 ;;
	--dir=*) dir=${1#*=}; shift ;;
	--check) check=1; shift ;;
	--force) force=1; shift ;;
	-h | --help) usage; exit 0 ;;
	*) usage >&2; die "unknown argument: $1" ;;
	esac
done

case $tag in
"" | v*) ;;
*) tag="v$tag" ;;
esac

# fetch URL OUTPUT downloads over HTTPS only.
fetch() {
	if have curl; then
		curl -fsSL --proto '=https' --tlsv1.2 -o "$2" "$1"
	elif have wget; then
		wget -q --https-only -O "$2" "$1"
	else
		return 1
	fi
}

# latest_tag resolves the latest release from the /releases/latest redirect,
# which avoids the rate-limited GitHub API.
latest_tag() {
	url=""
	if have curl; then
		url=$(curl -fsSLI --proto '=https' -o /dev/null -w '%{url_effective}' "$RELEASES/latest") || return 1
	elif have wget; then
		url=$(wget --https-only -S --spider "$RELEASES/latest" 2>&1 |
			sed -n 's/^ *[Ll]ocation: *//p' | tail -n 1 | tr -d '\r') || return 1
	else
		return 1
	fi

	latest=${url##*/}
	case $latest in
	v[0-9]*) printf '%s\n' "$latest" ;;
	*) return 1 ;;
	esac
}

# installed_path prints the batch-tool binary that would run, preferring PATH.
installed_path() {
	if have "$BIN"; then
		command -v "$BIN"
	elif [ -x "$dir/$BIN" ]; then
		printf '%s\n' "$dir/$BIN"
	else
		return 1
	fi
}

# installed_version prints the version of the given binary. Builds from
# `go install` report "dev", so fall back to the module version Go embeds.
installed_version() {
	ver=$("$1" --version 2>/dev/null | sed -n 's/^batch-tool version //p')
	if { [ -z "$ver" ] || [ "$ver" = dev ]; } && have go; then
		ver=$(go version -m "$1" 2>/dev/null |
			awk -v m="$MODULE" '$1 == "mod" && $2 == m { print $3 }')
	fi

	case $ver in
	"" | dev) printf 'unknown\n' ;;
	v*) printf '%s\n' "$ver" ;;
	*) printf 'v%s\n' "$ver" ;;
	esac
}

# vercmp A B prints -1, 0, or 1 comparing two semantic versions. Unparseable
# versions compare as older than anything.
vercmp() {
	awk -v a="${1#v}" -v b="${2#v}" '
	function core(v) { sub(/[-+].*/, "", v); return v }
	function pre(v) { sub(/\+.*/, "", v); return index(v, "-") ? substr(v, index(v, "-") + 1) : "" }
	function valid(v) { return v ~ /^[0-9]+\.[0-9]+\.[0-9]+([-+].*)?$/ }
	BEGIN {
		if (!valid(a) || !valid(b)) { print (valid(a) ? 1 : (valid(b) ? -1 : 0)); exit }
		split(core(a), x, "."); split(core(b), y, ".")
		for (i = 1; i <= 3; i++) {
			if (x[i] + 0 < y[i] + 0) { print -1; exit }
			if (x[i] + 0 > y[i] + 0) { print 1; exit }
		}
		pa = pre(a); pb = pre(b)
		if (pa == pb) print 0
		else if (pa == "") print 1
		else if (pb == "") print -1
		else print (pa < pb) ? -1 : 1
	}'
}

sha256() {
	if have sha256sum; then
		sha256sum "$1" | awk '{ print $1 }'
	elif have shasum; then
		shasum -a 256 "$1" | awk '{ print $1 }'
	else
		return 1
	fi
}

can_extract_xz() {
	have xz || tar --version 2>/dev/null | grep -qi bsdtar
}

platform() {
	case $(uname -s) in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	MINGW* | MSYS* | CYGWIN*) os=windows ;;
	*) os=unsupported ;;
	esac

	case $(uname -m) in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) arch=unsupported ;;
	esac
}

manual_steps() {
	log ""
	log "Install batch-tool manually:"
	log "  1. Download the archive for your platform from $RELEASES/latest"
	log "  2. Verify it against the release's batch-tool_<version>_checksums.txt"
	log "  3. Extract it and move the batch-tool binary into a directory on your PATH"
	log "Or, with Go installed: go install $MODULE@latest"
}

# install_release TAG downloads, verifies, and installs a release archive.
# Returns 2 when a prerequisite is missing so the caller can fall back to Go.
# A checksum mismatch is fatal and never falls back.
install_release() {
	platform
	if [ "$os" = unsupported ] || [ "$os" = windows ] || [ "$arch" = unsupported ]; then
		log "no prebuilt archive handled by this script for $(uname -s)/$(uname -m)"
		return 2
	fi

	if ! have curl && ! have wget; then
		log "neither curl nor wget is available"
		return 2
	fi

	if ! can_extract_xz; then
		log "tar cannot extract .txz archives here (install xz)"
		return 2
	fi

	if ! have sha256sum && ! have shasum; then
		log "no sha256sum or shasum available to verify the download"
		return 2
	fi

	asset="${BIN}_${os}_${arch}.txz"
	sums="${BIN}_${1#v}_checksums.txt"
	tmp=$(mktemp -d 2>/dev/null || mktemp -d -t batch-tool)

	log "downloading $asset ($1)"
	if ! fetch "$RELEASES/download/$1/$asset" "$tmp/$asset"; then
		log "download failed: $RELEASES/download/$1/$asset"
		return 2
	fi
	fetch "$RELEASES/download/$1/$sums" "$tmp/$sums" || die "could not download $sums for $1"

	expected=$(awk -v f="$asset" '$2 == f || $2 == "*" f { print $1 }' "$tmp/$sums")
	[ -n "$expected" ] || die "$sums has no entry for $asset"
	actual=$(sha256 "$tmp/$asset")
	[ "$expected" = "$actual" ] || die "checksum mismatch for $asset: expected $expected, got $actual"
	log "checksum verified (sha256:$actual)"

	(cd "$tmp" && tar -xf "$asset") || die "failed to extract $asset"
	[ -f "$tmp/$BIN" ] || die "$asset does not contain $BIN"

	staged="$dir/.$BIN.new.$$"
	mkdir -p "$dir" || die "cannot create $dir"
	cp "$tmp/$BIN" "$staged" || die "cannot write to $dir"
	chmod 755 "$staged" || die "cannot mark $staged executable"
	mv -f "$staged" "$dir/$BIN" || die "cannot replace $dir/$BIN"
}

# install_go TAG builds and installs with the Go toolchain, which verifies
# module contents against the Go checksum database.
install_go() {
	have go || return 1
	log "installing with: GOBIN=$dir go install $MODULE@$1"
	mkdir -p "$dir" || die "cannot create $dir"
	GOBIN=$dir go install "$MODULE@$1"
}

path_advice() {
	case ":$PATH:" in
	*":$dir:"*) ;;
	*)
		log ""
		log "note: $dir is not on your PATH. Add this line to your shell profile:"
		log "  export PATH=\"$dir:\$PATH\""
		;;
	esac

	active=$(command -v "$BIN" 2>/dev/null || true)
	if [ -n "$active" ] && [ "$active" != "$dir/$BIN" ]; then
		log ""
		log "warning: $active comes earlier on PATH and shadows $dir/$BIN"
	fi
}

latest=""
if [ -z "$tag" ] || [ "$check" -eq 1 ]; then
	latest=$(latest_tag) || latest=""
fi

current_path=$(installed_path) || current_path=""
current=""
if [ -n "$current_path" ]; then
	current=$(installed_version "$current_path")
fi

if [ "$check" -eq 1 ]; then
	target=${tag:-$latest}
	[ -n "$target" ] || die "could not determine the latest release from $RELEASES/latest"

	if [ -z "$current_path" ]; then
		printf 'not-installed -> %s\n' "$target"
	elif [ "$(vercmp "$current" "$target")" -ge 0 ]; then
		printf 'up-to-date %s %s\n' "$current" "$current_path"
	else
		printf 'outdated %s -> %s %s\n' "$current" "$target" "$current_path"
	fi
	exit 0
fi

if [ -z "$tag" ]; then
	tag=$latest
fi

if [ -n "$tag" ] && [ "$force" -eq 0 ] && [ -x "$dir/$BIN" ]; then
	existing=$(installed_version "$dir/$BIN")
	if [ "$(vercmp "$existing" "$tag")" -eq 0 ]; then
		log "batch-tool $existing is already installed at $dir/$BIN (use --force to reinstall)"
		path_advice
		exit 0
	fi
fi

if [ -z "$tag" ]; then
	log "could not determine the latest release from $RELEASES/latest"
	install_go latest || { manual_steps; exit 1; }
else
	status=0
	install_release "$tag" || status=$?
	if [ "$status" -ne 0 ]; then
		log "falling back to the Go toolchain"
		install_go "$tag" || { manual_steps; exit 1; }
	fi
fi

log "installed batch-tool $(installed_version "$dir/$BIN") to $dir/$BIN"
path_advice
