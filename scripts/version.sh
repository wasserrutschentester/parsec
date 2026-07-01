#!/usr/bin/env bash

# Generate semver-compliant version string
raw=$(git describe --tags --always --match "v[0-9]*.[0-9]*.[0-9]*" --dirty 2>/dev/null || echo "v0.0.0-unknown")

if [[ "$raw" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)(-([0-9]+)-g([0-9a-f]+))?(-dirty)?$ ]]; then
    MAJOR="${BASH_REMATCH[1]}"
    MINOR="${BASH_REMATCH[2]}"
    PATCH="${BASH_REMATCH[3]}"
    COMMITS="${BASH_REMATCH[5]}"
    HASH="${BASH_REMATCH[6]}"
    DIRTY="${BASH_REMATCH[7]}"

    if [[ -z "$COMMITS" && -z "$DIRTY" ]]; then
        echo "v${MAJOR}.${MINOR}.${PATCH}"
    else
        NEXT_PATCH=$((PATCH + 1))
        VERSION="v${MAJOR}.${MINOR}.${NEXT_PATCH}-dev"
        
        if [[ -n "$COMMITS" ]]; then
            VERSION="${VERSION}.${COMMITS}+${HASH}"
            if [[ -n "$DIRTY" ]]; then
                VERSION="${VERSION}.dirty"
            fi
        else
            VERSION="${VERSION}.0"
            if [[ -n "$DIRTY" ]]; then
                VERSION="${VERSION}+dirty"
            fi
        fi
        
        echo "$VERSION"
    fi
else
    # Fallback for untagged versions (e.g., just a commit hash)
    echo "$raw" | sed -e '/^v/! s/^/v0.0.0-/' -e 's/-dirty/+dirty/'
fi
