#!/bin/bash
set -e
DIST_DIR=${1:-dist}
DOCKER_BUILD_DIR=tools/docker
if [ "$DOCKERHUB_REPO" == "" ]; then
    DOCKERHUB_REPO=blinkdisk/blinkdisk
fi

cp -r "$DIST_DIR/blinkdisk_linux_amd64/" "$DOCKER_BUILD_DIR/bin-amd64/"
chmod 0755 "$DOCKER_BUILD_DIR/bin-amd64/blinkdisk"
chmod 0755 "$DOCKER_BUILD_DIR/bin-amd64/rclone"
cp -r "$DIST_DIR/blinkdisk_linux_arm64/" "$DOCKER_BUILD_DIR/bin-arm64/"
chmod 0755 "$DOCKER_BUILD_DIR/bin-arm64/blinkdisk"
chmod 0755 "$DOCKER_BUILD_DIR/bin-arm64/rclone"
cp -r "$DIST_DIR/blinkdisk_linux_arm_6/" "$DOCKER_BUILD_DIR/bin-arm/"
chmod 0755 "$DOCKER_BUILD_DIR/bin-arm/blinkdisk"
chmod 0755 "$DOCKER_BUILD_DIR/bin-arm/rclone"

if [ "$BLINKDISK_VERSION_NO_PREFIX" == "" ]; then
    echo BLINKDISK_VERSION_NO_PREFIX not set, not publishing.
    exit 1
fi

major=$(echo $BLINKDISK_VERSION_NO_PREFIX | cut -f 1 -d .)
minor=$(echo $BLINKDISK_VERSION_NO_PREFIX | cut -f 2 -d .)
rev=$(echo $BLINKDISK_VERSION_NO_PREFIX | cut -f 3 -d .)

# x.y.z
if [[ "$BLINKDISK_VERSION_NO_PREFIX" =~ [0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    extra_tags="latest testing $major $major.$minor"
fi

# x.y.z-prerelease
if [[ "$BLINKDISK_VERSION_NO_PREFIX" =~ [0-9]+\.[0-9]+\.[0-9]+\-.*$ ]]; then
    extra_tags="testing"
fi

# yyyymmdd.0.hhmmss starts with 20
if [[ "$BLINKDISK_VERSION_NO_PREFIX" =~ 20[0-9]+\.[0-9]+\.[0-9]+ ]]; then
    extra_tags="unstable"
fi

versioned_image=$DOCKERHUB_REPO:$BLINKDISK_VERSION_NO_PREFIX
tags="-t $versioned_image"
for t in $extra_tags; do
    if [ "$t" != "0" ]; then
        tags="$tags -t $DOCKERHUB_REPO:$t"
    fi
done

echo Building $versioned_image with tags [$tags]...
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 $tags --push $DOCKER_BUILD_DIR
