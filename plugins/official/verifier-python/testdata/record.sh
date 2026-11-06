#!/bin/sh
# record.sh re-records the replay fixtures of the verifier-python provider: it runs each tool on each candidate tree of
# testdata/candidates and writes the raw output under replay/recordings. Needs network once (uvx fetches the tools); the
# provider and its tests never do. The work directory is fixed, so every path in a recording is /candidate or relative, and
# the home directory and host name are redacted: a recording holds no host path, user name or secret.
# Usage: testdata/record.sh   (from anywhere)
set -eu
here=$(cd "$(dirname "$0")" && pwd)
out="$here/../replay/recordings"
tmp=/tmp/numinance-record
rm -rf "$tmp"
mkdir -p "$tmp/cfg"
cp "$here"/config/* "$tmp/cfg/"
trap 'rm -rf "$tmp"' EXIT
sum() { printf '%s' "kit-candidate:$1" | shasum -a 256 | cut -d' ' -f1; }
for cand in seeded clean suppressed; do
  dir="$out/$(sum "$cand")"
  mkdir -p "$dir"
  rm -rf "$tmp/c"
  cp -R "$here/candidates/$cand" "$tmp/c"
  cd "$tmp/c"
  rec() { # rec <check> <exit-code> <output-file>
    sed -e "s|/private$tmp/c|/candidate|g" -e "s|$tmp/c|/candidate|g" -e "s|\\(private\\.\\)\\{0,1\\}tmp\\.numinance-record\\.cfg\\.||g" \
      -e 's|/Users/[^/"]*/|/home/redacted/|g' -e 's|hostname="[^"]*"|hostname="redacted"|' "$3" >"$dir/$1.out"
    printf '{"check":"%s","candidate":"%s","exit_code":%s%s}\n' "$1" "$cand" "$2" "${4:-}" >"$dir/$1.json"
  }
  set +e
  ruff check --config "$tmp/cfg/ruff.toml" --output-format json --no-cache --ignore-noqa . >"$tmp/o" 2>/dev/null
  rec ruff $? "$tmp/o"
  uvx --from mypy mypy --config-file "$tmp/cfg/mypy.ini" --output json --no-incremental --cache-dir=/dev/null . >"$tmp/o" 2>/dev/null
  code=$?; extra=""; [ "$cand" = suppressed ] && extra=',"directives":[{"name":"type-ignore","path":"shop/pricing.py","line":5}]'
  rec mypy $code "$tmp/o" "$extra"
  uvx --from semgrep semgrep scan --config "$tmp/cfg/semgrep.yml" --json --disable-nosem --metrics=off --disable-version-check --quiet . >"$tmp/o" 2>/dev/null
  rec semgrep $? "$tmp/o"
  uvx --from import-linter lint-imports --config "$tmp/cfg/importlinter.ini" >"$tmp/o" 2>/dev/null
  rec import-linter $? "$tmp/o"
  rm -f "$tmp/junit.xml"
  uvx --with pytest pytest -c "$tmp/cfg/pytest.ini" --rootdir . -o pythonpath="$tmp/c" --junitxml="$tmp/junit.xml" -p no:cacheprovider -q >/dev/null 2>&1
  code=$?
  [ -f "$tmp/junit.xml" ] || : >"$tmp/junit.xml"
  rec pytest $code "$tmp/junit.xml"
  set -e
  cd "$here"
done

# Synthetic recordings: no real tool is made to crash or to emit hostile output, so these are written by hand. "crashing"
# is a tool that exits with a crash code and a traceback; "malformed" is output that is not what the tool prints, and a
# pytest report that declares an entity.
for check in ruff mypy semgrep import-linter pytest; do
  dir="$out/$(sum crashing)"
  mkdir -p "$dir"
  printf 'Traceback (most recent call last):\n  File "tool", line 1, in <module>\nRuntimeError: the tool crashed\n' >"$dir/$check.out"
  printf '{"check":"%s","candidate":"crashing","exit_code":139}\n' "$check" >"$dir/$check.json"
  dir="$out/$(sum malformed)"
  mkdir -p "$dir"
  printf '{"check":"%s","candidate":"malformed","exit_code":1}\n' "$check" >"$dir/$check.json"
  printf 'this is not what the tool prints {{{' >"$dir/$check.out"
done
printf '<?xml version="1.0"?>\n<!DOCTYPE testsuites [<!ENTITY x "expanded">]>\n<testsuites><testsuite name="s" tests="1" failures="1"><testcase classname="c" name="n"><failure message="&x;">boom</failure></testcase></testsuite></testsuites>\n' >"$out/$(sum malformed)/pytest.out"
