#!/bin/bash
# proxy.sh — Start the beta header stripping proxy inside the agent container.
#
# When STRIP_ANTHROPIC_BETA_HEADERS=true, this starts a local Node.js reverse
# proxy that strips anthropic-beta headers from outgoing API requests. The
# ANTHROPIC_BASE_URL is rewritten to point at the local proxy so Claude Code
# CLI routes all API traffic through it.
#
# Depends on: log.sh (sourced by entrypoint.sh)

start_beta_proxy() {
    if [ "${STRIP_ANTHROPIC_BETA_HEADERS}" != "true" ]; then
        return
    fi

    local upstream="${ANTHROPIC_BASE_URL:-https://api.anthropic.com}"
    local port_file
    port_file=$(mktemp)

    log_agent "Starting beta header stripping proxy (upstream: ${upstream})"

    BETA_PROXY_UPSTREAM="$upstream" BETA_PROXY_PORT_FILE="$port_file" \
        node /lib/strip_beta_proxy.js &
    BETA_PROXY_PID=$!

    # Wait for the proxy to write its port to the file
    local retries=0
    while [ $retries -lt 50 ]; do
        if [ -s "$port_file" ]; then
            break
        fi
        sleep 0.1
        retries=$((retries + 1))
    done

    local port
    port=$(cat "$port_file" 2>/dev/null)
    rm -f "$port_file"

    if [ -z "$port" ] || ! echo "$port" | grep -qE '^[0-9]+$'; then
        log_error "Failed to start beta header proxy"
        kill $BETA_PROXY_PID 2>/dev/null || true
        exit 1
    fi

    export ANTHROPIC_BASE_URL="http://127.0.0.1:${port}"
    log_agent "Beta header proxy running on port ${port}"
}
