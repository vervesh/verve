#!/bin/bash
# claude.sh — Run Claude Code and parse streaming JSON output

# Depends on: log.sh (sourced by entrypoint.sh)

run_claude() {
    local prompt="$1"

    CLAUDE_MODEL="${CLAUDE_MODEL:-sonnet}"

    log_agent "Starting Claude Code session..."
    if [ -n "${TASK_TITLE}" ]; then
        log_agent "Task: ${TASK_TITLE}"
    fi
    log_agent "Description: ${TASK_DESCRIPTION}"
    log_blank
    log_agent "Using model: ${CLAUDE_MODEL}"

    # Use pipefail so we capture claude's exit code through the pipe.
    # Without this, a claude failure (e.g. auth error) is masked by _parse_stream
    # succeeding, and the script continues as if nothing went wrong.
    # We temporarily disable set -e to capture the exit code ourselves.
    local claude_exit=0
    set -o pipefail
    set +e
    claude --output-format stream-json --verbose --dangerously-skip-permissions \
        --model "${CLAUDE_MODEL}" "${prompt}" 2>&1 | _parse_stream
    claude_exit=$?
    set -e
    set +o pipefail

    log_blank

    if [ "$claude_exit" -ne 0 ]; then
        log_agent "Claude Code session failed with exit code ${claude_exit}"
        return "$claude_exit"
    fi

    log_agent "Claude Code session completed"
}

_parse_stream() {
    while IFS= read -r line; do
        [ -z "$line" ] && continue

        if ! echo "$line" | jq -e . >/dev/null 2>&1; then
            echo "$line"
            continue
        fi

        local event_type
        event_type=$(echo "$line" | jq -r '.type // empty' 2>/dev/null)

        case "$event_type" in
            assistant) _handle_assistant_event "$line" ;;
            result)    _handle_result_event "$line" ;;
        esac
    done
}

_handle_assistant_event() {
    local line="$1"
    local content_type
    content_type=$(echo "$line" | jq -r '.message.content[0].type // empty' 2>/dev/null)

    case "$content_type" in
        thinking)
            local text
            text=$(echo "$line" | jq -r '.message.content[0].thinking // empty' 2>/dev/null)
            [ -n "$text" ] && log_think "$text"
            ;;
        text)
            local text
            text=$(echo "$line" | jq -r '.message.content[0].text // empty' 2>/dev/null)
            [ -n "$text" ] && log_claude "$text"
            ;;
        tool_use)
            local name input detail
            name=$(echo "$line" | jq -r '.message.content[0].name // empty' 2>/dev/null)
            input=$(echo "$line" | jq -c '.message.content[0].input // empty' 2>/dev/null)
            detail=$(_tool_detail "$name" "$input")
            if [ -n "$name" ]; then
                if [ -n "$detail" ]; then
                    log_tool "$name: $detail"
                else
                    log_tool "Using: $name"
                fi
            fi
            ;;
    esac
}

_tool_detail() {
    local name="$1" input="$2"
    [ -z "$input" ] || [ "$input" = "null" ] || [ "$input" = '""' ] && return

    case "$name" in
        Bash)
            local cmd
            cmd=$(echo "$input" | jq -r '.command // empty' 2>/dev/null)
            if [ -n "$cmd" ]; then
                # Truncate long commands to keep logs readable
                if [ ${#cmd} -gt 200 ]; then
                    cmd="${cmd:0:200}..."
                fi
                echo "$cmd"
            fi
            ;;
        Read)
            echo "$input" | jq -r '.file_path // empty' 2>/dev/null
            ;;
        Edit)
            echo "$input" | jq -r '.file_path // empty' 2>/dev/null
            ;;
        Write)
            echo "$input" | jq -r '.file_path // empty' 2>/dev/null
            ;;
        Grep)
            local pattern path
            pattern=$(echo "$input" | jq -r '.pattern // empty' 2>/dev/null)
            path=$(echo "$input" | jq -r '.path // empty' 2>/dev/null)
            if [ -n "$pattern" ] && [ -n "$path" ]; then
                echo "\"$pattern\" in $path"
            elif [ -n "$pattern" ]; then
                echo "\"$pattern\""
            fi
            ;;
        Glob)
            local pattern path
            pattern=$(echo "$input" | jq -r '.pattern // empty' 2>/dev/null)
            path=$(echo "$input" | jq -r '.path // empty' 2>/dev/null)
            if [ -n "$pattern" ] && [ -n "$path" ]; then
                echo "$pattern in $path"
            elif [ -n "$pattern" ]; then
                echo "$pattern"
            fi
            ;;
        WebFetch)
            echo "$input" | jq -r '.url // empty' 2>/dev/null
            ;;
        WebSearch)
            echo "$input" | jq -r '.query // empty' 2>/dev/null
            ;;
        Task)
            local desc subagent
            desc=$(echo "$input" | jq -r '.description // empty' 2>/dev/null)
            subagent=$(echo "$input" | jq -r '.subagent_type // empty' 2>/dev/null)
            if [ -n "$desc" ] && [ -n "$subagent" ]; then
                echo "[$subagent] $desc"
            elif [ -n "$desc" ]; then
                echo "$desc"
            fi
            ;;
        TodoWrite)
            local count
            count=$(echo "$input" | jq '.todos | length' 2>/dev/null)
            if [ -n "$count" ] && [ "$count" != "null" ]; then
                echo "$count items"
            fi
            ;;
    esac
}

_handle_result_event() {
    local line="$1"
    local text
    text=$(echo "$line" | jq -r '.result // empty' 2>/dev/null)
    if [ -n "$text" ] && [ "$text" != "null" ]; then
        log_result "$text"
    fi

    local cost
    cost=$(echo "$line" | jq -r '.total_cost_usd // empty' 2>/dev/null)
    if [ -n "$cost" ] && [ "$cost" != "null" ] && [ "$cost" != "0" ]; then
        echo "VERVE_COST:${cost}"
    fi
}
