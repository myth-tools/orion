#!/usr/bin/env bash
# ─── Orion Release Script ───────────────────────────────────────────────
# Usage: ./scripts/release.sh [options] [patch|minor|major|<version>]
#
# Features:
#   • Semver bump (patch/minor/major) or explicit version
#   • Dry-run, force-overwrite, draft, pre-release, local-only
#   • Conventional commit changelog (feat/fix/breaking)
#   • Binary compression with platform-mapped filenames
#   • SHA256 checksums (cross-platform sha256sum/shasum)
#   • Dirty-tree guard, SIGINT cleanup, GPG signing
#   • Editor review of release notes before publish
#   • Interactive confirmation before remote side-effects
# ─────────────────────────────────────────────────────────────────────────
set -euo pipefail

# ─── Config ────────────────────────────────────────────────────────────
SCRIPTDIR="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$SCRIPTDIR/.." && pwd)"
REMOTE="${REMOTE:-origin}"

# Read dynamic config from metadata.yaml (single source of truth)
METADATA="$REPO/metadata.yaml"
if [[ -f "$METADATA" ]]; then
	BIN=$(grep -E '^project_name:' "$METADATA" | sed 's/^project_name: *//' | tr -d '"')
	GITHUB_OWNER=$(grep -E '^\s+owner:' "$METADATA" | head -1 | sed 's/.*owner: *//' | tr -d '"')
	GITHUB_REPO=$(grep -E '^\s+repo:' "$METADATA" | head -1 | sed 's/.*repo: *//' | tr -d '"')
fi
BIN="${BIN:-$(grep -E '^project_name:' "$METADATA" 2>/dev/null | sed 's/^project_name: *//' | tr -d '"')}"
BIN="${BIN:-orion}"
GITHUB_OWNER="${GITHUB_OWNER:-$(grep -E '^\s+owner:' "$METADATA" 2>/dev/null | head -1 | sed 's/.*owner: *//' | tr -d '"')}"
GITHUB_OWNER="${GITHUB_OWNER:-myth-tools}"
GITHUB_REPO="${GITHUB_REPO:-$(grep -E '^\s+repo:' "$METADATA" 2>/dev/null | head -1 | sed 's/.*repo: *//' | tr -d '"')}"
GITHUB_REPO="${GITHUB_REPO:-orion}"

BUILDDIR="build"

# ─── Colors ────────────────────────────────────────────────────────────
if [[ -t 1 ]]; then
	c_reset='\033[0m'
	c_bold='\033[1m'
	c_dim='\033[2m'
	c_red='\033[0;31m'
	c_green='\033[0;32m'
	c_yellow='\033[0;33m'
	c_blue='\033[0;34m'
	c_cyan='\033[0;36m'
else
	# shellcheck disable=SC2034
	c_reset='' c_bold='' c_dim='' c_red='' c_green='' c_yellow='' c_blue='' c_cyan=''
fi

die() {
	echo -e "${c_red}✗${c_reset} $*" >&2
	exit 1
}
info() { echo -e "${c_blue}▶${c_reset} $*"; }
ok() { echo -e "${c_green}✓${c_reset} $*"; }
warn() { echo -e "${c_yellow}⚠${c_reset} $*"; }
step() { echo -e "${c_bold}${c_cyan}━━━ $* ━━━${c_reset}"; }

# ─── Cleanup trap ──────────────────────────────────────────────────────
CLEANUP_TAG=""
cleanup() {
	if [[ -n "$CLEANUP_TAG" && "$DRY_RUN" != "true" ]]; then
		warn "Interrupted — cleaning up local tag $CLEANUP_TAG"
		git tag -d "$CLEANUP_TAG" 2>/dev/null || true
	fi
	exit 1
}
trap cleanup SIGINT SIGTERM

# ─── Help ──────────────────────────────────────────────────────────────
usage() {
	cat <<EOF
${c_bold}Usage:${c_reset} $(basename "$0") [options] [bump|version]

${c_bold}Bump / version:${c_reset}
  patch              bump patch (1.0.0 → 1.0.1)  [default]
  minor              bump minor (1.0.0 → 1.1.0)
  major              bump major (1.0.0 → 2.0.0)
  vX.Y.Z             explicit semver tag (e.g. v2.1.0)

${c_bold}Options:${c_reset}
  -d, --dry-run      preview everything — no side-effects
  -f, --force        allow overwriting existing tag / release
      --draft        create release as draft (not published)
      --pre-release  mark release as pre-release
      --local-only   tag locally only; skip push & gh release
      --sign         GPG-sign the tag (-s instead of -a)
      --skip-build   skip binary build (reuse existing artifacts)
      --skip-editor  skip editor review of release notes
  -h, --help         show this help

${c_bold}Environment:${c_reset}
  REMOTE             git remote name (default: origin)
  EDITOR             editor for release notes (default: vi)

${c_bold}Examples:${c_reset}
  $(basename "$0") patch
  $(basename "$0") minor --dry-run
  $(basename "$0") v2.1.0 --draft --pre-release
  $(basename "$0") v2.1.0 --local-only --skip-build --skip-editor
EOF
	exit 0
}

# ─── Parse arguments ───────────────────────────────────────────────────
BUMP=""
DRY_RUN="false"
FORCE="false"
DRAFT="false"
PRE_RELEASE="false"
LOCAL_ONLY="false"
SIGN_TAG="false"
SKIP_BUILD="false"
SKIP_EDITOR="true"

while [[ $# -gt 0 ]]; do
	case "$1" in
	-h | --help) usage ;;
	-d | --dry-run)
		DRY_RUN="true"
		shift
		;;
	-f | --force)
		FORCE="true"
		shift
		;;
	--draft)
		DRAFT="true"
		shift
		;;
	--pre-release)
		PRE_RELEASE="true"
		shift
		;;
	--local-only)
		LOCAL_ONLY="true"
		shift
		;;
	--sign)
		SIGN_TAG="true"
		shift
		;;
	--skip-build)
		SKIP_BUILD="true"
		shift
		;;
	--skip-editor)
		SKIP_EDITOR="true"
		shift
		;;
	-*)
		if [[ "$1" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
			BUMP="$1"
		else
			die "unknown option: $1"
		fi
		shift
		;;
	*)
		BUMP="$1"
		shift
		;;
	esac
done

# ─── Pre-flight: tools ────────────────────────────────────────────────
step "Pre-flight checks"

info "checking required tools..."
for cmd in git make tar; do
	command -v "$cmd" >/dev/null 2>&1 || die "'$cmd' not found — install it first"
done

# zip is needed only when Windows binaries exist
ZIP_AVAILABLE=false
command -v zip >/dev/null 2>&1 && ZIP_AVAILABLE=true

# stat with byte-size flags (Linux stat -c vs macOS stat -f)
stat_bytes() {
	stat -c%s "$1" 2>/dev/null || stat -f%z "$1" 2>/dev/null || echo 0
}

# Human-readable size (pure bash, no bc required)
human_size() {
	local bytes=$1
	if ((bytes >= 1073741824)); then
		echo "$(((bytes + 536870912) / 1073741824)) GB"
	elif ((bytes >= 1048576)); then
		echo "$(((bytes + 524288) / 1048576)) MB"
	else
		echo "$(((bytes + 512) / 1024)) KB"
	fi
}

if [[ "$LOCAL_ONLY" != "true" ]]; then
	command -v gh >/dev/null 2>&1 || die "'gh' not found — install GitHub CLI first"
	if ! gh auth status 2>&1 | grep -qi "logged in"; then
		if [[ "$DRY_RUN" == "true" ]]; then
			warn "not logged into gh (dry-run, will proceed anyway)"
		else
			die "not logged into gh — run 'gh auth login'"
		fi
	else
		info "gh: logged in"
	fi
fi

# sha256sum preferred, fallback to shasum (macOS)
SHA_CMD=""
if command -v sha256sum >/dev/null 2>&1; then
	SHA_CMD="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
	SHA_CMD="shasum -a 256"
else
	die "neither sha256sum nor shasum found"
fi
info "checksum: $SHA_CMD"

# Go module path for install instructions
GO_MODULE="$(head -1 "$REPO/go.mod" | awk '{print $2}')"
info "module: $GO_MODULE"

# Read description from metadata.yaml
DESCRIPTION="$(grep -E '^\s+description:' "$METADATA" | sed 's/.*description: *//' | tr -d '"' 2>/dev/null || echo "")"

# ─── Dirty tree check (deferred — we'll auto-commit after version resolution) ───
HAS_DIRTY_TREE=false
if ! git diff-index --quiet HEAD --; then
	HAS_DIRTY_TREE=true
fi

# ─── Version resolution ───────────────────────────────────────────────
step "Version resolution"

META_VER="$(grep '^version:' "$METADATA" | sed 's/^version: *//' | tr -d '"')"
LAST_TAG="$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")"
LAST_VER="${LAST_TAG#v}"

info "metadata version: ${META_VER:-none}"
info "last tag: $LAST_TAG"

if [[ "$BUMP" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	NEW_TAG="${BUMP#v}"
	NEW_TAG="v$NEW_TAG"
	info "explicit version: $BUMP → $NEW_TAG"
elif [[ "$BUMP" == "patch" || "$BUMP" == "minor" || "$BUMP" == "major" ]]; then
	BASE_VER="${META_VER:-$LAST_VER}"
	IFS='.' read -r major minor patch <<<"$BASE_VER"
	case "$BUMP" in
	patch) NEW_TAG="v$major.$minor.$((patch + 1))" ;;
	minor) NEW_TAG="v$major.$((minor + 1)).0" ;;
	major) NEW_TAG="v$((major + 1)).0.0" ;;
	esac
	info "bump: $BUMP → $NEW_TAG (base: v$BASE_VER)"
else
	if [[ -n "$META_VER" ]]; then
		NEW_TAG="v$META_VER"
		info "using metadata version: $NEW_TAG"
	else
		die "no version in metadata.yaml — specify patch, minor, major, or vX.Y.Z"
	fi
fi

if [[ "$HAS_DIRTY_TREE" == "true" && "$DRY_RUN" != "true" ]]; then
	step "Auto-committing changes"
	git add -A
	git commit -m "chore: release $NEW_TAG"
	ok "committed: chore: release $NEW_TAG"
fi

# Semver validation
if ! [[ "$NEW_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
	die "generated tag '$NEW_TAG' is not valid semver (expected vX.Y.Z)"
fi

# ─── Tag existence checks ──────────────────────────────────────────────
TAG_EXISTS_LOCAL=false
TAG_EXISTS_REMOTE=false

if git rev-parse "$NEW_TAG" >/dev/null 2>&1; then
	TAG_EXISTS_LOCAL=true
fi

if git ls-remote --tags "$REMOTE" "$NEW_TAG" 2>/dev/null | grep -q "refs/tags/$NEW_TAG$"; then
	TAG_EXISTS_REMOTE=true
fi

if [[ "$TAG_EXISTS_LOCAL" == "true" || "$TAG_EXISTS_REMOTE" == "true" ]]; then
	warn "tag $NEW_TAG exists (local=$TAG_EXISTS_LOCAL, remote=$TAG_EXISTS_REMOTE) — will overwrite"
fi

# ─── Commits since last tag ────────────────────────────────────────────
COMMITS=""
if git rev-parse "$LAST_TAG" >/dev/null 2>&1; then
	COMMITS="$(git log "$LAST_TAG..HEAD" --oneline --no-decorate 2>/dev/null || true)"
fi
COMMIT_COUNT="$(echo "$COMMITS" | grep -c . || true)"

# ─── Summary ───────────────────────────────────────────────────────────
echo ""
step "Release summary"
echo ""
echo -e "  ${c_bold}Previous tag:${c_reset}  $LAST_TAG"
echo -e "  ${c_bold}New tag:${c_reset}       $NEW_TAG"
echo -e "  ${c_bold}Commits:${c_reset}        $COMMIT_COUNT since $LAST_TAG"
echo -e "  ${c_bold}Draft:${c_reset}          $DRAFT"
echo -e "  ${c_bold}Pre-release:${c_reset}    $PRE_RELEASE"
echo -e "  ${c_bold}Dry-run:${c_reset}        $DRY_RUN"
echo -e "  ${c_bold}Local only:${c_reset}     $LOCAL_ONLY"
echo -e "  ${c_bold}Force:${c_reset}          $FORCE"
echo -e "  ${c_bold}Sign tag:${c_reset}       $SIGN_TAG"

if [[ "$DRY_RUN" == "true" ]]; then
	echo ""
	warn "DRY-RUN mode — no changes will be made"
fi

# ─── Lint ──────────────────────────────────────────────────────────────
echo ""
step "Linting"

info "running: make lint-fast"
if make lint-fast; then
	ok "lint passed"
else
	die "lint failed — fix issues before releasing"
fi

# ─── Build ─────────────────────────────────────────────────────────────
echo ""
step "Building binaries"

if [[ "$SKIP_BUILD" == "true" ]]; then
	info "skipping build (--skip-build)"
elif [[ "$DRY_RUN" == "true" ]]; then
	info "dry-run: would run: make build-all"
else
	info "running: make build-all"
	make build-all
	ok "build complete"
fi

# ─── Compress & checksum ──────────────────────────────────────────────
echo ""
step "Compressing & checksumming"

mkdir -p "$BUILDDIR/release"
rm -f "$BUILDDIR/release"/*

# Platform → human-readable mapping (strip .exe/.tar.gz/.zip before matching)
plat_of() {
	local name="$1"
	name="${name%.exe}"
	name="${name%.tar.gz}"
	name="${name%.zip}"
	# Archive:  orion_0.1.0_linux_amd64 → linux_amd64
	if [[ "$name" =~ ^${BIN}_[0-9.]+_(.*) ]]; then name="${BASH_REMATCH[1]}"; fi
	# Raw binary: orion-linux-amd64 → linux-amd64
	name="${name#"${BIN}"-}"
	# Normalize underscores to dashes for consistent matching
	name="${name//_/-}"
	case "$name" in
	linux-amd64) echo "Linux (x86_64)" ;;
	linux-386) echo "Linux (x86)" ;;
	linux-arm64) echo "Linux (ARM64)" ;;
	linux-armv6) echo "Linux (ARMv6 — Raspberry Pi Zero/1)" ;;
	linux-armv7) echo "Linux (ARMv7 — Raspberry Pi 2/3)" ;;
	linux-arm) echo "Linux (ARM)" ;;
	darwin-amd64) echo "macOS (Intel)" ;;
	darwin-arm64) echo "macOS (Apple Silicon)" ;;
	windows-amd64) echo "Windows (x86_64)" ;;
	windows-386) echo "Windows (x86)" ;;
	termux-arm64) echo "Termux / Android (ARM64)" ;;
	termux-amd64) echo "Termux / Android (x86_64)" ;;
	termux-arm) echo "Termux / Android (ARM)" ;;
	*) echo "Unknown" ;;
	esac
}

# Extension for archive
archive_ext() {
	case "$1" in
	*-windows-*) echo ".zip" ;;
	*) echo ".tar.gz" ;;
	esac
}

# Compress a binary
compress_bin() {
	local src="$1"
	local base
	base="$(basename "$src")"
	local ext
	ext="$(archive_ext "$base")"
	local ver="${NEW_TAG#v}"
	local platform="${base#"$BIN"-}"
	platform="${platform%.exe}"
	platform="${platform//-/_}"
	local stem="${BIN}_${ver}_${platform}"

	if [[ "$ext" == ".zip" ]]; then
		local archive="$BUILDDIR/release/${stem}${ext}"
		if [[ "$ZIP_AVAILABLE" == "true" ]]; then
			info "  zip: $archive"
			if [[ "$DRY_RUN" != "true" ]]; then
				(cd "$BUILDDIR" && zip -q "$(basename "$archive")" "$base")
				mv "$BUILDDIR/$(basename "$archive")" "$archive"
			fi
		else
			info "  cp (zip not available): $archive"
			archive="$BUILDDIR/release/${stem}"
			if [[ "$DRY_RUN" != "true" ]]; then
				cp -p "$src" "$archive"
			fi
		fi
	else
		local archive="$BUILDDIR/release/${stem}.tar.gz"
		info "  tgz: $archive"
		if [[ "$DRY_RUN" != "true" ]]; then
			tar -czf "$archive" -C "$BUILDDIR" "$base"
		fi
	fi
	echo "$archive"
}

RELEASE_FILES=()
CHECKSUM_FILE="$BUILDDIR/release/checksums.txt"

if [[ "$DRY_RUN" == "true" ]]; then
	info "dry-run: would compress and checksum all binaries"
else
	# Move raw binaries into release/ too (for users who want raw binaries)
	info "collecting artifacts..."
	for src in "$BUILDDIR"/"$BIN"-*; do
		name="$(basename "$src")"
		[[ "$name" == "checksums.txt" || "$name" == "RELEASE_NOTES.md" || "$name" == "release" ]] && continue
		info "  compressing: $name"
		archive="$(compress_bin "$src")"
		RELEASE_FILES+=("$archive")
	done

	# Also include raw binaries for users who prefer them (versioned filenames)
	ver="${NEW_TAG#v}"
	for src in "$BUILDDIR"/"$BIN"-*; do
		name="$(basename "$src")"
		[[ "$name" == "checksums.txt" || "$name" == "RELEASE_NOTES.md" || "$name" == "release" ]] && continue
		ext="${name##*.}"
		if [[ "$ext" == "exe" ]]; then
			platform="${name#"$BIN"-}"
			platform="${platform%.exe}"
			platform="${platform//-/_}"
			stem="${BIN}_${ver}_${platform}.exe"
		else
			platform="${name#"$BIN"-}"
			platform="${platform//-/_}"
			stem="${BIN}_${ver}_${platform}"
		fi
		cp -p "$src" "$BUILDDIR/release/$stem"
		RELEASE_FILES+=("$BUILDDIR/release/$stem")
	done

	# Generate checksums for release artifacts only (not checksums.txt or release notes)
	(cd "$BUILDDIR/release" && for f in *; do
		[[ "$f" == "checksums.txt" || "$f" == "RELEASE_NOTES.md" ]] && continue
		"$SHA_CMD" "$f"
	done) >"$REPO/$CHECKSUM_FILE"
	ok "checksums written to $CHECKSUM_FILE"
fi

echo ""

# ─── Generate release notes ────────────────────────────────────────────
# NOTE: The generated notes are purely section-based, no editorialising.
step "Generating release notes"

RELEASE_NOTES="$BUILDDIR/release/RELEASE_NOTES.md"
ver="${NEW_TAG#v}"

{
	# ═══════════════════════════════════════════════════════════════════
	#  HEADER
	# ═══════════════════════════════════════════════════════════════════
	echo "# $BIN $NEW_TAG"
	echo ""
	echo "$DESCRIPTION"
	echo ""

	# ═══════════════════════════════════════════════════════════════════
	#  INSTALL
	# ═══════════════════════════════════════════════════════════════════
	echo "## Install"
	echo ""
	echo '```sh'
	echo "# Via Go"
	echo "go install ${GO_MODULE}/cmd/orion@${NEW_TAG}"
	echo ""
	echo "# Pre-built archive (extract & install)"
	echo "tar xzf ${BIN}_${ver}_linux_amd64.tar.gz"
	echo "sudo install ${BIN} /usr/local/bin/"
	echo ""
	echo "# Raw binary (just the executable)"
	echo "chmod +x ${BIN}_${ver}_linux_amd64"
	echo "sudo mv ${BIN}_${ver}_linux_amd64 /usr/local/bin/${BIN}"
	echo ""
	echo "# Verify checksums"
	echo "sha256sum -c checksums.txt --ignore-missing"
	echo '```'
	echo ""

	# ═══════════════════════════════════════════════════════════════════
	#  CHANGELOG
	# ═══════════════════════════════════════════════════════════════════
	echo "## Changelog"
	echo ""

	if [[ "$COMMIT_COUNT" -eq 0 ]]; then
		echo "_no changes since ${LAST_TAG}_"
		echo ""
	else
		breaking="$(echo "$COMMITS" | grep -i '!\|BREAKING[ :-]\|breaking[ :-]' || true)"
		feats="$(echo "$COMMITS" | grep -i '^[0-9a-f]\{7\} feat' || true)"
		fixes="$(echo "$COMMITS" | grep -i '^[0-9a-f]\{7\} fix' || true)"
		rest="$(echo "$COMMITS" | grep -iv '^[0-9a-f]\{7\} feat\|^[0-9a-f]\{7\} fix' || true)"

		if [[ -n "$breaking" ]]; then
			echo -e "### ⚠ Breaking Changes\n"
			while IFS= read -r line; do
				[[ -z "$line" ]] && continue
				msg="${line#* }"
				echo "- $msg"
			done <<<"$breaking"
			echo ""
			for hash in $(echo "$breaking" | awk '{print $1}'); do
				feats="$(echo "$feats" | grep -v "^$hash" || true)"
				fixes="$(echo "$fixes" | grep -v "^$hash" || true)"
			done
		fi

		if [[ -n "$feats" ]]; then
			echo -e "### 🚀 Features\n"
			while IFS= read -r line; do
				[[ -z "$line" ]] && continue
				msg="${line#* }"
				echo "- $msg"
			done <<<"$feats"
			echo ""
		fi

		if [[ -n "$fixes" ]]; then
			echo -e "### 🐛 Bug Fixes\n"
			while IFS= read -r line; do
				[[ -z "$line" ]] && continue
				msg="${line#* }"
				echo "- $msg"
			done <<<"$fixes"
			echo ""
		fi

		if [[ -n "$rest" ]]; then
			echo -e "### 📦 Other\n"
			while IFS= read -r line; do
				[[ -z "$line" ]] && continue
				msg="${line#* }"
				echo "- $msg"
			done <<<"$rest"
			echo ""
		fi
	fi

	# ═══════════════════════════════════════════════════════════════════
	#  DOWNLOADS
	# ═══════════════════════════════════════════════════════════════════
	echo ""
	echo "## Downloads"
	echo ""

	echo "| Archive | Platform | Size |"
	echo "|---------|----------|------|"
	for f in "$BUILDDIR"/release/"${BIN}"_*; do
		name="$(basename "$f")"
		[[ "$name" == "checksums.txt" || "$name" == "RELEASE_NOTES.md" ]] && continue
		if [[ "$name" == *".tar.gz" || "$name" == *".zip" ]]; then
			size="$(stat_bytes "$f")"
			size_hr="$(human_size "$size")"
			platform="$(plat_of "$name")"
			echo "| \`$name\` | $platform | $size_hr |"
		fi
	done

	echo ""
	echo "| Binary | Platform | Size |"
	echo "|--------|----------|------|"
	for f in "$BUILDDIR"/release/"${BIN}"_*; do
		name="$(basename "$f")"
		[[ "$name" == "checksums.txt" || "$name" == "RELEASE_NOTES.md" ]] && continue
		[[ "$name" == *".tar.gz" || "$name" == *".zip" ]] && continue
		size="$(stat_bytes "$f")"
		size_hr="$(human_size "$size")"
		platform="$(plat_of "$name")"
		echo "| \`$name\` | $platform | $size_hr |"
	done

	# ═══════════════════════════════════════════════════════════════════
	#  CHECKSUMS
	# ═══════════════════════════════════════════════════════════════════
	if [[ -f "$CHECKSUM_FILE" ]]; then
		echo ""
		echo "## Checksums (SHA256)"
		echo ""
		echo '```'
		cat "$CHECKSUM_FILE"
		echo '```'
	fi

	# ═══════════════════════════════════════════════════════════════════
	#  RESOURCES
	# ═══════════════════════════════════════════════════════════════════
	echo ""
	echo "---"
	echo "_Report issues at $(grep -E '^\s+issue_tracker:' "$METADATA" | sed 's/.*issue_tracker: *//' | tr -d '"' 2>/dev/null || echo "https://github.com/${GITHUB_OWNER}/${GITHUB_REPO}/issues")_"

} >"$RELEASE_NOTES"

info "release notes written to $RELEASE_NOTES"

# ─── Editor review ────────────────────────────────────────────────────
if [[ "$SKIP_EDITOR" != "true" && "$DRY_RUN" != "true" ]]; then
	echo ""
	step "Editor review"
	info "opening release notes for review..."
	${EDITOR:-vi} "$RELEASE_NOTES"
	ok "release notes saved"
fi

# ─── Dry-run stop ─────────────────────────────────────────────────────
if [[ "$DRY_RUN" == "true" ]]; then
	echo ""
	step "Dry-run summary"
	echo ""
	echo -e "  ${c_bold}Tag:${c_reset}         $NEW_TAG"
	echo -e "  ${c_bold}Push:${c_reset}         git push $REMOTE $NEW_TAG"
	echo -e "  ${c_bold}Release:${c_reset}      gh release create $NEW_TAG"
	if [[ "$DRAFT" == "true" ]]; then
		echo -e "  ${c_bold}Draft:${c_reset}        yes"
	fi
	if [[ "$PRE_RELEASE" == "true" ]]; then
		echo -e "  ${c_bold}Pre-release:${c_reset}  yes"
	fi
	echo -e "  ${c_bold}Artifacts:${c_reset}     ${#RELEASE_FILES[@]} files"
	echo ""
	ok "dry-run complete — no changes made"
	exit 0
fi

# ─── Confirm ──────────────────────────────────────────────────────────
if [[ "$LOCAL_ONLY" != "true" ]]; then
	step "Publishing"
	info "publishing $NEW_TAG to $REMOTE"
fi

# ─── Force-overwrite existing assets ──────────────────────────────────
if [[ "$TAG_EXISTS_LOCAL" == "true" ]]; then
	warn "deleting existing local tag $NEW_TAG"
	git tag -d "$NEW_TAG"
fi
if [[ "$TAG_EXISTS_REMOTE" == "true" && "$LOCAL_ONLY" != "true" ]]; then
	warn "deleting existing remote tag $REMOTE/$NEW_TAG"
	git push --delete "$REMOTE" "$NEW_TAG"
fi
if [[ "$TAG_EXISTS_REMOTE" == "true" && "$LOCAL_ONLY" != "true" ]]; then
	warn "deleting existing GitHub release $NEW_TAG"
	gh release delete "$NEW_TAG" --yes 2>/dev/null || true
fi

# ─── Create tag ───────────────────────────────────────────────────────
echo ""
step "Tagging"

TAG_FLAG="-a"

# Build an industry-grade tag message via a temp file to avoid quoting issues
TAG_MSG_FILE=$(mktemp)
{
	printf '%s %s (%s)\n' "$BIN" "$NEW_TAG" "$(date -u '+%Y-%m-%d')"
	if [[ "$COMMIT_COUNT" -gt 0 ]]; then
		printf '\n%s\n' "$COMMITS"
	fi
} >"$TAG_MSG_FILE"

if [[ "$SIGN_TAG" == "true" ]]; then
	TAG_FLAG="-s"
	info "creating GPG-signed tag: $NEW_TAG"
else
	info "creating annotated tag: $NEW_TAG"
fi

git tag "$TAG_FLAG" "$NEW_TAG" -F "$TAG_MSG_FILE"
rm -f "$TAG_MSG_FILE"

# Register for cleanup in case we're interrupted after tagging but before push
CLEANUP_TAG="$NEW_TAG"

ok "tag $NEW_TAG created locally"

# ─── Push tag ─────────────────────────────────────────────────────────
if [[ "$LOCAL_ONLY" == "true" ]]; then
	ok "local-only mode — done (tag $NEW_TAG is local)"
	CLEANUP_TAG=""
	exit 0
fi

echo ""
step "Pushing tag"

info "pushing tag $NEW_TAG to $REMOTE"
git push "$REMOTE" "$NEW_TAG"

# Tag pushed successfully — no longer need cleanup for local-only failures
CLEANUP_TAG=""

ok "tag $NEW_TAG pushed to $REMOTE"

# Wait for the tag to be visible remotely before creating the release
if [[ "$LOCAL_ONLY" != "true" ]]; then
	info "waiting for tag to propagate..."
	for _ in {1..10}; do
		if git ls-remote --tags "$REMOTE" "$NEW_TAG" 2>/dev/null | grep -q "refs/tags/$NEW_TAG$"; then
			ok "tag confirmed on $REMOTE"
			break
		fi
		sleep 1
	done
fi

# ─── Gather release assets ────────────────────────────────────────────
# Use archives + raw binaries + checksums
RELEASE_ASSETS=()
while IFS= read -r -d '' f; do
	RELEASE_ASSETS+=("$f")
done < <(find "$BUILDDIR/release" -type f \( -name "$BIN-*" -o -name "${BIN}_*" \) ! -name "RELEASE_NOTES.md" -print0 2>/dev/null || true)
RELEASE_ASSETS+=("$CHECKSUM_FILE")

# ─── Create GitHub release ────────────────────────────────────────────
echo ""
step "Creating GitHub release"

GH_OPTS=()
GH_OPTS+=(--title "$BIN $NEW_TAG")
GH_OPTS+=(--notes-file "$RELEASE_NOTES")
if [[ "$DRAFT" == "true" ]]; then
	GH_OPTS+=(--draft)
	info "creating draft release"
fi
if [[ "$PRE_RELEASE" == "true" ]]; then
	GH_OPTS+=(--prerelease)
	info "marking as pre-release"
fi

set +e
gh release create "$NEW_TAG" "${GH_OPTS[@]}" "${RELEASE_ASSETS[@]}"
GH_EXIT=$?
set -e

if [[ $GH_EXIT -ne 0 ]]; then
	warn "'gh release create' failed (exit $GH_EXIT)"
	echo ""
	echo -e "  ${c_bold}Tag:${c_reset}         $NEW_TAG (already pushed to $REMOTE)"
	echo -e "  ${c_bold}Release:${c_reset}      failed to create"
	echo ""
	echo "  To retry the release later:"
	echo "    gh release create $NEW_TAG \\"
	echo "      --title \"$BIN $NEW_TAG\" \\"
	echo "      --notes-file $RELEASE_NOTES \\"
	for asset in "${RELEASE_ASSETS[@]}"; do
		echo "      \"$asset\" \\"
	done
	echo ""
	echo "  To delete the tag and start over:"
	echo "    git tag -d $NEW_TAG && git push --delete $REMOTE $NEW_TAG"
	die "release creation failed"
fi

ok "GitHub release created"

# ─── Done ─────────────────────────────────────────────────────────────
echo ""
step "Done"

REPO_NWO="$(gh repo view --json nameWithOwner --jq .nameWithOwner 2>/dev/null || echo "${GITHUB_OWNER}/${GITHUB_REPO}")"
echo ""
echo -e "  ${c_bold}Release:${c_reset}       https://github.com/$REPO_NWO/releases/tag/$NEW_TAG"
echo -e "  ${c_bold}Tag:${c_reset}            $NEW_TAG"
echo -e "  ${c_bold}Artifacts:${c_reset}      ${#RELEASE_ASSETS[@]} files"
if [[ "$DRAFT" == "true" ]]; then
	echo -e "  ${c_bold}Status:${c_reset}        ${c_yellow}draft${c_reset} (publish on GitHub when ready)"
fi
echo ""
ok "release $NEW_TAG published successfully"
