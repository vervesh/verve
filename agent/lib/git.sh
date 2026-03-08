#!/bin/bash
# git.sh — Git configuration, cloning, and branch management

# Depends on: log.sh (sourced by entrypoint.sh)

configure_git() {
    log_agent "Configuring git..."
    git config --global credential.helper store
    echo "https://${GITHUB_TOKEN}@github.com" > /home/agent/.git-credentials
    git config --global user.name "Verve Agent"
    git config --global user.email ""

    if [ "${GITHUB_INSECURE_SKIP_VERIFY}" = "true" ]; then
        log_agent "TLS certificate verification disabled for git (GITHUB_INSECURE_SKIP_VERIFY=true)"
        git config --global http.sslVerify false
    fi

    # Install a global commit-msg hook that enforces Conventional Commits
    # (https://www.conventionalcommits.org/en/v1.0.0/) on every repository
    # the agent works with.
    _install_conventional_commit_hook
}

_install_conventional_commit_hook() {
    local hooks_dir="/home/agent/.config/git/hooks"
    mkdir -p "$hooks_dir"

    cat > "${hooks_dir}/commit-msg" << 'HOOK'
#!/bin/sh
# Verve agent global commit-msg hook — enforces Conventional Commits.
# https://www.conventionalcommits.org/en/v1.0.0/

commit_msg=$(head -1 "$1")

# Allow merge and revert commits
case "$commit_msg" in
    Merge\ *|Revert\ *) exit 0 ;;
esac

# Conventional Commits: type(optional scope)!: description
# Allowed types: feat, fix, refactor, docs, test, chore, ci, wip
if ! echo "$commit_msg" | grep -qE '^(feat|fix|refactor|docs|test|chore|ci|wip)(\(.+\))?!?: .+'; then
    echo >&2 "ERROR: Commit message does not follow Conventional Commits."
    echo >&2 ""
    echo >&2 "  Format: type(scope)?: description"
    echo >&2 "  Allowed types: feat, fix, refactor, docs, test, chore, ci"
    echo >&2 ""
    echo >&2 "  Examples:"
    echo >&2 "    feat: add epic planning support"
    echo >&2 "    fix: prevent stale tasks from blocking queue"
    echo >&2 "    feat(worker): add retry logic"
    echo >&2 ""
    echo >&2 "  Your message: $commit_msg"
    exit 1
fi
HOOK

    chmod +x "${hooks_dir}/commit-msg"
    git config --global core.hooksPath "$hooks_dir"
    log_agent "Conventional commit hook installed globally"
}

clone_repo() {
    log_agent "Cloning repository: ${GITHUB_REPO}..."
    git clone "https://${GITHUB_TOKEN}@github.com/${GITHUB_REPO}.git" /workspace/repo
    cd /workspace/repo || exit 1
}

detect_default_branch() {
    DEFAULT_BRANCH=$(git symbolic-ref refs/remotes/origin/HEAD 2>/dev/null | sed 's@^refs/remotes/origin/@@')

    # Empty repos have no remote HEAD, so the above produces an empty string.
    # Fall back to "main" when the result is empty.
    if [ -z "$DEFAULT_BRANCH" ]; then
        DEFAULT_BRANCH="main"
    fi

    log_agent "Default branch: ${DEFAULT_BRANCH}"
}

setup_branch() {
    BRANCH="verve/task-${TASK_NUMBER:-${TASK_ID}}"
    # Track whether the branch already existed on the remote. Used later to
    # decide between force-push vs first push, and whether a PR still needs
    # to be created.
    BRANCH_EXISTS_ON_REMOTE=false

    if [ "${ATTEMPT:-1}" -gt 1 ]; then
        log_agent "Retry attempt ${ATTEMPT}: checking out existing branch ${BRANCH}"
        if git fetch origin "${BRANCH}" 2>/dev/null; then
            BRANCH_EXISTS_ON_REMOTE=true
            git checkout "${BRANCH}"

            if echo "$RETRY_REASON" | grep -qi "merge conflict"; then
                log_agent "Rebasing on ${DEFAULT_BRANCH} to resolve merge conflicts..."
                git fetch origin "${DEFAULT_BRANCH}"
                # Don't fail if rebase has conflicts — Claude will resolve them
                git rebase "origin/${DEFAULT_BRANCH}" || true
            fi
        else
            log_agent "Branch ${BRANCH} not found on remote (previous attempt may have failed before pushing)"
            log_agent "Creating branch: ${BRANCH}"
            git checkout -b "${BRANCH}"
        fi
    else
        log_agent "Creating branch: ${BRANCH}"
        git checkout -b "${BRANCH}"
    fi
}

push_wip() {
    set +e  # Don't exit on errors in cleanup
    cd /workspace/repo 2>/dev/null || return 0

    git add -A 2>/dev/null
    if ! git diff --cached --quiet 2>/dev/null; then
        git commit -m "wip: agent interrupted" 2>/dev/null || return 0
    fi

    # Push if there are unpushed commits
    if git rev-parse "origin/${BRANCH}" >/dev/null 2>&1; then
        if git log "origin/${BRANCH}..HEAD" --oneline 2>/dev/null | grep -q .; then
            git push --force-with-lease origin "${BRANCH}" 2>/dev/null || true
        fi
    else
        git push -u origin "${BRANCH}" 2>/dev/null || true
    fi
}

# For empty repos, ensure the default branch exists on the remote so that
# a PR can be created against it. Creates an empty initial commit on the
# default branch and pushes it.
# Returns 0 if the base branch was created or already existed, 1 on failure.
ensure_base_branch() {
    # Safety check: DEFAULT_BRANCH must be set
    if [ -z "${DEFAULT_BRANCH}" ]; then
        log_agent "Warning: DEFAULT_BRANCH is empty, defaulting to 'main'"
        DEFAULT_BRANCH="main"
    fi

    if git rev-parse "origin/${DEFAULT_BRANCH}" >/dev/null 2>&1; then
        return 0  # Already exists
    fi

    log_agent "Empty repository detected — initializing ${DEFAULT_BRANCH} branch for PR base"

    # Create an orphan branch with an empty commit, push it, then switch back.
    # We need to preserve and restore the working tree so agent changes aren't
    # lost during the branch switch.
    local current_branch
    current_branch=$(git rev-parse --abbrev-ref HEAD)

    # Stash any uncommitted changes (including untracked files) so that the
    # orphan branch starts clean and we can restore them afterwards.
    local stashed=false
    if ! git diff --quiet 2>/dev/null || ! git diff --cached --quiet 2>/dev/null || [ -n "$(git ls-files --others --exclude-standard 2>/dev/null)" ]; then
        git stash push --include-untracked -m "verve: temp stash for base branch init" 2>/dev/null && stashed=true
    fi

    git checkout --orphan "${DEFAULT_BRANCH}"
    git rm -rf . >/dev/null 2>&1 || true
    git commit --allow-empty --no-verify -m "Initial commit"
    git push -u origin "${DEFAULT_BRANCH}" 2>&1
    git checkout "${current_branch}"

    # Restore stashed changes
    if [ "$stashed" = true ]; then
        git stash pop 2>/dev/null || true
    fi

    # Fetch so origin/DEFAULT_BRANCH is available locally
    git fetch origin "${DEFAULT_BRANCH}" 2>/dev/null || true
}

commit_and_push() {
    log_agent "Checking for changes..."
    git add -A

    if ! git diff --cached --quiet; then
        log_agent "Committing changes..."
        local commit_title="${TASK_TITLE:-${TASK_DESCRIPTION}}"
        # Ensure the fallback commit message follows Conventional Commits.
        # The agent is instructed to commit as it works, so this is only
        # reached when there are uncommitted changes left over. Prefix with
        # "feat:" unless the message already has a conventional type.
        if ! echo "$commit_title" | grep -qE '^(feat|fix|refactor|docs|test|chore|ci|wip)(\(.+\))?!?: '; then
            commit_title="feat: ${commit_title}"
        fi
        git commit -m "${commit_title}"
    else
        log_agent "No new changes to commit"
    fi

    # Fetch latest default branch so the comparison is accurate (on retries
    # only the task branch may have been fetched, leaving origin/DEFAULT_BRANCH stale).
    git fetch origin "${DEFAULT_BRANCH}" 2>/dev/null || true

    # Check for any commits ahead of the default branch.
    # For empty repos, origin/DEFAULT_BRANCH won't exist — in that case any
    # commits on HEAD count as new changes.
    local changes
    if git rev-parse "origin/${DEFAULT_BRANCH}" >/dev/null 2>&1; then
        changes=$(git log "origin/${DEFAULT_BRANCH}..HEAD" --oneline 2>/dev/null || true)
    else
        # Empty repo: no remote default branch exists yet. All local commits
        # are new changes.
        changes=$(git log HEAD --oneline 2>/dev/null || true)
    fi
    if [ -z "$changes" ]; then
        log_agent "No changes were made — task appears to already meet the required criteria"
        echo 'VERVE_NO_CHANGES:true'
        echo 'VERVE_STATUS:{"files_modified":[],"tests_status":"skip","confidence":"high","blockers":[],"criteria_met":["already_satisfied"],"notes":"No changes needed — the codebase already meets the required criteria"}'
        exit 0
    fi

    if [ "${BRANCH_EXISTS_ON_REMOTE}" = "true" ]; then
        log_agent "Pushing fixes to existing branch..."
        git push --force-with-lease origin "${BRANCH}"
    else
        log_agent "Pushing branch to origin..."
        if ! git push -u origin "${BRANCH}" 2>&1; then
            log_agent "Push was rejected (branch may already exist on remote), retrying with --force-with-lease..."
            git push --force-with-lease -u origin "${BRANCH}"
        fi
    fi
    log_agent "Branch pushed successfully: ${BRANCH}"
}
