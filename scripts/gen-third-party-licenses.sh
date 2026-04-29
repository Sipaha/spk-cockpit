#!/usr/bin/env bash
# Regenerate THIRD_PARTY_LICENSES.md from the current Go and web runtime deps.
# Run from the repo root: ./scripts/gen-third-party-licenses.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

OUT="$ROOT/THIRD_PARTY_LICENSES.md"
GO_FRAGMENT="$(mktemp)"
WEB_FRAGMENT="$(mktemp)"
trap 'rm -f "$GO_FRAGMENT" "$WEB_FRAGMENT"' EXIT

GOMODCACHE="$(go env GOMODCACHE)"
DEPS_TMP="$(mktemp)"
go list -deps -tags 'wails production' \
  -f '{{if .Module}}{{.Module.Path}}@{{.Module.Version}}{{end}}' \
  ./cmd/cockpit 2>/dev/null \
  | sort -u \
  | grep -v "^github.com/spk/spk-cockpit" \
  | grep "@" \
  > "$DEPS_TMP"

while IFS= read -r mod; do
    path="${mod%@*}"; ver="${mod#*@}"
    dir="$GOMODCACHE/$path@$ver"
    license_file=""
    for f in LICENSE LICENSE.txt LICENSE.md LICENCE COPYING; do
        if [ -f "$dir/$f" ]; then license_file="$dir/$f"; break; fi
    done
    if [ -z "$license_file" ]; then
        echo "warning: no LICENSE for $mod" >&2
        continue
    fi
    {
        echo "## $path"
        echo
        echo "Version: \`$ver\`"
        echo
        echo '```'
        cat "$license_file"
        echo '```'
        echo
    } >> "$GO_FRAGMENT"
done < "$DEPS_TMP"
rm -f "$DEPS_TMP"

# Web runtime deps: the full transitive `dependencies` tree (excluding @types/*
# which are TS-only and never bundled), plus build-time tools whose own code ends
# up in the redistributed bundle. Right now the only such build-time tool is
# tailwindcss: its preflight CSS is copied into our final stylesheet (verifiable
# via the `/*! tailwindcss ... | MIT */` comment in web/embed/dist/assets/index-*.css).
# Other devDependencies (vite, eslint, typescript, vitest, @testing-library/*, etc.)
# do not emit their own code into the final artifact.
BUNDLED_BUILD_TOOLS="tailwindcss"
TRANSITIVE_PROD="$(cd "$ROOT/web" && pnpm list -r --prod --depth Infinity --json 2>/dev/null | node -e "
const data = JSON.parse(require('fs').readFileSync(0, 'utf8'));
const seen = new Set();
function walk(deps) {
  if (!deps) return;
  for (const [name, info] of Object.entries(deps)) {
    if (name.startsWith('@types/')) continue;
    seen.add(name);
    walk(info.dependencies);
  }
}
for (const p of data) walk(p.dependencies);
console.log([...seen].sort().join(' '));
")"
WEB_DEPS="$TRANSITIVE_PROD $BUNDLED_BUILD_TOOLS"

cd "$ROOT/web"
for pkg in $WEB_DEPS; do
    real=""
    if [ -L "node_modules/$pkg" ] || [ -d "node_modules/$pkg" ]; then
        real="$(readlink -f "node_modules/$pkg" 2>/dev/null || echo "node_modules/$pkg")"
    else
        real="$(find node_modules/.pnpm -maxdepth 3 -type d -name "$pkg" 2>/dev/null | head -1)"
    fi
    if [ -z "$real" ] || [ ! -d "$real" ]; then
        echo "warning: no node_modules for $pkg" >&2
        continue
    fi
    license_file=""
    for f in LICENSE LICENSE.md LICENSE.txt LICENCE COPYING License.md license.md; do
        if [ -f "$real/$f" ]; then license_file="$real/$f"; break; fi
    done
    ver="$(node -e "console.log(require('$real/package.json').version)" 2>/dev/null || echo "?")"
    {
        echo "## $pkg"
        echo
        if [ -z "$license_file" ]; then
            echo "Version: \`$ver\` (no LICENSE file shipped)"
            echo
        else
            echo "Version: \`$ver\`"
            echo
            echo '```'
            cat "$license_file"
            echo '```'
            echo
        fi
    } >> "$WEB_FRAGMENT"
done
cd "$ROOT"

{
cat <<'HDR'
# Third-Party Licenses

`spk-cockpit` redistributes the following third-party software, each under its own license. The full text of each license is reproduced below.

This file lists only **runtime dependencies** that are compiled into the released binary or bundled web assets — build-only and test-only dependencies are not included.

This file is generated. Regenerate with `scripts/gen-third-party-licenses.sh`.

---

# Go dependencies

The following Go modules are linked into the `spk-cockpit` binary.

HDR
cat "$GO_FRAGMENT"

cat <<'WEB'
---

# Web dependencies

The following npm packages are bundled into the embedded React UI (under `web/embed/dist`).

WEB
cat "$WEB_FRAGMENT"
} > "$OUT"

echo "wrote $OUT ($(wc -l < "$OUT") lines)"
