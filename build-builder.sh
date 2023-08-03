#!/bin/bash

# Print usage
usage() {
    echo "Usage: build-release.sh -v <version number>"
    echo "This script builds the Smartnode builder image used to build the daemon binaries."
    exit 0
}

# =================
# === Main Body ===
# =================

# Get the version
while getopts "admv:" FLAG; do
    case "$FLAG" in
        v) VERSION="$OPTARG" ;;
        *) usage ;;
    esac
done
if [ -z "$VERSION" ]; then
    usage
fi

echo -n "Building Docker image... "
docker build -t poolseatest/smartnode-builder:$VERSION -f docker/smartnode-builder .
docker tag poolseatest/smartnode-builder:$VERSION poolseatest/smartnode-builder:latest
docker push poolseatest/smartnode-builder:$VERSION
docker push poolseatest/smartnode-builder:latest
echo "done!"