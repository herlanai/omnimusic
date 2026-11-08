#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/.."
app="$PWD/build/darwin-universal/OmniMusic.app"
version=$(/usr/libexec/PlistBuddy -c 'Print CFBundleShortVersionString' "$app/Contents/Info.plist")
volume="OmniMusic $version"
output="$PWD/build/darwin-universal/OmniMusic-$version.dmg"
work=$(mktemp -d /tmp/omnimusic-dmg.XXXXXX)
mount="$work/mount"
cleanup() {
  if mountpoint_output=$(hdiutil info) && [[ "$mountpoint_output" == *"$mount"* ]]; then
    hdiutil detach "$mount" -quiet || true
  fi
}
trap cleanup EXIT
GOCACHE=/tmp/omni-build go run ./tools/dmg-background
sips -s format tiff build/installer/background@2x.png --out build/installer/background.tiff >/dev/null
sips -s dpiWidth 144 -s dpiHeight 144 build/installer/background.tiff >/dev/null
mkdir -p "$work/stage/.background" "$mount"
ditto "$app" "$work/stage/OmniMusic.app"
ln -s /Applications "$work/stage/Applications"
cp build/installer/background.tiff "$work/stage/.background/background.tiff"
hdiutil create -srcfolder "$work/stage" -volname "$volume" -fs HFS+ -format UDRW "$work/writable.dmg" -quiet
hdiutil attach "$work/writable.dmg" -mountpoint "$mount" -nobrowse -quiet
osascript - "$mount" <<'APPLESCRIPT'
on run argv
  tell application "Finder"
    tell folder ((POSIX file (item 1 of argv)) as text)
      open
      set current view of container window to icon view
      set toolbar visible of container window to false
      set statusbar visible of container window to false
      set bounds of container window to {180, 140, 900, 628}
      set opts to icon view options of container window
      set arrangement of opts to not arranged
      set icon size of opts to 92
      set text size of opts to 13
      set background picture of opts to file ".background:background.tiff"
      set position of item "OmniMusic.app" of container window to {185, 263}
      set position of item "Applications" of container window to {535, 263}
      update without registering applications
      delay 2
      close
    end tell
  end tell
end run
APPLESCRIPT
sync
hdiutil detach "$mount" -quiet
hdiutil convert "$work/writable.dmg" -format UDZO -imagekey zlib-level=9 -o "$output" -ov -quiet
hdiutil verify "$output"
echo "$output"
