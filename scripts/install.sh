#!/bin/sh
# Requires POSIX utilities, curl, GNU/BSD tar, gzip, mktemp, and either
# sha256sum or shasum (macOS provides shasum). No package runtime is installed.
set -eu
LC_ALL=C
export LC_ALL
unset TAR_OPTIONS GZIP
umask 077

fail() { printf 'sei installer: %s\n' "$*" >&2; exit 1; }
valid_version() {
    printf '%s\n' "$1" | awk '
        NR != 1 { bad=1 }
        !/^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$/ { bad=1 }
        { n=split($0,a,"-"); if (n>1) {
            sub(/^[^-]*-/, ""); n=split($0,a,".");
            for(i=1;i<=n;i++) if(a[i] ~ /^0[0-9]+$/) bad=1
        } }
        END { exit bad || NR != 1 }'
}
absent() { [ ! -e "$1" ] && [ ! -L "$1" ]; }
regular() { [ ! -L "$1" ] && [ -f "$1" ]; }
conflict() { fail 'unrecognized sei or receipt path; inspect and relocate existing paths manually before retrying.'; }
unchanged_binary() {
    if [ -z "$old_record" ]; then absent "$install_dir/sei"
    else
        regular "$install_dir/sei" && [ -x "$install_dir/sei" ] &&
            [ "$(digest "$install_dir/sei")" = "$old_digest" ]
    fi
}
unchanged_receipt() {
    if [ -z "$receipt_digest" ]; then absent "$install_dir/.sei-install-receipt"
    else
        regular "$install_dir/.sei-install-receipt" &&
            [ "$(digest "$install_dir/.sei-install-receipt")" = "$receipt_digest" ]
    fi
}
digest() {
    if [ "$sha_tool" = sha256sum ]; then
        sum_output=$(sha256sum < "$1") || return 1
    else
        sum_output=$(shasum -a 256 < "$1") || return 1
    fi
    printf '%s\n' "$sum_output" | awk '{print $1}'
}
download() {
    curl -q --fail --silent --show-error --location --proto '=https' \
        --proto-redir '=https' --tlsv1.2 --output "$2" "$1"
}

tag=latest
install_dir=
version_set=false
dir_set=false
while [ "$#" -gt 0 ]; do
    case "$1" in
        --version)
            [ "$#" -ge 2 ] && [ "$version_set" = false ] || fail 'expected one --version value'
            tag=$2; version_set=true; shift 2 ;;
        --install-dir)
            [ "$#" -ge 2 ] && [ "$dir_set" = false ] && [ -n "$2" ] || fail 'expected one --install-dir value'
            install_dir=$2; dir_set=true; shift 2 ;;
        --help)
            printf '%s\n' 'Usage: sh install.sh [--version vMAJOR.MINOR.PATCH[-PRERELEASE]|latest] [--install-dir DIR]'
            exit 0 ;;
        *) fail "unknown option: $1" ;;
    esac
done
for tool in awk cat chmod curl gzip mkdir mktemp mv rm sed tar uname; do
    command -v "$tool" >/dev/null 2>&1 || fail "required utility missing: $tool"
done
if command -v sha256sum >/dev/null 2>&1; then sha_tool=sha256sum
elif command -v shasum >/dev/null 2>&1; then sha_tool=shasum
else fail 'required utility missing: sha256sum or shasum'; fi
[ "$tag" = latest ] || valid_version "$tag" || fail 'invalid version; expected vMAJOR.MINOR.PATCH[-PRERELEASE]'
case "$(uname -s)/$(uname -m)" in
    Linux/x86_64) target=linux_amd64 ;;
    Linux/aarch64) target=linux_arm64 ;;
    Darwin/x86_64) target=darwin_amd64 ;;
    Darwin/arm64) target=darwin_arm64 ;;
    *) fail 'unsupported host; expected Linux x86_64/aarch64 or Darwin x86_64/arm64' ;;
esac
if [ "$dir_set" = false ]; then
    [ -n "${HOME:-}" ] || fail 'HOME is unset; specify --install-dir'
    install_dir=$HOME/.local/bin
fi
case "$install_dir" in /*) ;; *) install_dir=$PWD/$install_dir ;; esac
printf '%s\n' "$install_dir" | awk 'NR != 1 || /[[:cntrl:]]/ { bad=1 } END { exit bad }' || fail 'install directory contains control characters'
mkdir -p "$install_dir" || fail 'cannot create install directory; choose a user-writable --install-dir (no sudo)'
# Keep a sentinel so command substitution cannot hide trailing path newlines.
install_dir=$(CDPATH='' cd -P "$install_dir" && pwd -P && printf '.') || fail 'cannot resolve install directory'
install_dir=${install_dir%.}
install_dir=${install_dir%?}
printf '%s\n' "$install_dir" | awk 'NR != 1 || /[[:cntrl:]]/ { bad=1 } END { exit bad }' || fail 'resolved install directory contains control characters'
old_record='' old_digest='' receipt_digest=''
if ! absent "$install_dir/sei" || ! absent "$install_dir/.sei-install-receipt"; then
    if ! regular "$install_dir/sei" || [ ! -x "$install_dir/sei" ] ||
        ! regular "$install_dir/.sei-install-receipt"; then conflict; fi
    receipt_digest=$(digest "$install_dir/.sei-install-receipt") || conflict
    # Accept only canonical v1 records, with no duplicate version or digest.
    awk '
        NR == 1 { if ($0 != "sei-install-receipt-v1") bad=1; next }
        $1 !~ /^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$/ { bad=1 }
        NF != 2 || $0 != $1 " " $2 || length($2) != 64 || $2 ~ /[^0-9a-f]/ { bad=1 }
        versions[$1]++ || digests[$2]++ { bad=1 }
        END { exit bad || NR < 2 || NR > 3 }' "$install_dir/.sei-install-receipt" || conflict
    {
        IFS= read -r header || conflict
        [ "$header" = sei-install-receipt-v1 ] || conflict
        while IFS= read -r record; do
            valid_version "${record% *}" || conflict
        done
        [ -z "$record" ] || conflict
    } < "$install_dir/.sei-install-receipt"
    old_digest=$(digest "$install_dir/sei") || conflict
    old_record=$(awk -v sum="$old_digest" 'NR > 1 && $2 == sum { print }' "$install_dir/.sei-install-receipt") || conflict
    [ -n "$old_record" ] || conflict
fi
stage=$(mktemp -d "$install_dir/.sei-install.XXXXXXXX") || fail 'cannot create private install stage'
trap 'rm -rf "$stage"' 0
trap 'exit 1' HUP INT TERM
base=https://github.com/primaprashant/sei/releases
if [ "$tag" = latest ]; then
    resolved=$(curl -q --fail --silent --show-error --location --proto '=https' \
        --proto-redir '=https' --tlsv1.2 --output /dev/null --write-out '%{url_effective}' "$base/latest") || fail 'cannot resolve latest release'
    case "$resolved" in "$base/tag/"*) tag=${resolved#"$base/tag/"} ;; *) fail 'unexpected latest release URL' ;; esac
    valid_version "$tag" || fail 'invalid resolved release tag'
fi
version=${tag#v}
asset=sei_${version}_${target}.tar.gz
download "$base/download/$tag/$asset" "$stage/archive.tar.gz" || fail 'archive download failed'
download "$base/download/$tag/sei_${version}_checksums.txt" "$stage/checksums" || fail 'checksum download failed'
expected=$(awk -v name="$asset" '
    $2 == name { count++; if(NF != 2 || length($1) != 64 || $1 ~ /[^0-9a-f]/) bad=1; sum=$1 }
    END { if(count != 1 || bad) exit 1; print sum }' "$stage/checksums") || fail 'expected exactly one valid archive checksum entry'
[ "$(digest "$stage/archive.tar.gz")" = "$expected" ] || fail 'archive checksum mismatch'
gzip -t "$stage/archive.tar.gz" || fail 'invalid gzip archive'
# GNU and BSD tar differ in owner/date columns. Validate names separately and
# only the common leading type character in verbose output; never parse paths
# from verbose columns or extract archive-controlled paths onto the filesystem.
# Inspect beyond zero padding too. BSD tar requires the long option here.
tar --ignore-zeros -tzf "$stage/archive.tar.gz" > "$stage/names" || fail 'cannot list archive'
awk '
    $0 != "sei" && $0 != "README.md" && $0 != "LICENSE" && $0 != "THIRD_PARTY_NOTICES" { bad=1 }
    seen[$0]++ { bad=1 }
    END { exit bad || NR != 4 }' "$stage/names" || fail 'unexpected, missing or duplicate archive member'
tar --ignore-zeros -tvzf "$stage/archive.tar.gz" > "$stage/types" || fail 'cannot inspect archive types'
awk 'substr($0,1,1) != "-" { bad=1 } END { exit bad || NR != 4 }' "$stage/types" || fail 'archive members must be ordinary files'
tar -xOzf "$stage/archive.tar.gz" sei > "$stage/sei" || fail 'cannot extract sei'
[ -s "$stage/sei" ] || fail 'empty executable'
chmod 755 "$stage/sei" || fail 'cannot make candidate executable'
"$stage/sei" --version > "$stage/version" 2> "$stage/stderr" || fail 'candidate --version failed'
[ ! -s "$stage/stderr" ] && [ "$(cat "$stage/version")" = "sei $version" ] || fail 'candidate version mismatch'
binary_digest=$(digest "$stage/sei") || fail 'cannot hash candidate'
case "$binary_digest" in ''|*[!0-9a-f]*) fail 'invalid candidate digest' ;; esac
[ "${#binary_digest}" -eq 64 ] || fail 'invalid candidate digest length'
candidate_record="$tag $binary_digest"
if [ -n "$old_record" ] && [ "$old_record" != "$candidate_record" ]; then
    [ "${old_record% *}" != "$tag" ] && [ "$old_digest" != "$binary_digest" ] || fail 'ambiguous candidate version/digest'
    printf 'sei-install-receipt-v1\n%s\n%s\n' "$old_record" "$candidate_record" > "$stage/receipt" || fail 'cannot prepare receipt'
else
    printf 'sei-install-receipt-v1\n%s\n' "$candidate_record" > "$stage/receipt" || fail 'cannot prepare receipt'
fi
if ! unchanged_binary || ! unchanged_receipt; then fail 'install paths changed during preparation'; fi
receipt_digest=$(digest "$stage/receipt") || fail 'cannot hash prepared receipt'
# No hostile concurrent-writer or crash-durability guarantee. Receipt first:
# a failed final rename may leave a prepared record, never an unowned binary.
mv "$stage/receipt" "$install_dir/.sei-install-receipt" || fail 'cannot commit receipt; binary not installed'
if ! unchanged_binary || ! unchanged_receipt; then fail 'install paths changed; prepared receipt retained'; fi
mv "$stage/sei" "$install_dir/sei" || fail 'cannot commit binary; prepared receipt retained'
quoted_dir=$(printf '%s' "$install_dir" | sed "s/'/'\\\\''/g")
printf 'Installed sei %s at %s/sei\n' "$version" "$install_dir"
case ":${PATH:-}:" in
    *:"$install_dir":*) ;;
    *)
        printf 'To add sei to PATH in your current shell, run:\n'
        printf "export PATH='%s':\"\$PATH\"\n" "$quoted_dir" ;;
esac
printf "Run:\n'%s/sei'\n" "$quoted_dir"
