# JSON Patch Fuzz Testing

This directory contains a comprehensive fuzz testing system that validates JavaScript-Go compatibility for the JSON patch library, with special focus on Unicode string handling.

## Files

- **`test.mjs`** - JavaScript fuzz test generator using Immer and json-joy
- **`run-fuzz.sh`** - Test runner script that coordinates JS-Go communication  
- **`package.json`** - Node.js dependencies for testing

## Quick Start

```bash
# Install dependencies
npm install

# Run 100 fuzz test cases
./run-fuzz.sh 100

# Run 500 test cases for extended testing  
./run-fuzz.sh 500
```

## How It Works

1. **Document Generation**: Creates random complex documents with Unicode strings
2. **Mutation**: Uses Immer to apply random mutations and generate patches
3. **Conversion**: Converts Immer patches to json-joy operations
4. **Validation**: Spawns Go process to apply operations and compare results
5. **Reporting**: Shows success rate and detailed failure analysis

## Test Coverage

- ✅ Complex nested document structures
- ✅ Unicode strings (emoji, CJK, Arabic, mathematical symbols)
- ✅ Mixed data types (strings, numbers, arrays, objects)
- ✅ Random operation sequences  
- ✅ Cross-language compatibility validation

## Environment Variables

- `TEST_HARNESS_PATH` - Path to Go test harness binary (set automatically by run-fuzz.sh)
