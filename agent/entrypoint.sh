#!/bin/bash
set -e

# ── Load libraries ──────────────────────────────────────────────────
LIB_DIR="$(dirname "$0")/lib"
source "${LIB_DIR}/log.sh"
source "${LIB_DIR}/validate.sh"
source "${LIB_DIR}/git.sh"
source "${LIB_DIR}/github.sh"
source "${LIB_DIR}/prompt.sh"
source "${LIB_DIR}/claude.sh"
source "${LIB_DIR}/dryrun.sh"
source "${LIB_DIR}/proxy.sh"

# ── Start beta header proxy (if enabled) ───────────────────────────
start_beta_proxy

# ── Branch on work type ─────────────────────────────────────────────
if [ "${WORK_TYPE}" = "epic" ]; then
    source "${LIB_DIR}/epic_plan.sh"
    run_epic_planning
    exit $?
fi

if [ "${WORK_TYPE}" = "setup" ]; then
    source "${LIB_DIR}/repo_setup.sh"
    run_repo_setup
    exit $?
fi

if [ "${WORK_TYPE}" = "setup-review" ]; then
    source "${LIB_DIR}/repo_setup.sh"
    run_repo_setup_review
    exit $?
fi

if [ "${WORK_TYPE}" = "conversation" ]; then
    source "${LIB_DIR}/conversation.sh"
    run_conversation
    exit $?
fi

# ── Task execution (default) ────────────────────────────────────────

# ── Failure trap ─────────────────────────────────────────────────────
cleanup_on_failure() {
    local exit_code=$?
    if [ "$exit_code" -ne 0 ] && [ -n "${BRANCH:-}" ]; then
        log_agent "Agent exiting with error — pushing work-in-progress to branch"
        push_wip
    fi
}
trap cleanup_on_failure EXIT

# ── Banner ──────────────────────────────────────────────────────────
log_header "Verve Agent Starting"
[ -n "${TASK_NUMBER}" ] && echo "Task: #${TASK_NUMBER}"
echo "Task ID: ${TASK_ID}"
echo "Repository: ${GITHUB_REPO}"
[ -n "${TASK_TITLE}" ] && echo "Title: ${TASK_TITLE}"
echo "Description: ${TASK_DESCRIPTION}"
if [ "${ATTEMPT:-1}" -gt 1 ]; then
    echo "Attempt: ${ATTEMPT} (retry)"
    echo "Retry Reason: ${RETRY_REASON}"
fi
log_blank

# ── Setup ───────────────────────────────────────────────────────────
validate_env
configure_git
clone_repo
detect_default_branch
setup_branch

# ── Initialize tome (session memory) ──────────────────────────────
if [ "$TOME_ENABLED" != "false" ] && command -v tome &>/dev/null; then
    if tome init --no-hooks >/dev/null 2>&1; then
        log_agent "Tome session memory initialized"
        tome sync --pull >/dev/null 2>&1 || true
    else
        log_agent "Tome initialization skipped (no writable data directory)"
    fi
fi

# ── Dry run shortcut ────────────────────────────────────────────────
if [ "$DRY_RUN" = "true" ]; then
    run_dry_run
    exit 0
fi

# ── Run Claude Code ─────────────────────────────────────────────────
if [ "${ATTEMPT:-1}" -gt 1 ]; then
    log_agent "Building retry-aware prompt..."
fi
build_prompt
run_claude "$PROMPT"

# ── Commit, push, and create PR ────────────────────────────────────
commit_and_push

if [ "$SKIP_PR" = "true" ]; then
    log_agent "Skip PR mode: branch pushed, skipping PR creation"
    echo "VERVE_BRANCH_PUSHED:{\"branch\":\"${BRANCH}\"}"
elif [ "${ATTEMPT:-1}" -le 1 ] || [ "${BRANCH_EXISTS_ON_REMOTE}" != "true" ]; then
    # For empty repos, ensure the default branch exists so PRs have a base
    ensure_base_branch
    generate_and_create_pr "${BRANCH}" "${DEFAULT_BRANCH}"
elif pr_exists_for_branch "${BRANCH}"; then
    log_agent "Retry: pushed fixes to existing PR (${PR_URL})"
    generate_and_update_pr "${PR_NUMBER}" "${PR_URL}" "${BRANCH}" "${DEFAULT_BRANCH}"
else
    log_agent "Retry: no existing PR found for branch, creating one..."
    generate_and_create_pr "${BRANCH}" "${DEFAULT_BRANCH}"
fi

# ── Sync tome sessions to remote ─────────────────────────────────
if [ "$TOME_ENABLED" != "false" ] && command -v tome &>/dev/null; then
    _tome_out=$(tome sync --push 2>/dev/null) || true
    if [ -n "$_tome_out" ]; then
        log_agent "Tome: $_tome_out"
    fi
fi

log_blank
log_header "Task Completed Successfully"
echo "Branch: ${BRANCH}"
