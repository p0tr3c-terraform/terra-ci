#!/usr/bin/env bash

set -x

usage() {
    echo "github-release.sh [--asset-dir=<path>] [--tag=<git tag>]"
    echo "    Default --asset-dir is $PWD and --tag $TAG_NAME "
}

TAG_NAME=${TAG_NAME}
ASSET_DIR=${PWD:-"./"}

for i in "$@"; do
    case $i in
    --asset-dir=*)
        ASSET_DIR="${i#*=}"
        shift
        ;;
    --tag=*)
        TAG_NAME="${i#*=}"
        shift
        ;;
    *)
        usage
        exit 1
        ;;
    esac
done

ASSETS=()
for asset in "${ASSET_DIR}"/*; do
    ASSETS+=("-a" "$asset")
done

RELEASE_NOTES="release-notes.md"

# The hub CLI expects the first line to be the title
echo -e "${TAG_NAME}\n" > "${RELEASE_NOTES}"

# Update or create a release on github
if hub release show "${TAG_NAME}" > /dev/null; then
    # Occurs when the tag is created via GitHub UI w/ a release
    # Use -m "" to preserve the existing text.
    hub release edit "${ASSETS[@]}" -m "" "${TAG_NAME}"
else
    # Create a draft release
    hub release create "${ASSETS[@]}" -F ${RELEASE_NOTES} --draft "${TAG_NAME}"
fi
