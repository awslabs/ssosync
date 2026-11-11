#!/usr/bin/env bash
# Build one Serverless Application Repository variant of template.yaml (#399).
#
#   bash cicd/build/package/variant.sh <feature-patch> <app-name> <output>
#
# Run from the repository root (it reads ./template.yaml). It copies
# template.yaml, applies <feature-patch> (legacy.patch or next-gen.patch) with
# no fuzz, then sets Metadata.AWS::ServerlessRepo::Application.Name to
# <app-name>. Any drift fails: a hunk whose context no longer matches exactly,
# or an Application block without exactly one Name line. <output> is written
# only on success. Used by cicd/build/package/buildspec.yml and
# `make sar-variants`; how to change a feature patch is in the Makefile, above
# sar-variants.
#
# Portable between GNU (CodeBuild) and BSD/macOS patch and awk: no sed -i, the
# edit goes to a temp file next to <output>, then mv.
set -euo pipefail

die() { echo "variant.sh: $*" >&2; exit 1; }

[ $# -eq 3 ] || { echo "usage: $0 <feature-patch> <app-name> <output>" >&2; exit 2; }
feature_patch=$1 name=$2 out=$3

[ -f template.yaml ] || die "template.yaml not found in $(pwd); run from the repository root"
[ -f "$feature_patch" ] || die "feature patch not found: $feature_patch"
# SAR application names: letters, digits and hyphens. This also keeps the
# value literal in awk -v.
printf '%s\n' "$name" | grep -Eq '^[A-Za-z0-9-]{1,140}$' || die "invalid application name: $name"

patched=$(mktemp "$out.XXXXXX")
named=$(mktemp "$out.XXXXXX")
trap 'rm -f "$patched" "$patched.orig" "$patched.rej" "$named"' EXIT

cp template.yaml "$patched"
# -F 0: no fuzz, so a hunk whose context changed fails instead of applying
# near the right place. -N: never prompt to reverse. The rejects are printed so
# the build log shows the failing hunk.
if ! patch -N -F 0 "$patched" "$feature_patch"; then
  [ -f "$patched.rej" ] && cat "$patched.rej" >&2
  die "$feature_patch does not apply cleanly to template.yaml; regenerate it (see the Makefile, sar-variants)"
fi

# The Name line of the AWS::ServerlessRepo::Application block (two-space
# indented key under Metadata; its entries are four-space indented).
# Replace it, counting both the block and its Name lines. Plain EREs only (no
# interval braces or [[:classes:]], which older mawk lacks). awk would add a
# newline to an unterminated last line (template.yaml has none), so lines are
# joined by hand and the final newline is written only if the input had one.
if [ "$(tail -c 1 "$patched" | wc -l)" -eq 1 ]; then final_nl=1; else final_nl=0; fi
awk -v name="$name" -v final_nl="$final_nl" '
  function out(s) { printf "%s%s", sep, s; sep = "\n" }
  /^  AWS::ServerlessRepo::Application:[ \t]*$/ { blocks++; inapp = 1; out($0); next }
  inapp && /^ ? ?[^ #]/ { inapp = 0 }
  inapp && /^    Name:/ { names++; out("    Name: " name); next }
  { out($0) }
  END {
    if (final_nl && NR) printf "\n"
    if (blocks != 1 || names != 1) {
      printf "variant.sh: expected one AWS::ServerlessRepo::Application block with one Name line, found %d block(s), %d Name line(s)\n", blocks, names > "/dev/stderr"
      exit 1
    }
  }
' "$patched" > "$named"

# Verify: same number of lines, and the only change is the Name line.
[ "$(wc -l < "$patched")" -eq "$(wc -l < "$named")" ] || die "Name edit changed the line count"
changed=$(awk 'NR == FNR { a[FNR] = $0; next } a[FNR] != $0 { n++ } END { print n + 0 }' "$patched" "$named")
grep -Fxq "    Name: $name" "$named" || die "Name: $name not found after the edit"
[ "$changed" -le 1 ] || die "Name edit changed $changed lines"

chmod 644 "$named" # mktemp creates it 0600
mv "$named" "$out"
grep -E '^    (Name|ReadmeUrl):' "$out"
