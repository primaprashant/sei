#!/usr/bin/env bash
# CI-only release boundary; tests substitute local git/gh commands, never credentials.
set -euo pipefail
export LC_ALL=C
fail() { printf 'sei release: %s\n' "$*" >&2; exit 1; }

tag=${GITHUB_REF_NAME:?}
[[ ${GITHUB_REF:-} == "refs/tags/$tag" && ${GITHUB_REF_TYPE:-} == tag ]] || fail 'not a tag ref'
core='(0|[1-9][0-9]*)'
[[ $tag =~ ^v$core\.$core\.$core(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ ]] || fail 'invalid release tag'
if [[ $tag == *-* ]]; then
    IFS=. read -r -a identifiers <<< "${tag#*-}"
    for identifier in "${identifiers[@]}"; do
        [[ ! $identifier =~ ^0[0-9]+$ ]] || fail 'leading zero in prerelease'
    done
fi
commit=$(git rev-parse HEAD)
[[ $commit == "${GITHUB_SHA:?}" ]] || fail 'checkout differs from event commit'
[[ $(git rev-parse --verify "refs/tags/$tag^{commit}") == "$commit" ]] || fail 'tag differs from checkout'
case ${1:-} in
    tag) exit 0 ;;
    draft) ;;
    *) fail 'expected tag or draft' ;;
esac
[[ ${SEI_RELEASE_COMMIT:?} == "$commit" && ${SEI_RELEASE_VERSION:?} == "${tag#v}" ]] || fail 'producer metadata mismatch'

# Reconstruct the exact manifest rather than trusting paths from downloaded text.
cd dist
assets=("sei_${SEI_RELEASE_VERSION}_linux_amd64.tar.gz"
        "sei_${SEI_RELEASE_VERSION}_linux_arm64.tar.gz"
        "sei_${SEI_RELEASE_VERSION}_darwin_amd64.tar.gz"
        "sei_${SEI_RELEASE_VERSION}_darwin_arm64.tar.gz")
for file in "${assets[@]}" "sei_${SEI_RELEASE_VERSION}_checksums.txt" install.sh install.sh.sha256; do
    [[ -f $file && ! -L $file ]] || fail "missing/nonregular asset: $file"
done
cmp <(shasum -a 256 "${assets[@]}" | sort) <(sort "sei_${SEI_RELEASE_VERSION}_checksums.txt") || fail 'archive manifest mismatch'
cmp <(shasum -a 256 install.sh) install.sh.sha256 || fail 'installer checksum mismatch'
cmp ../scripts/install.sh install.sh || fail 'installer differs from tested source'
assets+=("sei_${SEI_RELEASE_VERSION}_checksums.txt" install.sh install.sh.sha256)

gh --version
repo=${GITHUB_REPOSITORY:?}
[[ $(gh api "repos/$repo/commits/refs/tags/$tag" --jq .sha) == "$commit" ]] || fail 'remote tag commit mismatch'
# Listing must succeed; an authentication/network failure is not proof of absence.
existing=$(gh api --paginate "repos/$repo/releases" --jq '.[].tag_name')
while IFS= read -r released; do
    [[ $released != "$tag" ]] || fail 'release already exists; refusing overwrite'
done <<< "$existing"
prerelease=false
if [[ $tag == *-* ]]; then prerelease=true; fi
gh release create "$tag" "${assets[@]}" --repo "$repo" --draft --verify-tag \
    "--prerelease=$prerelease" --latest=false --title "sei $tag" \
    --notes "Draft release for $tag, built from $commit. See README.md for installation and usage."
