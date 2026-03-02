#!/bin/sh
set -e

BUMP="${1:-patch}"

main() {
    need_cmd git
    need_cmd goreleaser

    # Ensure working tree is clean
    if [ -n "$(git status --porcelain)" ]; then
        err "working tree is not clean — commit or stash changes first"
    fi

    current="$(latest_tag)"
    next="$(bump_version "$current" "$BUMP")"

    info "current version: ${current:-none}"
    info "next version:    ${next}"

    git tag -a "v${next}" -m "v${next}"
    info "created tag v${next}"

    git push origin "v${next}"
    info "pushed tag v${next}"

    goreleaser release --clean
    info "release v${next} published"
}

latest_tag() {
    git tag -l 'v*' --sort=-v:refname | head -1 | sed 's/^v//'
}

bump_version() {
    version="$1"
    bump_type="$2"

    if [ -z "$version" ]; then
        # No existing tag — start at 0.1.0
        echo "0.1.0"
        return
    fi

    major="$(echo "$version" | cut -d. -f1)"
    minor="$(echo "$version" | cut -d. -f2)"
    patch="$(echo "$version" | cut -d. -f3)"

    case "$bump_type" in
        major)
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        minor)
            minor=$((minor + 1))
            patch=0
            ;;
        patch)
            patch=$((patch + 1))
            ;;
        *)
            err "invalid bump type: ${bump_type} (expected: patch, minor, or major)"
            ;;
    esac

    echo "${major}.${minor}.${patch}"
}

need_cmd() {
    if ! command -v "$1" >/dev/null 2>&1; then
        err "required command not found: $1"
    fi
}

info() {
    printf '\033[0;32m=>\033[0m %s\n' "$1"
}

err() {
    printf '\033[0;31merror:\033[0m %s\n' "$1" >&2
    exit 1
}

main
