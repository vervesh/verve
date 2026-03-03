#!/bin/bash
# github.sh — GitHub API helpers (PR creation)

# Depends on: log.sh (sourced by entrypoint.sh)

# _curl_opts returns extra curl flags when TLS verification is disabled.
_curl_opts() {
    if [ "${GITHUB_INSECURE_SKIP_VERIFY}" = "true" ]; then
        echo "--insecure"
    fi
}

# Check whether an open pull request already exists for a given head branch.
# Returns 0 (true) if a PR exists, 1 (false) otherwise.
# Sets PR_URL to the HTML URL of the existing PR when found.
# Sets PR_NUMBER to the number of the existing PR when found.
# Usage: pr_exists_for_branch <head_branch>
pr_exists_for_branch() {
    local head="$1"

    local response
    response=$(curl -s -w "\n%{http_code}" $(_curl_opts) \
        -H "Authorization: token ${GITHUB_TOKEN}" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/${GITHUB_REPO}/pulls?head=${GITHUB_REPO%%/*}:${head}&state=open")

    local http_code response_body
    http_code=$(echo "$response" | tail -1)
    response_body=$(echo "$response" | sed '$d')

    if [ "$http_code" = "200" ]; then
        local count
        count=$(echo "$response_body" | jq 'length' 2>/dev/null || echo "0")
        if [ "$count" -gt 0 ]; then
            PR_URL=$(echo "$response_body" | jq -r '.[0].html_url // empty' 2>/dev/null || echo "")
            PR_NUMBER=$(echo "$response_body" | jq -r '.[0].number // empty' 2>/dev/null || echo "")
            return 0
        fi
    fi
    return 1
}

# Create a pull request via the GitHub API.
# Usage: create_pr <title> <body> <head_branch> <base_branch>
create_pr() {
    local title="$1" body="$2" head="$3" base="$4"

    local json_title json_body
    json_title=$(printf '%s' "$title" | jq -Rs .)
    json_body=$(printf '%s' "$body" | jq -Rs .)

    local draft_field=""
    if [ "${DRAFT_PR}" = "true" ]; then
        draft_field=",\"draft\":true"
    fi

    local response
    response=$(curl -s -w "\n%{http_code}" $(_curl_opts) -X POST \
        -H "Authorization: token ${GITHUB_TOKEN}" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/${GITHUB_REPO}/pulls" \
        -d "{\"title\":${json_title},\"body\":${json_body},\"head\":\"${head}\",\"base\":\"${base}\"${draft_field}}")

    local http_code response_body
    http_code=$(echo "$response" | tail -1)
    response_body=$(echo "$response" | sed '$d')

    if [ "$http_code" = "201" ]; then
        local pr_url pr_number
        pr_url=$(echo "$response_body" | jq -r '.html_url // empty')
        pr_number=$(echo "$response_body" | jq -r '.number // empty')
        if [ -n "$pr_url" ] && [ -n "$pr_number" ]; then
            log_agent "Pull request created: ${pr_url}"
            echo "VERVE_PR_CREATED:{\"url\":\"${pr_url}\",\"number\":${pr_number}}"
        else
            log_agent "Pull request created but could not parse response"
        fi
        return 0
    else
        local error_msg errors_detail
        error_msg=$(echo "$response_body" | jq -r '.message // empty' 2>/dev/null || echo "unknown error")
        errors_detail=$(echo "$response_body" | jq -r '.errors[]?.message // empty' 2>/dev/null || echo "")
        log_agent "Failed to create pull request (HTTP ${http_code}): ${error_msg}"
        if [ -n "$errors_detail" ]; then
            log_agent "Details: ${errors_detail}"
        fi
        return 1
    fi
}

# Update an existing pull request's title and body via the GitHub API.
# Usage: update_pr <pr_number> <title> <body>
update_pr() {
    local pr_number="$1" title="$2" body="$3"

    local json_title json_body
    json_title=$(printf '%s' "$title" | jq -Rs .)
    json_body=$(printf '%s' "$body" | jq -Rs .)

    local response
    response=$(curl -s -w "\n%{http_code}" $(_curl_opts) -X PATCH \
        -H "Authorization: token ${GITHUB_TOKEN}" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/${GITHUB_REPO}/pulls/${pr_number}" \
        -d "{\"title\":${json_title},\"body\":${json_body}}")

    local http_code response_body
    http_code=$(echo "$response" | tail -1)
    response_body=$(echo "$response" | sed '$d')

    if [ "$http_code" = "200" ]; then
        local pr_url
        pr_url=$(echo "$response_body" | jq -r '.html_url // empty')
        log_agent "Pull request #${pr_number} updated: ${pr_url}"
        echo "VERVE_PR_UPDATED:{\"url\":\"${pr_url}\",\"number\":${pr_number}}"
        return 0
    else
        local error_msg
        error_msg=$(echo "$response_body" | jq -r '.message // empty' 2>/dev/null || echo "unknown error")
        log_agent "Failed to update pull request #${pr_number} (HTTP ${http_code}): ${error_msg}"
        return 1
    fi
}

# Generate an updated PR title and description using Claude, then update the existing PR.
# Compares the current diff against the original task to determine if the PR description
# needs updating after changes made in a retry attempt.
# Usage: generate_and_update_pr <pr_number> <pr_url> <branch> <default_branch>
generate_and_update_pr() {
    local pr_number="$1" pr_url="$2" branch="$3" default_branch="$4"

    log_agent "Checking if PR #${pr_number} description needs updating..."

    # Get the current PR title and body
    local pr_response
    pr_response=$(curl -s -w "\n%{http_code}" $(_curl_opts) \
        -H "Authorization: token ${GITHUB_TOKEN}" \
        -H "Accept: application/vnd.github.v3+json" \
        "https://api.github.com/repos/${GITHUB_REPO}/pulls/${pr_number}")

    local http_code response_body
    http_code=$(echo "$pr_response" | tail -1)
    response_body=$(echo "$pr_response" | sed '$d')

    local current_title="" current_body=""
    if [ "$http_code" = "200" ]; then
        current_title=$(echo "$response_body" | jq -r '.title // empty' 2>/dev/null || echo "")
        current_body=$(echo "$response_body" | jq -r '.body // empty' 2>/dev/null || echo "")
    else
        log_agent "Could not fetch current PR details (HTTP ${http_code}), skipping update"
        return 0
    fi

    local diff_summary
    if git rev-parse "origin/${default_branch}" >/dev/null 2>&1; then
        diff_summary=$(git diff "origin/${default_branch}...HEAD" --stat 2>/dev/null | tail -20 || echo "Changes made by Verve Agent")
    else
        diff_summary=$(git log --oneline --stat HEAD 2>/dev/null | tail -30 || echo "Changes made by Verve Agent (new repository)")
    fi

    local update_prompt="You are reviewing a pull request that was updated after a retry attempt. The agent made additional changes based on feedback or failure fixes.

Task Title: ${TASK_TITLE:-${TASK_DESCRIPTION}}
Task Description: ${TASK_DESCRIPTION}

Retry Reason: ${RETRY_REASON:-unknown}

Current PR Title: ${current_title}

Current PR Description:
${current_body}

Updated files changed (full diff from base):
${diff_summary}

Determine if the current PR title and description still accurately reflect ALL the changes now in this PR. If the changes made during the retry are significant enough to warrant updating the PR description, provide an updated title and description. If the current description is still accurate, respond with no_update.

Respond with ONLY valid JSON in this exact format (no markdown, no code blocks, no extra text):
{\"update_needed\": true, \"title\": \"Updated short title (max 72 chars)\", \"description\": \"## Summary\\n\\nUpdated description of all changes.\\n\\n## Changes\\n\\n- Bullet points of what was done\"}

Or if no update is needed:
{\"update_needed\": false}"

    log_agent "Evaluating PR description with Claude..."
    local model="${CLAUDE_MODEL:-sonnet}"
    local update_raw
    update_raw=$(claude --print --model "${model}" "${update_prompt}" 2>/dev/null || echo "")

    local update_json=""
    if [ -n "${update_raw}" ]; then
        # Try to extract JSON from ```json ... ``` or ``` ... ``` blocks
        update_json=$(echo "${update_raw}" | sed -n '/^```/,/^```$/p' | sed '1d;$d' | tr -d '\n' || echo "")
        # If that didn't work, try to find raw JSON object
        if [ -z "${update_json}" ] || ! echo "${update_json}" | jq -e . >/dev/null 2>&1; then
            update_json=$(echo "${update_raw}" | grep -o '{[^}]*}' | head -1 || echo "")
        fi
        # Last resort: use raw output if it's valid JSON
        if [ -z "${update_json}" ] || ! echo "${update_json}" | jq -e . >/dev/null 2>&1; then
            if echo "${update_raw}" | jq -e . >/dev/null 2>&1; then
                update_json="${update_raw}"
            fi
        fi
    fi

    if [ -z "${update_json}" ] || ! echo "${update_json}" | jq -e . >/dev/null 2>&1; then
        log_agent "Could not parse Claude response for PR update evaluation, skipping"
        return 0
    fi

    local update_needed
    update_needed=$(echo "${update_json}" | jq -r '.update_needed // false' 2>/dev/null || echo "false")

    if [ "${update_needed}" != "true" ]; then
        log_agent "PR description is still accurate, no update needed"
        return 0
    fi

    local new_title new_body
    new_title=$(echo "${update_json}" | jq -r '.title // empty' 2>/dev/null || echo "")
    new_body=$(echo "${update_json}" | jq -r '.description // empty' 2>/dev/null || echo "")

    if [ -z "${new_title}" ] && [ -z "${new_body}" ]; then
        log_agent "Claude indicated update needed but provided no content, skipping"
        return 0
    fi

    # Use current values as fallback
    if [ -z "${new_title}" ]; then
        new_title="${current_title}"
    fi
    if [ -z "${new_body}" ]; then
        new_body="${current_body}"
    fi

    if ! update_pr "${pr_number}" "${new_title}" "${new_body}"; then
        log_agent "PR update failed, but continuing (non-fatal)"
    fi
    return 0
}

# Generate PR title and description using Claude, then create the PR.
# Usage: generate_and_create_pr <branch> <default_branch>
generate_and_create_pr() {
    local branch="$1" default_branch="$2"

    log_agent "Creating pull request..."

    local diff_summary
    if git rev-parse "origin/${default_branch}" >/dev/null 2>&1; then
        diff_summary=$(git diff "origin/${default_branch}...HEAD" --stat 2>/dev/null | tail -20 || echo "Changes made by Verve Agent")
    else
        # Empty repo: no base branch exists yet — list all files as the diff summary
        diff_summary=$(git log --oneline --stat HEAD 2>/dev/null | tail -30 || echo "Changes made by Verve Agent (new repository)")
    fi

    local pr_prompt="Generate a pull request title and description for the following task and changes.

Task Title: ${TASK_TITLE:-${TASK_DESCRIPTION}}
Task Description: ${TASK_DESCRIPTION}

Files changed:
${diff_summary}

Respond with ONLY valid JSON in this exact format (no markdown, no code blocks, no extra text):
{\"title\": \"Short descriptive title (max 72 chars)\", \"description\": \"## Summary\\n\\nBrief description of changes.\\n\\n## Changes\\n\\n- Bullet points of what was done\"}"

    log_agent "Generating PR description with Claude..."
    local model="${CLAUDE_MODEL:-sonnet}"
    local pr_raw
    pr_raw=$(claude --print --model "${model}" "${pr_prompt}" 2>/dev/null || echo "")

    local pr_json=""
    if [ -n "${pr_raw}" ]; then
        # Try to extract JSON from ```json ... ``` or ``` ... ``` blocks
        pr_json=$(echo "${pr_raw}" | sed -n '/^```/,/^```$/p' | sed '1d;$d' | tr -d '\n' || echo "")
        # If that didn't work, try to find raw JSON object
        if [ -z "${pr_json}" ] || ! echo "${pr_json}" | jq -e . >/dev/null 2>&1; then
            pr_json=$(echo "${pr_raw}" | grep -o '{[^}]*}' | head -1 || echo "")
        fi
        # Last resort: use raw output if it's valid JSON
        if [ -z "${pr_json}" ] || ! echo "${pr_json}" | jq -e . >/dev/null 2>&1; then
            if echo "${pr_raw}" | jq -e . >/dev/null 2>&1; then
                pr_json="${pr_raw}"
            fi
        fi
    fi

    local pr_title="" pr_body=""
    if [ -n "${pr_json}" ] && echo "${pr_json}" | jq -e . >/dev/null 2>&1; then
        pr_title=$(echo "${pr_json}" | jq -r '.title // empty' 2>/dev/null || echo "")
        pr_body=$(echo "${pr_json}" | jq -r '.description // empty' 2>/dev/null || echo "")
    fi

    # Fallbacks
    if [ -z "${pr_title}" ]; then
        pr_title="${TASK_TITLE:-${TASK_DESCRIPTION}}"
    fi
    if [ -z "${pr_body}" ]; then
        pr_body="## Summary

Automated implementation of: ${TASK_DESCRIPTION}

## Changes

${diff_summary}"
    fi

    if ! create_pr "${pr_title}" "${pr_body}" "${branch}" "${default_branch}"; then
        log_agent "PR creation failed"
        exit 1
    fi
}
