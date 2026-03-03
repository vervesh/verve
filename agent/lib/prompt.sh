#!/bin/bash
# prompt.sh — Build the Claude Code prompt based on task context
# Sets the global PROMPT variable used by claude.sh

build_prompt() {
    local task_title_or_desc="${TASK_TITLE:-${TASK_DESCRIPTION}}"
    local prompt

    if [ "${ATTEMPT:-1}" -gt 1 ]; then
        prompt="You are an autonomous coding agent running non-interactively. You MUST fix the issues described below by making actual code changes. Do not just explore or plan — write and commit the code.

IMPORTANT: Do NOT use EnterPlanMode or ExitPlanMode. There is no human to approve plans. Just implement the changes directly.

This is retry attempt ${ATTEMPT}. The previous attempt created a PR but it needs fixes.
Reason for retry: ${RETRY_REASON}"

        if echo "$RETRY_REASON" | grep -qi "ci_failure"; then
            prompt+="
Please examine the existing code changes on this branch, review the CI failure details below, and fix the issues. Do NOT create a new PR - just fix the code and commit to this branch."
        elif echo "$RETRY_REASON" | grep -qi "merge_conflict"; then
            prompt+="
The branch had merge conflicts with ${DEFAULT_BRANCH}. A rebase was attempted. Please resolve any remaining conflicts, ensure the code works correctly with the latest ${DEFAULT_BRANCH} branch, and commit. Do NOT create a new PR."
        else
            prompt+="
The user has reviewed your previous changes and provided feedback. Please examine the existing code on this branch, address the feedback above, and push the improved changes. Do NOT create a new PR - just fix the code and commit to this branch."
        fi

        if [ -n "$RETRY_CONTEXT" ]; then
            prompt+="

=== CI Failure Output ===
${RETRY_CONTEXT}
=== End CI Output ==="
        fi

        if [ -n "$PREVIOUS_STATUS" ]; then
            prompt+="

=== Previous Iteration Notes ===
${PREVIOUS_STATUS}
=== End Notes ==="
        fi

        # Include git log of previous commits so Claude knows what's been done
        local previous_commits
        previous_commits=$(git log --oneline "${DEFAULT_BRANCH}..HEAD" 2>/dev/null | head -20)
        if [ -n "$previous_commits" ]; then
            prompt+="

=== Previous Commits on This Branch ===
${previous_commits}
=== End Previous Commits ===

The above commits are from a previous attempt and are already applied to the working tree.
Review these changes before continuing — do not redo work that is already complete."
        fi

        prompt+="

Original task: ${task_title_or_desc}"
        if [ -n "${TASK_TITLE}" ] && [ -n "${TASK_DESCRIPTION}" ]; then
            prompt+="
Details: ${TASK_DESCRIPTION}"
        fi
    else
        prompt="You are an autonomous coding agent running non-interactively. You MUST implement the following task by making actual code changes. Do not just explore or plan — write and commit the code.

IMPORTANT: Do NOT use EnterPlanMode or ExitPlanMode. There is no human to approve plans. Just implement the changes directly.

Task: ${task_title_or_desc}"

        if [ -n "${TASK_TITLE}" ] && [ -n "${TASK_DESCRIPTION}" ]; then
            prompt+="
Details: ${TASK_DESCRIPTION}"
        fi
    fi

    if [ -n "$ACCEPTANCE_CRITERIA" ]; then
        prompt+="

ACCEPTANCE CRITERIA (report which are met in your VERVE_STATUS output):
${ACCEPTANCE_CRITERIA}"
    fi

    prompt+='

COMMIT MESSAGE FORMAT: All git commits MUST follow the Conventional Commits specification (https://www.conventionalcommits.org/en/v1.0.0/).
Format: type(scope)?: description
Allowed types: feat, fix, refactor, docs, test, chore, ci
Examples: "feat: add user authentication", "fix(api): handle null response", "chore: update dependencies"
A git hook will reject commits that do not follow this format.

As you work, periodically save your progress by running: git add -A && git commit -m "wip: <brief description>" && git push -u origin HEAD
This ensures your work is pushed to the remote and can be recovered if the session is interrupted.

IMPORTANT: Before you finish, output a status line in this exact format on its own line:
VERVE_STATUS:{"files_modified":[],"tests_status":"pass|fail|skip","confidence":"high|medium|low","blockers":[],"criteria_met":[],"notes":"Any context for future retry attempts"}'

    PROMPT="$prompt"
}
