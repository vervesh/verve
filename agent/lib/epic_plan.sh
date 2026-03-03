#!/bin/bash
# epic_plan.sh — Epic planning agent that runs as a short-lived container.
# Clones the repo for context, runs Claude to generate proposed tasks,
# submits them to the API via the complete endpoint, and exits.
# Change requests (user feedback) cause a new planning run to be dispatched
# by the queue — no durable session is needed.

# Depends on: log.sh, validate.sh, git.sh, claude.sh (sourced by entrypoint.sh)

POLL_URL="${API_URL}/api/v1/agent/epics/${EPIC_ID}"

run_epic_planning() {
    log_header "Verve Epic Planning Agent Starting"
    echo "Epic ID: ${EPIC_ID}"
    echo "Repository: ${GITHUB_REPO}"
    echo "Title: ${EPIC_TITLE}"
    echo "Description: ${EPIC_DESCRIPTION}"
    [ -n "${EPIC_PLANNING_PROMPT}" ] && echo "Planning Prompt: ${EPIC_PLANNING_PROMPT}"
    [ -n "${EPIC_FEEDBACK}" ] && echo "Change Request: ${EPIC_FEEDBACK}"
    echo "Model: ${CLAUDE_MODEL:-sonnet}"
    log_blank

    # Validate required env vars
    if [ -z "$EPIC_ID" ] || [ -z "$API_URL" ]; then
        log_error "EPIC_ID and API_URL are required for epic planning"
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

    # Determine if this is a change request or initial planning
    if [ -n "${EPIC_FEEDBACK}" ]; then
        _epic_send_log "system: Re-planning based on user feedback..."
    else
        _epic_send_log "system: Planning started. Analyzing epic and generating task breakdown..."
    fi

    # Run planning — feedback and previous plan are passed as context if present
    _epic_run_planning "${EPIC_FEEDBACK}" "${EPIC_PREVIOUS_PLAN}"

    local plan_result=$?

    if [ "$plan_result" -eq 0 ]; then
        log_agent "Epic planning completed successfully"
    else
        log_agent "Epic planning failed"
        _epic_complete_fail
    fi
}

_epic_run_planning() {
    local feedback="$1"
    local previous_plan="$2"

    local prompt
    prompt=$(_build_epic_prompt "$feedback" "$previous_plan")

    log_agent "Running Claude for epic planning..."

    # Write raw stream-json output to a temp file so we can extract the
    # result afterwards. Use tee to also pipe through _parse_stream (from
    # claude.sh) which prints human-readable log lines to stdout in
    # real-time — these are captured by the Docker log streamer.
    local raw_output_file
    raw_output_file=$(mktemp /tmp/epic_claude_raw.XXXXXX)

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
        _epic_send_log "system: Planning failed — Claude produced no output"
        return 1
    fi

    # Parse proposed tasks from Claude's output
    local tasks_json
    tasks_json=$(_extract_proposed_tasks "$output")

    if [ -z "$tasks_json" ] || [ "$tasks_json" = "null" ] || [ "$tasks_json" = "[]" ]; then
        log_error "Could not extract proposed tasks from Claude output"
        _epic_send_log "system: Planning produced no tasks. The agent's response has been logged."
        return 1
    fi

    local task_count
    task_count=$(printf '%s\n' "$tasks_json" | jq 'length')
    log_agent "Generated ${task_count} proposed tasks"

    # Submit proposed tasks via the complete endpoint and exit
    _epic_complete_success "$tasks_json"
    _epic_send_log "system: Planning complete. Proposed ${task_count} tasks."
}

_build_epic_prompt() {
    local feedback="$1"
    local previous_plan="$2"

    local prompt="You are a technical project planner. Your job is to analyze a software epic and break it down into concrete, actionable implementation tasks.

## Epic Details

**Title:** ${EPIC_TITLE}

**Description:**
${EPIC_DESCRIPTION}"

    if [ -n "${EPIC_PLANNING_PROMPT}" ]; then
        prompt="${prompt}

**Additional Planning Instructions:**
${EPIC_PLANNING_PROMPT}"
    fi

    if [ -n "$previous_plan" ] && [ -n "$feedback" ]; then
        prompt="${prompt}

**Previous Task Breakdown (for reference):**
\`\`\`json
${previous_plan}
\`\`\`

**User Feedback on Previous Plan:**
${feedback}

Please revise the task breakdown based on this feedback. You should produce a completely new set of tasks that incorporates the feedback."
    elif [ -n "$feedback" ]; then
        prompt="${prompt}

**User Feedback on Previous Plan:**
${feedback}

Please revise the task breakdown based on this feedback."
    fi

    prompt="${prompt}

## Repository Context

You have access to the repository at $(pwd). Use the tools available to explore the codebase structure, read key files, and understand the architecture before creating your plan.

## Task Sizing Guidelines

Each task should represent a **substantial, logical piece of work** — not a tiny atomic change. Think in terms of meaningful features or vertical slices that span across components.

**Good task sizing (aim for this):**
- \"Add database schema and migrations for user profiles\" (covers migration, model, repository)
- \"Implement backend API endpoints for user profiles\" (covers handler, routes, request/response types, validation)
- \"Build frontend UI for user profile management\" (covers components, state, API integration)
- \"Add authentication middleware and integrate with existing routes\" (covers the full auth layer)

**Bad task sizing (too small — avoid this):**
- \"Add a single constant to the config file\"
- \"Create the User struct\"
- \"Add one helper function\"
- \"Add a single database column\"
- \"Write the interface definition\"

A single task should typically touch **multiple files** and produce a **coherent, self-contained unit of functionality**. It is fine for a task to span across layers (e.g. database + backend, or backend + frontend) as long as it forms a logical unit. Aim for roughly **3-7 tasks** for a typical epic. Only create more if the epic is genuinely large in scope.

Each task will be completed by an AI coding agent that can handle complex, multi-file changes. Do not break work into pieces smaller than what a senior developer would consider a single pull request.

## Output Requirements

After analyzing the codebase and the epic, output a JSON array of proposed tasks. Each task should have:
- \`temp_id\`: A unique identifier like \"task_1\", \"task_2\", etc.
- \`title\`: A concise, actionable title (imperative form, e.g. \"Add user authentication middleware\")
- \`description\`: Detailed description of what needs to be done, including relevant file paths and implementation details. Be thorough — include enough context and specifics that an AI agent can implement the task without further clarification.
- \`depends_on_temp_ids\`: Array of temp_ids this task depends on (empty array if none)
- \`acceptance_criteria\`: Array of specific, testable criteria for completion

Output the tasks as a JSON array wrapped in a markdown code block with the language tag \`verve-tasks\`. Example:

\`\`\`verve-tasks
[
  {
    \"temp_id\": \"task_1\",
    \"title\": \"Add database schema and repository for user profiles\",
    \"description\": \"Create the database migration, domain model, and repository implementation for user profiles. This includes the migration file with the profiles table, the Go struct, the repository interface methods, and the PostgreSQL/SQLite implementations...\",
    \"depends_on_temp_ids\": [],
    \"acceptance_criteria\": [\"Migration creates profiles table with required columns\", \"Repository supports CRUD operations\", \"Both PostgreSQL and SQLite implementations work\"]
  },
  {
    \"temp_id\": \"task_2\",
    \"title\": \"Implement user profile API endpoints\",
    \"description\": \"Build the HTTP handler layer for user profiles including all REST endpoints, request/response types, input validation, and route registration...\",
    \"depends_on_temp_ids\": [\"task_1\"],
    \"acceptance_criteria\": [\"GET/POST/PUT/DELETE endpoints work\", \"Input validation returns proper errors\", \"Routes registered under /api/v1/profiles\"]
  }
]
\`\`\`

Order tasks by dependency (tasks with no dependencies first). Remember: prefer fewer, larger tasks over many small ones. Each task should be a meaningful chunk of work, not a tiny atomic change."

    echo "$prompt"
}

_extract_proposed_tasks() {
    local output="$1"

    # Extract content from the first ```verve-tasks code block.
    local tasks
    tasks=$(printf '%s\n' "$output" | awk '
        /^```verve-tasks/ { if (!found) { capturing=1; found=1 }; next }
        capturing && /^```[[:space:]]*$/ { capturing=0; next }
        capturing { print }
    ')

    if [ -n "$tasks" ]; then
        # Validate that extracted content is a single JSON array
        local validated
        validated=$(printf '%s\n' "$tasks" | jq -e 'if type == "array" then . else error("not an array") end' 2>/dev/null)
        if [ -n "$validated" ]; then
            printf '%s\n' "$validated"
            return
        fi
    fi

    echo "[]"
}

_epic_complete_success() {
    local tasks_json="$1"

    # Write the tasks JSON to a temp file and use jq's --slurpfile to
    # avoid shell argument size limits and special character issues.
    local tasks_file
    tasks_file=$(mktemp /tmp/epic_tasks.XXXXXX)
    printf '%s\n' "$tasks_json" > "$tasks_file"

    local body
    body=$(jq -n --slurpfile tasks "$tasks_file" '{"success": true, "tasks": $tasks[0]}')
    rm -f "$tasks_file"

    local response
    response=$(curl -s -w "\n%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$body" \
        "${POLL_URL}/complete" 2>/dev/null) || true

    local http_code
    http_code=$(echo "$response" | tail -1)

    if [ -z "$http_code" ] || [ "$http_code" != "204" ]; then
        log_error "Failed to submit planning result (HTTP ${http_code})"
    else
        log_agent "Planning result submitted successfully"
    fi
}

_epic_complete_fail() {
    local body='{"success": false, "error": "planning failed"}'

    curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$body" \
        "${POLL_URL}/complete" > /dev/null 2>&1 || true
}

_epic_send_log() {
    local message="$1"
    local body
    body=$(jq -n --arg line "$message" '{"lines": [$line]}')

    curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$body" \
        "${POLL_URL}/logs" > /dev/null 2>&1 || true
}
