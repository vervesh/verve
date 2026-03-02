#!/bin/bash
# prereqs.sh — Detect project type and verify required tools are installed

# Depends on: log.sh (sourced by entrypoint.sh)

# Detect whether a project type is present (via files or task description).
# Sets _DETECTED=true/false and _FILE_MATCH=true/false as side effects.
_detect_project() {
    local file_test="$1" desc_pattern="$2"
    local desc_lower
    desc_lower=$(echo "$TASK_DESCRIPTION" | tr '[:upper:]' '[:lower:]')

    _FILE_MATCH=false
    IFS=';' read -ra files <<< "$file_test"
    for f in "${files[@]}"; do
        if [ -f "$f" ]; then
            _FILE_MATCH=true
            break
        fi
    done

    local desc_match=false
    if echo "$desc_lower" | grep -qE "$desc_pattern"; then
        desc_match=true
    fi

    if $_FILE_MATCH || $desc_match; then
        _DETECTED=true
    else
        _DETECTED=false
    fi
}

check_prereqs() {
    log_agent "Checking project prerequisites..."

    _PREREQ_DETECTED=()
    _PREREQ_MISSING=()

    # ── Go ──
    _detect_project "go.mod;go.sum" '\bgolang\b|\bgo (app|api|server|service|module|project|program|binary|cli)\b'
    if $_DETECTED; then
        _PREREQ_DETECTED+=("go")
        if ! command -v go &>/dev/null; then
            local reason="go.mod found but go is not installed"
            if ! $_FILE_MATCH; then
                reason="Task description references Go but go is not installed"
            fi
            _PREREQ_MISSING+=("{\"tool\":\"go\",\"reason\":\"${reason}\",\"install\":\"Install Go or use a Go-based agent image\"}")
        fi
    fi

    # ── Python ──
    _detect_project "requirements.txt;pyproject.toml;setup.py;Pipfile;poetry.lock" '\bpython\b|\bdjango\b|\bflask\b|\bfastapi\b|\bpip\b'
    if $_DETECTED; then
        _PREREQ_DETECTED+=("python")
        if ! command -v python3 &>/dev/null && ! command -v python &>/dev/null; then
            local reason="Python project detected but python3/python not available"
            if ! $_FILE_MATCH; then
                reason="Task description references Python but python3/python not available"
            fi
            _PREREQ_MISSING+=("{\"tool\":\"python\",\"reason\":\"${reason}\",\"install\":\"Install Python or use a Python-based agent image\"}")
        fi
    fi

    # ── Rust ──
    _detect_project "Cargo.toml" '\brust\b|\bcargo\b'
    if $_DETECTED; then
        _PREREQ_DETECTED+=("rust")
        if ! command -v cargo &>/dev/null; then
            local reason="Cargo.toml found but cargo is not installed"
            if ! $_FILE_MATCH; then
                reason="Task description references Rust but cargo is not installed"
            fi
            _PREREQ_MISSING+=("{\"tool\":\"cargo\",\"reason\":\"${reason}\",\"install\":\"Install Rust or use a Rust-based agent image\"}")
        fi
    fi

    # ── Gradle (Java/Kotlin) ──
    _detect_project "build.gradle;build.gradle.kts" '\bgradle\b|\bkotlin\b'
    if $_DETECTED; then
        _PREREQ_DETECTED+=("gradle")
        if ! command -v gradle &>/dev/null && [ ! -f "gradlew" ]; then
            _PREREQ_MISSING+=('{"tool":"gradle","reason":"Gradle build file found but gradle not available and no gradlew wrapper","install":"Install Gradle or include gradlew in the repo"}')
        fi
    fi

    # ── Maven (Java) ──
    _detect_project "pom.xml" '\bmaven\b|\bjava\b|\bspring\b'
    if $_DETECTED; then
        _PREREQ_DETECTED+=("maven")
        if ! command -v mvn &>/dev/null && [ ! -f "mvnw" ] && ! command -v java &>/dev/null; then
            local reason="pom.xml found but mvn/java not available and no mvnw wrapper"
            if ! $_FILE_MATCH; then
                reason="Task description references Java but java/mvn not available"
            fi
            _PREREQ_MISSING+=("{\"tool\":\"java\",\"reason\":\"${reason}\",\"install\":\"Install Java/Maven or use a Java-based agent image\"}")
        fi
    fi

    # ── Ruby ──
    _detect_project "Gemfile" '\bruby\b|\brails\b'
    if $_DETECTED; then
        _PREREQ_DETECTED+=("ruby")
        if ! command -v ruby &>/dev/null; then
            local reason="Gemfile found but ruby is not installed"
            if ! $_FILE_MATCH; then
                reason="Task description references Ruby but ruby is not installed"
            fi
            _PREREQ_MISSING+=("{\"tool\":\"ruby\",\"reason\":\"${reason}\",\"install\":\"Install Ruby or use a Ruby-based agent image\"}")
        fi
    fi

    # ── PHP ──
    _detect_project "composer.json" '\bphp\b|\blaravel\b|\bsymfony\b'
    if $_DETECTED; then
        _PREREQ_DETECTED+=("php")
        if ! command -v php &>/dev/null; then
            local reason="composer.json found but php is not installed"
            if ! $_FILE_MATCH; then
                reason="Task description references PHP but php is not installed"
            fi
            _PREREQ_MISSING+=("{\"tool\":\"php\",\"reason\":\"${reason}\",\"install\":\"Install PHP or use a PHP-based agent image\"}")
        fi
    fi

    # ── .NET ──
    local dotnet_file_match=false
    if compgen -G "*.csproj" >/dev/null 2>&1 || compgen -G "*.fsproj" >/dev/null 2>&1 || compgen -G "*.sln" >/dev/null 2>&1; then
        dotnet_file_match=true
    fi
    local desc_lower
    desc_lower=$(echo "$TASK_DESCRIPTION" | tr '[:upper:]' '[:lower:]')
    if $dotnet_file_match || echo "$desc_lower" | grep -qE '\b\.net\b|\bdotnet\b|\bcsharp\b|\bc#\b'; then
        _PREREQ_DETECTED+=("dotnet")
        if ! command -v dotnet &>/dev/null; then
            _PREREQ_MISSING+=('{"tool":"dotnet","reason":".NET project detected but dotnet CLI not available","install":"Install .NET SDK or use a .NET-based agent image"}')
        fi
    fi

    # ── Swift ──
    _detect_project "Package.swift" '\bswift\b|\bswiftui\b'
    if $_DETECTED; then
        _PREREQ_DETECTED+=("swift")
        if ! command -v swift &>/dev/null; then
            local reason="Package.swift found but swift is not installed"
            if ! $_FILE_MATCH; then
                reason="Task description references Swift but swift is not installed"
            fi
            _PREREQ_MISSING+=("{\"tool\":\"swift\",\"reason\":\"${reason}\",\"install\":\"Install Swift or use a Swift-based agent image\"}")
        fi
    fi

    # ── Report results ──
    if [ ${#_PREREQ_MISSING[@]} -gt 0 ]; then
        _report_missing_prereqs
        exit 1
    fi

    if [ ${#_PREREQ_DETECTED[@]} -gt 0 ]; then
        log_agent "Prerequisite check passed for: ${_PREREQ_DETECTED[*]}"
    else
        log_agent "No specific runtime requirements detected"
    fi
}

_report_missing_prereqs() {
    # Build JSON arrays from _PREREQ_DETECTED and _PREREQ_MISSING globals
    local missing_json="["
    local first=true
    for item in "${_PREREQ_MISSING[@]}"; do
        if [ "$first" = true ]; then first=false; else missing_json+=","; fi
        missing_json+="$item"
    done
    missing_json+="]"

    local detected_json
    detected_json=$(printf '"%s",' "${_PREREQ_DETECTED[@]}")
    detected_json="[${detected_json%,}]"

    log_blank
    log_agent "PREREQUISITE CHECK FAILED"
    log_agent "Detected project types: ${_PREREQ_DETECTED[*]}"
    log_agent "Missing tools:"
    for item in "${_PREREQ_MISSING[@]}"; do
        local tool reason
        tool=$(echo "$item" | jq -r '.tool')
        reason=$(echo "$item" | jq -r '.reason')
        log_agent "  - ${tool}: ${reason}"
    done

    local dockerfile_json
    dockerfile_json=$(_generate_dockerfile_suggestion)

    log_blank
    echo "VERVE_PREREQ_FAILED:{\"detected\":${detected_json},\"missing\":${missing_json},\"dockerfile\":${dockerfile_json}}"
}

# Generate a Dockerfile suggestion for missing prerequisites using Claude.
_generate_dockerfile_suggestion() {
    if [ "$DRY_RUN" = "true" ]; then
        echo "null"
        return
    fi

    log_blank
    log_agent "Analyzing repository and generating suggested Dockerfile..."

    local prompt="Analyze this repository and generate a Dockerfile that extends the Verve agent base image with all the dependencies needed to build and work on this project.

Look at the project files (e.g. go.mod, requirements.txt, Cargo.toml, package.json, Makefile, etc.) to determine the exact language versions, build tools, and system dependencies required.

Rules:
- Start with: FROM ghcr.io/joshjon/verve:base
- The base image is Alpine Linux (node:22-alpine based). Node.js and npm are already available.
- Where official language images exist (e.g. golang, rust, python), prefer COPY --from to copy the toolchain. For example: COPY --from=golang:1.25-alpine /usr/local/go /usr/local/go
- Use 'apk add' for system packages, not apt-get
- Switch to USER root for installs, then back to USER agent at the end
- The non-root user is called 'agent' with home dir /home/agent
- Add a header comment with build and usage instructions
- Keep it minimal — only install what's actually needed
- Match the exact language versions from the project's config files where possible
- Output ONLY the raw Dockerfile content, no markdown fences, no explanation, no surrounding text"

    local model="${CLAUDE_MODEL:-sonnet}"
    local content
    content=$(claude --print --model "${model}" "${prompt}" 2>/dev/null || echo "")

    if [ -n "$content" ]; then
        log_agent "Suggested Dockerfile:"
        echo "$content"
        printf '%s' "$content" | jq -Rs .
    else
        log_agent "Could not generate Dockerfile suggestion"
        echo "null"
    fi
}
