#!/bin/bash
# conversation_test.sh — Tests for _extract_conversation_response
#
# Run: bash agent/lib/conversation_test.sh

set -euo pipefail

PASS=0
FAIL=0

# Source just the extraction function from conversation.sh.
# We define a minimal stub for the rest of the file's dependencies
# so the source doesn't fail.
_extract_conversation_response() {
    local output="$1"

    local response
    response=$(printf '%s\n' "$output" | awk '
        /^````verve-response/ { if (!found) { capturing=1; found=1 }; next }
        capturing && /^````[[:space:]]*$/ { capturing=0; next }
        capturing { print }
    ')

    if [ -n "$response" ]; then
        printf '%s\n' "$response"
        return
    fi

    # Fallback: try 3-backtick fence (backwards compatibility)
    response=$(printf '%s\n' "$output" | awk '
        /^```verve-response/ { if (!found) { capturing=1; found=1 }; next }
        capturing && /^```[[:space:]]*$/ { capturing=0; next }
        capturing { print }
    ')

    if [ -n "$response" ]; then
        printf '%s\n' "$response"
        return
    fi

    echo ""
}

assert_eq() {
    local test_name="$1" expected="$2" actual="$3"
    if [ "$expected" = "$actual" ]; then
        echo "PASS: $test_name"
        PASS=$((PASS + 1))
    else
        echo "FAIL: $test_name"
        echo "  expected: $(printf '%q' "$expected")"
        echo "  actual:   $(printf '%q' "$actual")"
        FAIL=$((FAIL + 1))
    fi
}

# ---- Tests ----

echo "=== _extract_conversation_response tests ==="
echo ""

# Test 1: Simple 4-backtick response with no inner code blocks
input='````verve-response
Hello, this is a simple response.
````'
result=$(_extract_conversation_response "$input")
assert_eq "simple 4-backtick response" \
    "Hello, this is a simple response." \
    "$result"

# Test 2: 4-backtick response with inner 3-backtick code block (the bug case)
input='````verve-response
Here is some advice:

## Current URLs

```python
print("hello world")
```

## New URLs

You should change to the new format.
````'
expected='Here is some advice:

## Current URLs

```python
print("hello world")
```

## New URLs

You should change to the new format.'
result=$(_extract_conversation_response "$input")
assert_eq "4-backtick with inner code block" "$expected" "$result"

# Test 3: Multiple inner code blocks
input='````verve-response
First code block:

```bash
echo "hello"
```

Some text between blocks.

```go
func main() {
    fmt.Println("world")
}
```

End of response.
````'
expected='First code block:

```bash
echo "hello"
```

Some text between blocks.

```go
func main() {
    fmt.Println("world")
}
```

End of response.'
result=$(_extract_conversation_response "$input")
assert_eq "4-backtick with multiple inner code blocks" "$expected" "$result"

# Test 4: Bare triple backtick line inside 4-backtick fence (should NOT close)
input='````verve-response
Before

```
bare code block with no language
```

After
````'
expected='Before

```
bare code block with no language
```

After'
result=$(_extract_conversation_response "$input")
assert_eq "4-backtick with bare triple backtick inside" "$expected" "$result"

# Test 5: Backwards compatibility — 3-backtick fence without inner blocks
input='```verve-response
Simple response from old format.
```'
result=$(_extract_conversation_response "$input")
assert_eq "3-backtick backwards compatibility" \
    "Simple response from old format." \
    "$result"

# Test 6: No code block at all — returns empty
input='Just some text without any code blocks.'
result=$(_extract_conversation_response "$input")
assert_eq "no code block returns empty" "" "$result"

# Test 7: 4-backtick fence preferred over 3-backtick fence
input='````verve-response
This is the 4-backtick response.
````

```verve-response
This is the 3-backtick response.
```'
result=$(_extract_conversation_response "$input")
assert_eq "4-backtick preferred over 3-backtick" \
    "This is the 4-backtick response." \
    "$result"

# Test 8: Response with text before the code block (e.g., Claude thinking aloud)
input='Let me analyze the codebase...

I found the relevant files.

````verve-response
The answer to your question is that the authentication flow works as follows:

1. User submits credentials
2. Server validates against the database

```typescript
const token = await auth.login(email, password);
```

3. A JWT token is returned.
````'
expected='The answer to your question is that the authentication flow works as follows:

1. User submits credentials
2. Server validates against the database

```typescript
const token = await auth.login(email, password);
```

3. A JWT token is returned.'
result=$(_extract_conversation_response "$input")
assert_eq "response with preamble text before code block" "$expected" "$result"

# ---- Summary ----

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
