#!/bin/bash
set -e
dist_dir=$1
ver=$2

target_repo=$REPO_OWNER/scoop-bucket
source_repo=$REPO_OWNER/blinkdisk

if [ "$CI_TAG" == "" ]; then
  target_repo=$REPO_OWNER/scoop-test-builds
  source_repo=$REPO_OWNER/blinkdisk-test-builds
fi

if [ "$GITHUB_TOKEN" == "" ]; then
  echo Not publishing Scoop package because GITHUB_TOKEN is not set.
  exit 0
fi

echo Publishing Scoop version $source_repo version $ver to $target_repo from $dist_dir...

HASH_WINDOWS_AMD64=$(sha256sum $dist_dir/blinkdisk-$ver-windows-x64.zip | cut -f 1 -d " ")
tmpdir=$(mktemp -d)
git clone https://$GITHUB_TOKEN@github.com/$target_repo.git $tmpdir

cat tools/scoop-blinkdisk.json.template | \
   sed "s/VERSION/$ver/g" | \
   sed "s!SOURCE_REPO!$source_repo!g" | \
   sed "s/HASH_WINDOWS_AMD64/$HASH_WINDOWS_AMD64/g" > $tmpdir/blinkdisk.json

(cd $tmpdir && git add blinkdisk.json && git -c "user.name=BlinkDisk Builder" -c "user.email=builder@blinkdisk.com" commit -m "Scoop update for blinkdisk version v$ver" && git push)
rm -rf "$tmpdir"
