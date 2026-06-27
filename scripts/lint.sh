#!/usr/bin/env bash
set -uo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)" && cd "$ROOT" || exit 1
TMPDIR="$(mktemp -d /tmp/golint.XXXXXX)"
trap 'rm -rf "$TMPDIR"' EXIT

SHORT=false
[[ "$*" == *--short* ]] && SHORT=true

export HOME="${HOME:-$HOME}"
export PATH="$HOME/go/bin:$PATH"

PASS=0
FAIL=0
SKIP=0

green='\e[32m'
red='\e[31m'
yellow='\e[33m'
bold='\e[1m'
dim='\e[2m'
reset='\e[0m'

passed() {
	PASS=$((PASS + 1))
	echo -e "  [${green}PASS${reset}] $1"
}
failed() {
	FAIL=$((FAIL + 1))
	echo -e "  [${red}FAIL${reset}] $1\n$2"
}
skipped() {
	SKIP=$((SKIP + 1))
	echo -e "  [${yellow}SKIP${reset}] $1"
}
banner() { echo -e "\n${bold}━━━ $1 ━━━${reset}"; }

_install() {
	local pkg="$1"
	echo -e "  ${yellow}→ Installing $pkg...${reset}"
	go install "${pkg}@latest" 2>&1
}

_has() { command -v "$1" &>/dev/null; }

# Run a check in the background, writing result file parseable by _collect.
# Delimiter ␟ (record separator) avoids label-vs-status split issues.
run_check() {
	local id="$1"
	local label="$2"
	shift 2
	if "$@" >"$TMPDIR/$id.out" 2>&1; then
		printf 'passed␟%s\n' "$label" >"$TMPDIR/$id.result"
	else
		printf 'failed␟%s\n' "$label" >"$TMPDIR/$id.result"
	fi
}

_collect() {
	local id="$1"
	local rf="$TMPDIR/$id.result"
	[[ -f "$rf" ]] || return
	IFS='␟' read -r status label <"$rf" || return
	case "$status" in
	passed) passed "$label" ;;
	failed)
		local out
		out="$(cat "$TMPDIR/$id.out" 2>/dev/null | sed 's/^/    /')"
		failed "$label" "$out"
		;;
	skipped)
		if [[ "$id" != "gosec" ]] || [[ "$SHORT" != true ]]; then
			skipped "$label"
		fi
		;;
	esac
}

echo -e "${bold}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${reset}"
echo -e "${bold}  Go Mega Lint — $(date '+%H:%M:%S')${reset}"
echo -e "${bold}  Root: $ROOT${reset}"
echo -e "${bold}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${reset}"

# ═══════════════════════════════════════════════════════
# PHASE 1 — Format, vet & test (sequential, mutates)
# ═══════════════════════════════════════════════════════

banner "PHASE 1: Auto-format & fix"

# gofumpt
if _has gofumpt || _install mvdan.cc/gofumpt; then
	unformatted=$(gofumpt -l -e . 2>/dev/null) || true
	if [[ -z "$unformatted" ]]; then
		passed "gofumpt — all clean"
	else
		n=$(echo "$unformatted" | wc -l)
		echo -e "  ${yellow}→ gofumpt fixing $n file(s)...${reset}"
		gofumpt -w . 2>/dev/null || true
		passed "gofumpt — formatted $n file(s)"
	fi
else
	skipped "gofumpt (install failed)"
fi

# go fmt
out=$(go fmt ./... 2>&1) || true
if [[ -z "$out" ]]; then
	passed "go fmt — all clean"
else
	failed "go fmt" "    ${out//$'\n'/$'\n'    }"
fi

# go fix
out=$(go fix ./... 2>&1) || true
if [[ -z "$out" ]]; then
	passed "go fix — no old API patterns"
else
	failed "go fix" "    ${out//$'\n'/$'\n'    }"
fi

# go mod tidy
if go mod tidy 2>/dev/null; then
	passed "go mod tidy — clean"
else
	failed "go mod tidy" "$(go mod tidy 2>&1 | sed 's/^/    /')"
fi

# ═══════════════════════════════════════════════════════
# PHASE 2 & 3 — Tests & static analysis (all parallel)
# ═══════════════════════════════════════════════════════

banner "PHASE 2 & 3: Tests and static analysis (parallel)"

CPUS=$(nproc 2>/dev/null || getconf _NPROCESSORS_ONLN || echo 4)

# go test
GOTEST_EXTRA=""
if $SHORT; then GOTEST_EXTRA="-short"; fi
run_check gotest "go test -race" go test -race -count=1 -parallel="$CPUS" $GOTEST_EXTRA ./...

# go vet
run_check govet "go vet" go vet ./...

# staticcheck
if _has staticcheck || _install honnef.co/go/tools/cmd/staticcheck; then
	run_check staticcheck "staticcheck" staticcheck ./...
else
	printf 'skipped␟staticcheck (install failed)\n' >"$TMPDIR/staticcheck.result"
fi

# gosec
if [[ "$SHORT" == true ]]; then
	printf 'skipped␟gosec (security, short mode)\n' >"$TMPDIR/gosec.result"
else
	if _has gosec || _install github.com/securego/gosec/v2/cmd/gosec; then
		run_check gosec "gosec (security)" gosec -quiet -fmt=text ./...
	else
		printf 'skipped␟gosec (install failed)\n' >"$TMPDIR/gosec.result"
	fi
fi

# golangci-lint
if _has golangci-lint || _install github.com/golangci/golangci-lint/cmd/golangci-lint; then
	run_check golangci "golangci-lint (100+ linters)" golangci-lint run --timeout=10m ./...
else
	printf 'skipped␟golangci-lint (install failed)\n' >"$TMPDIR/golangci.result"
fi

# shadow
if _has shadow || _install golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow; then
	run_check shadow "shadow (variable shadowing)" shadow ./...
else
	printf 'skipped␟shadow (install failed)\n' >"$TMPDIR/shadow.result"
fi

# deadcode
if _has deadcode || _install golang.org/x/tools/cmd/deadcode; then
	run_check deadcode "deadcode (unreachable funcs)" deadcode -test ./...
else
	printf 'skipped␟deadcode (install failed)\n' >"$TMPDIR/deadcode.result"
fi

# go mod verify
run_check modverify "go mod verify" go mod verify

# ─────────────────────────────────────────────────────
# Wait for all parallel checks
# ─────────────────────────────────────────────────────
echo -e "\n  ${dim}Waiting for parallel checks to finish...${reset}"
wait
echo -e "  ${dim}Done.${reset}"

# Show full test output
if [[ -f "$TMPDIR/gotest.out" ]]; then
	echo ""
	cat "$TMPDIR/gotest.out"
	echo ""
fi

# ─────────────────────────────────────────────────────
# Collect and report results
# ─────────────────────────────────────────────────────
banner "RESULTS"
_collect gotest
_collect govet
_collect staticcheck
_collect gosec
_collect golangci
_collect shadow
_collect deadcode
_collect modverify

# ─────────────────────────────────────────────────────
# SUMMARY
# ─────────────────────────────────────────────────────
echo ""
echo -e "${bold}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${reset}"
echo -e "  ${green}PASS:${reset} $PASS   ${red}FAIL:${reset} $FAIL   ${yellow}SKIP:${reset} $SKIP"
echo -e "${bold}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${reset}"

[[ "$FAIL" -eq 0 ]] || exit 1
