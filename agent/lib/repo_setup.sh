#!/bin/bash
# repo_setup.sh — Repository setup scanning agent.
# Clones the repo and runs Claude Code as a full agent to analyze
# the repository structure, tech stack, and configuration.
# All detection and analysis is performed by Claude — no programmatic
# file checking or tech-stack detection is done in this script.

# Depends on: log.sh, validate.sh, git.sh, claude.sh (sourced by entrypoint.sh)

SETUP_COMPLETE_URL="${API_URL}/api/v1/agent/repos/${REPO_ID}/setup-complete"

run_repo_setup() {
    log_header "Verve Repo Setup Scan Starting"
    echo "Repo ID: ${REPO_ID}"
    echo "Repository: ${GITHUB_REPO}"
    echo "Model: ${CLAUDE_MODEL:-sonnet}"
    log_blank

    # Validate required env vars
    if [ -z "$REPO_ID" ] || [ -z "$API_URL" ]; then
        log_error "REPO_ID and API_URL are required for repo setup scan"
        _setup_complete_fail
        exit 1
    fi

    if [ -z "$ANTHROPIC_API_KEY" ] && [ -z "$CLAUDE_CODE_OAUTH_TOKEN" ]; then
        log_error "ANTHROPIC_API_KEY or CLAUDE_CODE_OAUTH_TOKEN must be set"
        _setup_complete_fail
        exit 1
    fi

    # Clone repo (read-only)
    configure_git
    clone_repo
    log_agent "Repository cloned for analysis"
    log_blank

    # Check if repo is empty (no files besides .git) — this is the one
    # check we do before invoking Claude because there is nothing for the
    # agent to analyse in an empty repo.
    local file_count
    file_count=$(find . -not -path './.git/*' -not -path './.git' -not -path '.' -type f | head -5 | wc -l)

    if [ "$file_count" -eq 0 ]; then
        log_agent "Repository is empty (no source files found)"
        _setup_complete_empty
        return 0
    fi

    # Run Claude Code as a full agent to analyze the repository.
    # Claude will explore the codebase using its tools (Read, Glob, Grep, Bash)
    # and return structured JSON with the analysis results.
    log_agent "Running Claude Code agent to analyze repository..."

    _run_setup_analysis

    local analysis_result=$?

    if [ "$analysis_result" -eq 0 ]; then
        log_agent "Repository analysis completed successfully"
    else
        log_agent "Repository analysis failed"
        _setup_complete_fail
    fi

    log_blank
    log_header "Repo Setup Scan Completed"
}

_run_setup_analysis() {
    local prompt
    prompt=$(_build_setup_prompt)

    # Write raw stream-json output to a temp file so we can extract the
    # result afterwards. Use tee to also pipe through _parse_stream (from
    # claude.sh) which prints human-readable log lines to stdout in
    # real-time — these are captured by the Docker log streamer.
    local raw_output_file
    raw_output_file=$(mktemp /tmp/setup_claude_raw.XXXXXX)

    local model="${CLAUDE_MODEL:-sonnet}"

    local claude_exit=0
    set -o pipefail
    set +e
    claude --output-format stream-json --verbose --dangerously-skip-permissions \
        --model "$model" "$prompt" 2>&1 \
        | tee "$raw_output_file" \
        | _parse_stream
    claude_exit=$?
    set -e
    set +o pipefail

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

    if [ "$claude_exit" -ne 0 ]; then
        log_error "Claude Code session failed with exit code ${claude_exit}"
        return 1
    fi

    if [ -z "$output" ]; then
        log_error "Claude produced no output"
        return 1
    fi

    # Parse the structured analysis from Claude's output
    local analysis_json
    analysis_json=$(_extract_setup_analysis "$output")

    if [ -z "$analysis_json" ] || [ "$analysis_json" = "null" ] || [ "$analysis_json" = "{}" ]; then
        log_error "Could not extract analysis from Claude output"
        return 1
    fi

    # Extract fields from the analysis JSON and submit results
    local summary tech_stack has_code has_claude_md has_readme needs_setup
    summary=$(printf '%s\n' "$analysis_json" | jq -r '.summary // ""')
    tech_stack=$(printf '%s\n' "$analysis_json" | jq -c '.tech_stack // []')
    has_code=$(printf '%s\n' "$analysis_json" | jq -r '.has_code // true')
    has_claude_md=$(printf '%s\n' "$analysis_json" | jq -r '.has_claude_md // false')
    has_readme=$(printf '%s\n' "$analysis_json" | jq -r '.has_readme // false')
    needs_setup=$(printf '%s\n' "$analysis_json" | jq -r '.needs_setup // true')

    log_agent "Summary: ${summary}"
    log_agent "Tech stack: $(printf '%s\n' "$tech_stack" | jq -r 'join(", ")')"
    log_agent "Has code: ${has_code}, Has CLAUDE.md: ${has_claude_md}, Has README: ${has_readme}"
    log_agent "Needs setup: ${needs_setup}"

    _setup_complete "$summary" "$tech_stack" "$has_code" "$has_claude_md" "$has_readme" "$needs_setup"
}

_build_setup_prompt() {
    cat <<'PROMPT'
You are a repository analysis agent. Your job is to thoroughly analyze this repository and produce a structured assessment.

You have access to the full repository at the current working directory. Use your tools (Read, Glob, Grep, Bash) to explore the codebase. Do NOT guess — actually look at the files.

## What to Analyze

1. **Repository overview**: What does this project do? Summarize in 2-3 sentences.

2. **Tech stack detection**: Identify ALL technologies, languages, frameworks, and tools used. Look at:
   - Build files (go.mod, package.json, Cargo.toml, pyproject.toml, requirements.txt, Gemfile, pom.xml, build.gradle, composer.json, *.csproj, Package.swift, etc.)
   - Config files (tsconfig.json, .eslintrc, .prettierrc, Makefile, Dockerfile, docker-compose.yml, terraform files, CI configs, etc.)
   - Framework indicators (next.config.*, nuxt.config.*, svelte.config.*, angular.json, vite.config.*, rails config, django manage.py, etc.)
   - Source file extensions to identify languages used
   - Any other technology indicators

3. **Documentation check**:
   - Does a CLAUDE.md file exist? (Check root and common locations)
   - Does a README file exist? (README.md, README, readme.md, README.rst, etc.)

4. **Coding standards assessment**: Are coding standards and patterns well-established? Look for:
   - Linter/formatter configs (.eslintrc, .prettierrc, .golangci.yml, rustfmt.toml, etc.)
   - CI/CD configs (.github/workflows, .gitlab-ci.yml, Jenkinsfile, etc.)
   - CLAUDE.md with project conventions
   - Consistent code structure and patterns
   - Test files and testing conventions

5. **Needs setup determination**: Does the repository need additional setup? A repo needs setup if:
   - It has no CLAUDE.md file (AI coding agents won't have project context)
   - It has no README (developers won't have documentation)
   - It has code but no clear coding standards or conventions documented
   - A repo does NOT need setup if it has a CLAUDE.md + README + well-established patterns

## Output Format

After your analysis, output your findings as a JSON object wrapped in a markdown code block with the language tag `verve-setup`. The JSON must have exactly these fields:

```verve-setup
{
  "summary": "A 2-3 sentence summary of what this project does and its purpose.",
  "tech_stack": ["Go", "PostgreSQL", "Docker", "React", "TypeScript"],
  "has_code": true,
  "has_claude_md": false,
  "has_readme": true,
  "needs_setup": true
}
```

Field descriptions:
- `summary` (string): Concise 2-3 sentence description of the project
- `tech_stack` (string array): All detected technologies, languages, frameworks, and tools
- `has_code` (boolean): Whether the repository contains source code
- `has_claude_md` (boolean): Whether a CLAUDE.md file exists
- `has_readme` (boolean): Whether a README file exists (any format)
- `needs_setup` (boolean): Whether the repo needs additional setup/configuration for AI coding agents

Be thorough in your analysis but concise in your output. Actually explore the codebase — don't guess from file names alone.
PROMPT
}

_extract_setup_analysis() {
    local output="$1"

    # Extract content from the ```verve-setup code block.
    local analysis
    analysis=$(printf '%s\n' "$output" | awk '
        /^```verve-setup/ { if (!found) { capturing=1; found=1 }; next }
        capturing && /^```[[:space:]]*$/ { capturing=0; next }
        capturing { print }
    ')

    if [ -n "$analysis" ]; then
        # Validate that extracted content is a JSON object
        local validated
        validated=$(printf '%s\n' "$analysis" | jq -e 'if type == "object" then . else error("not an object") end' 2>/dev/null)
        if [ -n "$validated" ]; then
            printf '%s\n' "$validated"
            return
        fi
    fi

    echo "{}"
}

_setup_complete_empty() {
    local body
    body=$(jq -n '{
        "success": true,
        "summary": "Empty repository with no source files.",
        "tech_stack": [],
        "has_code": false,
        "has_claude_md": false,
        "has_readme": false,
        "needs_setup": true
    }')

    log_agent "Reporting setup scan results (empty repo)..."
    _post_setup_complete "$body"
}

_setup_complete() {
    local summary="$1"
    local tech_stack="$2"
    local has_code="$3"
    local has_claudemd="$4"
    local has_readme="$5"
    local needs_setup="$6"

    local body
    body=$(jq -n \
        --arg summary "$summary" \
        --argjson tech_stack "$tech_stack" \
        --argjson has_code "$has_code" \
        --argjson has_claude_md "$has_claudemd" \
        --argjson has_readme "$has_readme" \
        --argjson needs_setup "$needs_setup" \
        '{
            "success": true,
            "summary": $summary,
            "tech_stack": $tech_stack,
            "has_code": $has_code,
            "has_claude_md": $has_claude_md,
            "has_readme": $has_readme,
            "needs_setup": $needs_setup
        }')

    log_agent "Reporting setup scan results..."
    _post_setup_complete "$body"
}

_post_setup_complete() {
    local body="$1"

    local response
    response=$(curl -s -w "\n%{http_code}" -X POST \
        -H "Content-Type: application/json" \
        -d "$body" \
        "${SETUP_COMPLETE_URL}" 2>/dev/null) || true

    local http_code
    http_code=$(echo "$response" | tail -1)

    if [ -z "$http_code" ] || [ "$http_code" != "204" ]; then
        log_error "Failed to submit setup results (HTTP ${http_code})"
    else
        log_agent "Setup scan results submitted successfully"
    fi
}

_setup_complete_fail() {
    local body='{"success": false}'

    curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$body" \
        "${SETUP_COMPLETE_URL}" > /dev/null 2>&1 || true
}
