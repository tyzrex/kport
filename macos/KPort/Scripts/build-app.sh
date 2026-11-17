#!/bin/bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
APP="$ROOT/dist/KPort.app"
cd "$ROOT/macos/KPort"
swift build -c release --product KPort
rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp .build/release/KPort "$APP/Contents/MacOS/KPort"
CGO_ENABLED=0 GOOS=darwin GOARCH="$(go env GOARCH)" go build -trimpath -ldflags="-s -w" -o "$APP/Contents/Resources/kport" "$ROOT/cmd/kport"
ICONSET="$ROOT/macos/KPort/.build/KPort.iconset"
mkdir -p "$ICONSET"
swift "$ROOT/macos/KPort/Assets/make-icon.swift" "$ROOT/macos/KPort/.build/kport-icon.png"
for size in 16 32 128 256 512; do
    sips -z "$size" "$size" "$ROOT/macos/KPort/.build/kport-icon.png" --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
    double=$((size * 2))
    sips -z "$double" "$double" "$ROOT/macos/KPort/.build/kport-icon.png" --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/AppIcon.icns"
cat > "$APP/Contents/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key><string>KPort</string>
    <key>CFBundleIdentifier</key><string>com.tyzrex.kport</string>
    <key>CFBundleName</key><string>kport</string>
    <key>CFBundleIconFile</key><string>AppIcon</string>
    <key>CFBundlePackageType</key><string>APPL</string>
    <key>LSMinimumSystemVersion</key><string>13.0</string>
    <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
PLIST
codesign --force --deep --sign - "$APP"
echo "Built $APP"
