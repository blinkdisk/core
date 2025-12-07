#!/bin/bash

EXCLUDES=("./README.md" "./tools/fork/rename-blinkdisk.sh" "./.github/workflows/rename-blinkdisk.yml")

build_exclude_expr() {
  local expr=()
  for ex in "${EXCLUDES[@]}"; do
    expr+=(-path "$ex" -prune -o)
  done
  echo "${expr[@]}"
}

replace_contents() {
  local search="$1"
  local replace="$2"

  local exclude_expr
  exclude_expr=$(build_exclude_expr)

  eval find . -type d -name .git -prune -o -type f $exclude_expr -print | while read -r file; do
    if [ -f "$file" ]; then
      sed -i "s#${search}#${replace}#g" "$file"
    fi
  done
}

rename_files() {
  local search="$1"
  local replace="$2"

  local exclude_expr
  exclude_expr=$(build_exclude_expr)

  eval find . -type d -name .git -prune -o -type f $exclude_expr -print | sort -r | while read -r file; do
    dir=$(dirname "$file")
    base=$(basename "$file")
    if [[ "$base" == *"$search"* ]]; then
      new_base="${base//$search/$replace}"
      mv "$file" "$dir/$new_base"
    fi
  done
}

replace_contents "github.com:kopia/kopia.git" "github.com:blinkdisk/core.git"
replace_contents "github.com/kopia/kopia" "github.com/blinkdisk/core"
replace_contents "https://github.com/kopia/kopia" "https://github.com/blinkdisk/core"
replace_contents "http://kopia.github.io" "https://blinkdisk.com"
replace_contents "https://kopia.github.io" "https://blinkdisk.com"
replace_contents "http://kopia.io" "https://blinkdisk.com"
replace_contents "https://kopia.io" "https://blinkdisk.com"
replace_contents "kopia.io" "blinkdisk.com"

replace_contents "KOPIA_UI" "BLINKDISK_UI"
replace_contents "KOPIA" "BLINKDISK"
replace_contents "KopiaUI" "BlinkDiskUI"
replace_contents "Kopia UI" "BlinkDisk UI"
replace_contents "Kopia" "BlinkDisk"
replace_contents "kopia-ui" "blinkdisk-ui"
replace_contents "kopia" "blinkdisk"

rename_files "KOPIA_UI" "BLINKDISK_UI"
rename_files "KOPIA" "BLINKDISK"
rename_files "KopiaUI" "BlinkDiskUI"
rename_files "Kopia UI" "BlinkDisk UI"
rename_files "Kopia" "BlinkDisk"
rename_files "kopia-ui" "blinkdisk-ui"
rename_files "kopia" "blinkdisk"
