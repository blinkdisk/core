#!/bin/sh
export LISTEN_PID=$$
exec $BLINKDISK_ORIG_EXE "${@}"
