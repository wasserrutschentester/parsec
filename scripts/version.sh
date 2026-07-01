#!/bin/sh

# Generate semver-compliant version string
raw=$(git describe --tags --always --match "v[0-9]*.[0-9]*.[0-9]*" --dirty 2>/dev/null || echo "v0.0.0-unknown")

if echo "$raw" | grep -Eq "^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9]+-g[0-9a-f]+)?(-dirty)?$"; then
    read -r MAJOR MINOR PATCH COMMITS HASH DIRTY <<EOF
$(echo "$raw" | sed -E 's/^v([0-9]+)\.([0-9]+)\.([0-9]+)(-([0-9]+)-g([0-9a-f]+))?(-dirty)?$/\1 \2 \3 \5 \6 \7/')
EOF

    if [ -z "$COMMITS" ] && [ -z "$DIRTY" ]; then
        echo "v${MAJOR}.${MINOR}.${PATCH}"
    else
        NEXT_PATCH=$((PATCH + 1))
        VERSION="v${MAJOR}.${MINOR}.${NEXT_PATCH}-dev"
        
        if [ -n "$COMMITS" ]; then
            VERSION="${VERSION}.${COMMITS}+${HASH}"
            if [ -n "$DIRTY" ]; then
                VERSION="${VERSION}.dirty"
            fi
        else
            VERSION="${VERSION}.0"
            if [ -n "$DIRTY" ]; then
                VERSION="${VERSION}+dirty"
            fi
        fi
        
        echo "$VERSION"
    fi
else
    # Fallback for untagged versions (e.g., just a commit hash)
    echo "$raw" | sed -e '/^v/! s/^/v0.0.0-/' -e 's/-dirty/+dirty/'
fi
