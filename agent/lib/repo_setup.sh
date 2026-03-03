#!/bin/bash
# repo_setup.sh — Repository setup scanning agent.
# Clones the repo, analyzes its structure and configuration,
# invokes Claude for a summary, and reports results back to the API.

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

    # Analyze repository
    local has_code=false
    local has_claudemd=false
    local has_readme=false
    local needs_setup=true
    local tech_stack="[]"
    local summary=""
    local claudemd_contents=""
    local readme_contents=""

    # Check if repo is empty (no files besides .git)
    local file_count
    file_count=$(find . -not -path './.git/*' -not -path './.git' -not -path '.' -type f | head -5 | wc -l)

    if [ "$file_count" -eq 0 ]; then
        log_agent "Repository is empty (no source files found)"
        has_code=false
        needs_setup=true
        summary="Empty repository with no source files."
        _setup_complete "$summary" "$tech_stack" "$has_code" "$has_claudemd" "$has_readme" "$needs_setup"
        return 0
    fi

    has_code=true
    log_agent "Repository has source files"

    # Check for CLAUDE.md
    if [ -f "CLAUDE.md" ]; then
        has_claudemd=true
        claudemd_contents=$(head -500 CLAUDE.md)
        log_agent "Found CLAUDE.md ($(wc -l < CLAUDE.md) lines)"
    else
        log_agent "No CLAUDE.md found"
    fi

    # Check for README
    if [ -f "README.md" ]; then
        has_readme=true
        readme_contents=$(head -500 README.md)
        log_agent "Found README.md ($(wc -l < README.md) lines)"
    elif [ -f "README" ]; then
        has_readme=true
        readme_contents=$(head -500 README)
        log_agent "Found README ($(wc -l < README) lines)"
    elif [ -f "readme.md" ]; then
        has_readme=true
        readme_contents=$(head -500 readme.md)
        log_agent "Found readme.md ($(wc -l < readme.md) lines)"
    else
        log_agent "No README found"
    fi

    # Detect tech stack
    local detected_stack=()
    _detect_tech_stack detected_stack
    tech_stack=$(_array_to_json "${detected_stack[@]}")
    log_agent "Detected tech stack: ${detected_stack[*]:-none}"

    # Generate file tree for context
    local tree_output
    tree_output=$(find . -not -path './.git/*' -not -path './.git' -not -path './node_modules/*' -not -path './vendor/*' -not -path './.venv/*' -not -path './dist/*' -not -path './build/*' -type f | sort | head -200)

    # Build Claude prompt and get summary
    log_agent "Running Claude to generate repository summary..."
    summary=$(_generate_repo_summary "$tree_output" "$claudemd_contents" "$readme_contents")

    if [ -z "$summary" ]; then
        log_agent "Claude produced no summary, using fallback"
        summary="Repository contains source code."
        if [ ${#detected_stack[@]} -gt 0 ]; then
            summary="Repository using ${detected_stack[*]}."
        fi
    fi

    log_agent "Summary: ${summary}"

    # Determine if setup is needed
    if $has_claudemd && $has_readme && $has_code; then
        needs_setup=false
        log_agent "Repository appears well-configured (has CLAUDE.md, README, and code)"
    else
        needs_setup=true
        local missing=""
        if ! $has_claudemd; then missing="${missing} CLAUDE.md"; fi
        if ! $has_readme; then missing="${missing} README"; fi
        log_agent "Repository may need setup (missing:${missing})"
    fi

    _setup_complete "$summary" "$tech_stack" "$has_code" "$has_claudemd" "$has_readme" "$needs_setup"
}

_detect_tech_stack() {
    local -n _result=$1

    # Go
    [ -f "go.mod" ] && _result+=("Go")

    # Node.js / JavaScript / TypeScript
    if [ -f "package.json" ]; then
        _result+=("Node.js")
        [ -f "tsconfig.json" ] && _result+=("TypeScript")
        [ -f "next.config.js" ] || [ -f "next.config.mjs" ] || [ -f "next.config.ts" ] && _result+=("Next.js")
        [ -f "nuxt.config.js" ] || [ -f "nuxt.config.ts" ] && _result+=("Nuxt")
        [ -f "svelte.config.js" ] && _result+=("Svelte")
        [ -f "angular.json" ] && _result+=("Angular")
        [ -f "vite.config.js" ] || [ -f "vite.config.ts" ] && _result+=("Vite")
    fi

    # Python
    if [ -f "requirements.txt" ] || [ -f "pyproject.toml" ] || [ -f "setup.py" ] || [ -f "Pipfile" ]; then
        _result+=("Python")
        [ -f "manage.py" ] && _result+=("Django")
    fi

    # Rust
    [ -f "Cargo.toml" ] && _result+=("Rust")

    # Java / Kotlin
    [ -f "pom.xml" ] && _result+=("Java" "Maven")
    [ -f "build.gradle" ] || [ -f "build.gradle.kts" ] && _result+=("Gradle")

    # Ruby
    [ -f "Gemfile" ] && _result+=("Ruby")
    [ -f "Gemfile" ] && [ -f "config/routes.rb" ] && _result+=("Rails")

    # PHP
    [ -f "composer.json" ] && _result+=("PHP")

    # .NET
    compgen -G "*.csproj" >/dev/null 2>&1 && _result+=(".NET")
    compgen -G "*.sln" >/dev/null 2>&1 && _result+=(".NET")

    # Swift
    [ -f "Package.swift" ] && _result+=("Swift")

    # Docker
    [ -f "Dockerfile" ] || [ -f "docker-compose.yml" ] || [ -f "docker-compose.yaml" ] && _result+=("Docker")

    # Makefile
    [ -f "Makefile" ] && _result+=("Make")

    # Terraform
    compgen -G "*.tf" >/dev/null 2>&1 && _result+=("Terraform")
}

_array_to_json() {
    local arr=("$@")
    if [ ${#arr[@]} -eq 0 ]; then
        echo "[]"
        return
    fi

    local json="["
    local first=true
    for item in "${arr[@]}"; do
        if [ "$first" = true ]; then first=false; else json+=","; fi
        json+="\"${item}\""
    done
    json+="]"
    echo "$json"
}

_generate_repo_summary() {
    local tree_output="$1"
    local claudemd_contents="$2"
    local readme_contents="$3"

    local prompt="Analyze this repository and provide a concise 2-3 sentence summary of what this project does, the key technologies used, and whether coding standards/patterns are well-established.

Repository structure:
${tree_output}"

    if [ -n "$claudemd_contents" ]; then
        prompt="${prompt}

CLAUDE.md contents:
${claudemd_contents}"
    fi

    if [ -n "$readme_contents" ]; then
        prompt="${prompt}

README contents:
${readme_contents}"
    fi

    prompt="${prompt}

Respond with ONLY the summary text. No markdown formatting, no bullet points, no headers. Just 2-3 plain sentences."

    local model="${CLAUDE_MODEL:-sonnet}"
    local result
    result=$(claude --print --model "${model}" "${prompt}" 2>/dev/null || echo "")

    echo "$result"
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

    log_blank
    log_header "Repo Setup Scan Completed"
}

_setup_complete_fail() {
    local body='{"success": false}'

    curl -s -X POST \
        -H "Content-Type: application/json" \
        -d "$body" \
        "${SETUP_COMPLETE_URL}" > /dev/null 2>&1 || true
}
