#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$SCRIPT_DIR/pkg/webui/frontend"
OUTPUT_BIN="${1:-$SCRIPT_DIR/nekobot}"

echo "🔨 Building nekobot..."
echo "   Output: $OUTPUT_BIN"
echo ""

# Build frontend
echo "📦 Building frontend..."
cd "$FRONTEND_DIR"

# Detect package manager
if command -v pnpm &> /dev/null; then
    PKG_MANAGER="pnpm"
elif command -v npm &> /dev/null; then
    PKG_MANAGER="npm"
else
    echo "   ❌ Neither pnpm nor npm found"
    exit 1
fi
echo "   Using: $PKG_MANAGER"

if [ ! -d "node_modules" ]; then
    echo "   Installing dependencies..."
    $PKG_MANAGER install
fi
$PKG_MANAGER run build
echo "   ✅ Frontend built"
echo ""

# Build Go binary
echo "🏗️  Building Go binary..."
cd "$SCRIPT_DIR"
go build -o "$OUTPUT_BIN" ./cmd/nekobot
echo "   ✅ Binary built: $OUTPUT_BIN"
echo ""

echo "🎉 Build complete!"
