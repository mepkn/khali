#!/usr/bin/env bash
# Build khali.app and replace /Applications/Khali.app with it, then relaunch.
set -euo pipefail
cd "$(dirname "$0")/.."
export PATH="$HOME/go/bin:$PATH"

wails3 task package
pkill -x khali || true
rm -rf /Applications/Khali.app
# Move, not copy: a second khali.app in bin/ shows up in Spotlight too.
mv bin/khali.app /Applications/Khali.app
xattr -cr /Applications/Khali.app
open /Applications/Khali.app
echo "Installed /Applications/Khali.app"
