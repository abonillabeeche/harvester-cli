#!/usr/bin/env bash
# verify-catalog-urls.sh
#
# For every image URL in image-metadata.json, fetches the first 1 MiB via a
# HTTP Range request and reports PASS / FAIL. Follows redirects, so mirror
# redirects (Fedora, Ubuntu, etc.) resolve to the actual byte source.
#
# Usage:
#   ./scripts/verify-catalog-urls.sh                            # uses ./image-metadata.json
#   ./scripts/verify-catalog-urls.sh path/to/other.json
#
# Env vars:
#   RANGE_BYTES  bytes to fetch per URL (default 1048575 = 1 MiB - 1)
#   TIMEOUT      curl --max-time seconds (default 60)
#
# Exit code: 0 if every URL passes, non-zero if any fail.

set -u

METADATA="${1:-image-metadata.json}"
RANGE_BYTES="${RANGE_BYTES:-1048575}"
TIMEOUT="${TIMEOUT:-60}"

if [ ! -f "$METADATA" ]; then
    echo "error: metadata file not found: $METADATA" >&2
    exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "error: jq is required (brew install jq)" >&2
    exit 2
fi

tmp_entries="$(mktemp)"
trap 'rm -f "$tmp_entries"' EXIT

jq -r '
    .HarvesterImageCatalog
    | to_entries[]
    | .key as $os
    | .value[]
    | "\($os)|\(.version)|\(.url)"
' "$METADATA" > "$tmp_entries"

total=$(wc -l <"$tmp_entries" | tr -d ' ')
if [ "$total" -eq 0 ]; then
    echo "error: no entries found in $METADATA" >&2
    exit 2
fi

echo "Verifying $total catalog URLs (first $((RANGE_BYTES + 1)) bytes, timeout ${TIMEOUT}s each)..."
echo

printf '%-6s %-6s %-38s %-12s %s\n' "STATUS" "HTTP" "OS/VERSION" "SIZE" "NOTES"
printf -- '-%.0s' $(seq 1 100); echo

pass=0
fail=0
fail_lines=""

while IFS='|' read -r os version url; do
    out=$(curl -r "0-${RANGE_BYTES}" -sSL --max-time "$TIMEOUT" \
        -o /dev/null \
        -w '%{http_code}|%{size_download}' \
        "$url" 2>&1)
    rc=$?

    status="FAIL"
    http_code="?"
    size="0"
    reason=""

    if [ "$rc" -eq 0 ]; then
        http_code=$(echo "$out" | cut -d'|' -f1)
        size=$(echo "$out" | cut -d'|' -f2)
        # 200 = server ignored range and sent full body, 206 = partial content. Both are fine.
        if { [ "$http_code" = "200" ] || [ "$http_code" = "206" ]; } && [ "$size" -gt 0 ] 2>/dev/null; then
            status="PASS"
        else
            reason="http=$http_code size=$size"
        fi
    else
        reason="curl rc=$rc"
    fi

    printf '%-6s %-6s %-38s %-12s %s\n' \
        "$status" "$http_code" "$os/$version" "${size}B" "$reason"

    if [ "$status" = "PASS" ]; then
        pass=$((pass + 1))
    else
        fail=$((fail + 1))
        fail_lines="${fail_lines}${os}/${version}  ${url}
"
    fi
done < "$tmp_entries"

echo
echo "Summary: $pass passed, $fail failed (of $total total)"

if [ "$fail" -gt 0 ]; then
    echo
    echo "Failed URLs:"
    printf '%s' "$fail_lines"
    exit 1
fi
