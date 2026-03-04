#!/bin/bash
# conversation.sh — Conversation agent that responds to user messages.
# Clones the repo for context, runs Claude with conversation history,
# submits the response to the API via the complete endpoint, and exits.

# Depends on: log.sh, validate.sh, git.sh, claude.sh (sourced by entrypoint.sh)

CONVERSATION_URL="${API_URL}/api/v1/agent/conversations/${CONVERSATION_ID}"

run_conversation() {
    log_header "Verve Conversation Agent Starting"
    echo "Conversation ID: ${CONVERSATION_ID}"
    echo "Repository: ${GITHUB_REPO}"
    echo "Title: ${CONVERSATION_TITLE}"
    echo "Model: ${CLAUDE_MODEL:-sonnet}"
    log_blank

    # Validate required env vars
    if [ -z "$CONVERSATION_ID" ] || [ -z "$API_URL" ]; then
        log_error "CONVERSATION_ID and API_URL are required for conversation"
        exit 1
    fi

    if [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$CLAUDE_CODE_OAUTH_TOKEN" ]; then
        log_error "ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN must be set"
        exit 1
    fi

    # Clone repo for context (read-only)
    configure_git
    clone_repo
    log_agent "Repository cloned for context analysis"
    log_blank

    _conversation_send_log "system: Processing message..."

    # Run conversation
    _conversation_run

    local result=$?

    if [ "$result" -eq 0 ]; then
        log_agent "Conversation processing completed successfully"
    else
        log_agent "Conversation processing failed"
        _conversation_complete_fail "conversation processing failed"
    fi
}

_conversation_run() {
    local prompt
    prompt=$(_build_conversation_prompt)

    log_agent "Running Claude for conversation response..."

    # Write raw stream-json output to a temp file so we can extract the
    # result afterwards. Use tee to also pipe through _parse_stream (from
    # claude.sh) which prints human-readable log lines to stdout in
    # real-time — these are captured by the Docker log streamer.
    local raw_output_file
    raw_output_file=$(mktemp /tmp/conversation_claude_raw.XXXXXX)

    local model="${CLAUDE_MODEL:-sonnet}"
    claude --output-format stream-json --verbose --dangerously-skip-permissions \
        --model "$model" "$prompt" 2>&1 \
        | tee "$raw_output_file" \
        | _parse_stream

    # Extract the result text from the saved raw output
    local output=""
    while IFS= read -r line; do
        [ -z "$line" ] && continue
        local event_type
        event_type=$(echo "$line" | jq -r '.type // empty' 2>/dev/null)
        if [ "$event_type" = "result" ]; then
            output=$(echo "$line" | jq -r '.result // empty' 2>/dev/null)
        fi
    done < "$raw_output_file"
    rm -f "$raw_output_file"

    log_blank
    log_agent "Claude Code session completed"

    if [ -z "$output" ]; then
        log_error "Claude produced no output"
        _conversation_send_log "system: Failed — Claude produced no output"
        return 1
    fi

    # Extract response from the verve-response code block if present,
    # otherwise use the full output as the response
    local response
    response=$(_extract_conversation_response "$output")

    if [ -z "$response" ]; then
        # Fall back to the full output if no code block was found
        response="$output"
    fi

    # Submit the response via the complete endpoint
    _conversation_complete_success "$response"
    _conversation_send_log "system: Response submitted."
}

_build_conversation_prompt() {
    # Write conversation messages to a temp file so we can safely include
    # them without shell quoting issues
    local messages_file
    messages_file=$(mktemp /tmp/conversation_messages.XXXXXX)
    printf '%s\n' "${CONVERSATION_MESSAGES}" > "$messages_file"

    # Format conversation history from JSON messages array
    local formatted_history=""
    formatted_history=$(jq -r '
        .[] |
        if .role == "user" then "User: " + .content
        elif .role == "assistant" then "Assistant: " + .content
        else .role + ": " + .content
        end
    ' "$messages_file" 2>/dev/null || echo "")
    rm -f "$messages_file"

    local prompt="You are a conversational assistant for discussing a software repository. You help users understand the codebase, answer questions about architecture, explain code patterns, and provide technical guidance.

## CRITICAL RULES — READ CAREFULLY

1. You must NEVER make code changes, create files, edit files, or modify the repository in any way.
2. You must NEVER create pull requests, branches, or commits.
3. You must NEVER implement features, fix bugs, or write code that modifies the codebase.
4. You may ONLY read files and analyze the codebase to inform your responses.
5. Your role is strictly informational and advisory — you discuss and explain, you do not act.

## Repository Context

You have access to the repository at $(pwd). Use tools to read files and explore the codebase structure to inform your answers."

    # Add repo context if available
    if [ -n "${REPO_SUMMARY}" ] || [ -n "${REPO_EXPECTATIONS}" ] || [ -n "${REPO_TECH_STACK}" ]; then
        prompt="${prompt}

## Repository Setup Context"
        if [ -n "${REPO_SUMMARY}" ]; then
            prompt="${prompt}

**Repository Summary:** ${REPO_SUMMARY}"
        fi
        if [ -n "${REPO_TECH_STACK}" ]; then
            prompt="${prompt}

**Tech Stack:** ${REPO_TECH_STACK}"
        fi
        if [ -n "${REPO_EXPECTATIONS}" ]; then
            prompt="${prompt}

**Repository Expectations:**
${REPO_EXPECTATIONS}"
        fi
    fi

    prompt="${prompt}

## Conversation Title

${CONVERSATION_TITLE}"

    # Add conversation history if present
    if [ -n "$formatted_history" ]; then
        prompt="${prompt}

## Previous Conversation

${formatted_history}"
    fi

    prompt="${prompt}

## Current User Message

${CONVERSATION_PENDING_MESSAGE}

## Response Instructions

Respond helpfully and informatively about the codebase. You may read any files in the repository to provide accurate answers. Reference specific file paths and line numbers when discussing code.

Wrap your final response in a \`\`\`verve-response\`\`\` code block so it can be parsed. Only the content inside this block will be sent to the user. Example:

\`\`\`verve-response
Your response here with markdown formatting.
\`\`\`

Remember: You are a read-only assistant. Do NOT modify any files or create any changes to the repository."

    echo "$prompt"
}

_extract_conversation_response() {
    local output="$1"

    # Extract content from the first ```verve-response code block
    local response
    response=$(printf '%s\n' "$output" | awk '
        /^```verve-response/ { if (!found) { capturing=1; found=1 }; next }
        capturing && /^```[[:space:]]*$/ { capturing=0; next }
        capturing { print }
    ')

    if [ -n "$response" ]; then
        printf '%s\n' "$response"
        return
    fi

    # No code block found — return empty to signal fallback
    echo ""
}

_conversation_complete_success() {
    local response="$1"

    # Write the response to a temp file to avoid shell argument size limits
    local response_file
    response_file=$(mktemp /tmp/conversation_response.XXXXXX)
    printf '%s\n' "$response" > "$response_file"

    local body
    body=$(jq -n --rawfile resp "$response_file" '{"success": true, "response": $resp}')
    rm -f "$response_file"

    local http_response
    http_response=$(curl -s -w "\n%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$body" \
        "${CONVERSATION_URL}/complete" 2>/dev/null) || true

    local http_code
    http_code=$(echo "$http_response" | tail -1)

    if [ -z "$http_code" ] || [ "$http_code" != "204" ]; then
        log_error "Failed to submit conversation response (HTTP ${http_code})"
    else
        log_agent "Conversation response submitted successfully"
    fi
}

_conversation_complete_fail() {
    local error_msg="${1:-conversation failed}"
    local body
    body=$(jq -n --arg err "$error_msg" '{"success": false, "error": $err}')

    curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$body" \
        "${CONVERSATION_URL}/complete" > /dev/null 2>&1 || true
}

_conversation_send_log() {
    local message="$1"
    local body
    body=$(jq -n --arg line "$message" '{"lines": [$line]}')

    curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$body" \
        "${CONVERSATION_URL}/logs" > /dev/null 2>&1 || true
}
