#!/bin/bash

# Chaos monkey / fuzz test runner for go-jsonpatch
# Tests JS-Go compatibility with Unicode strings and complex operations

set -e

echo "🐵 JSON Patch Chaos Monkey Test Runner"
echo "======================================"

# Check if Go is available
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed or not in PATH"
    exit 1
fi

# Check if Node.js is available
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed or not in PATH"
    exit 1
fi

# Change to test directory
cd "$(dirname "$0")"

# Check if node_modules exists
if [ ! -d "node_modules" ]; then
    echo "📦 Installing Node.js dependencies..."
    npm install
fi

# Build the Go test harness
echo "🔨 Building Go test harness..."
TEMP_HARNESS=$(mktemp -t test-harness-XXXXXX)
go build -o "$TEMP_HARNESS" ../../cmd/test-harness/main.go

# Clean up on exit
trap "rm -f '$TEMP_HARNESS'" EXIT

# Run the fuzz tests
NUM_TESTS=${1:-100}
echo "🚀 Running $NUM_TESTS fuzz tests..."
echo "   Each test generates a random document with Unicode strings,"
echo "   applies random mutations using Immer, converts to json-joy operations,"
echo "   and verifies Go produces identical results."
echo ""

# Export the harness path for the JS test
export TEST_HARNESS_PATH="$TEMP_HARNESS"
node test.mjs "$NUM_TESTS"

echo ""
echo "✅ Fuzz testing complete!"
echo "💡 To run with a different number of tests: ./run-fuzz.sh 500"