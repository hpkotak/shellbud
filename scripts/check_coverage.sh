#!/usr/bin/env bash
set -euo pipefail

profile="${COVERAGE_PROFILE:-/tmp/shellbud-cover.out}"
total_min="${COVERAGE_TOTAL_MIN:-85}"
critical_min="${COVERAGE_CRITICAL_MIN:-90}"
setup_min="${COVERAGE_SETUP_MIN:-70}"

compare_ge() {
	local got="$1"
	local want="$2"
	awk -v got="$got" -v want="$want" 'BEGIN { exit(got + 0 >= want + 0 ? 0 : 1) }'
}

extract_percent() {
	sed -n 's/.*coverage: \([0-9.][0-9.]*\)%.*/\1/p' | head -n1
}

echo "Checking coverage thresholds..."
echo "  total >= ${total_min}%"
echo "  critical packages >= ${critical_min}%"
echo "  internal/setup >= ${setup_min}% (temporary floor)"

go test -coverprofile="$profile" ./...
total_cov="$(go tool cover -func="$profile" | awk '/^total:/{gsub("%","",$3); print $3}')"

if [[ -z "$total_cov" ]]; then
	echo "Could not determine total coverage from $profile"
	exit 1
fi

failures=0
if ! compare_ge "$total_cov" "$total_min"; then
	echo "FAIL total coverage: got ${total_cov}% (want >= ${total_min}%)"
	failures=1
else
	echo "PASS total coverage: ${total_cov}%"
fi

packages=(
	"./cmd"
	"./internal/repl"
	"./internal/provider"
	"./internal/safety"
	"./internal/prompt"
	"./internal/setup"
)

thresholds=(
	"$critical_min"
	"$critical_min"
	"$critical_min"
	"$critical_min"
	"$critical_min"
	"$setup_min"
)

for i in "${!packages[@]}"; do
	pkg="${packages[$i]}"
	threshold="${thresholds[$i]}"

	cover_line="$(go test -cover "$pkg" 2>&1 | extract_percent || true)"
	if [[ -z "$cover_line" ]]; then
		echo "Could not determine coverage for $pkg"
		failures=1
		continue
	fi

	if ! compare_ge "$cover_line" "$threshold"; then
		echo "FAIL $pkg: got ${cover_line}% (want >= ${threshold}%)"
		failures=1
	else
		echo "PASS $pkg: ${cover_line}%"
	fi
done

if [[ "$failures" -ne 0 ]]; then
	exit 1
fi

echo "Coverage gates passed."
