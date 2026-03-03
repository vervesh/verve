package worker

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parsePRMarker extracts PR URL and number from a VERVE_PR_CREATED marker line.
func parsePRMarker(line string) (string, int) { //nolint:unused // used by tests
	if !strings.HasPrefix(line, "VERVE_PR_CREATED:") {
		return "", 0
	}
	jsonStr := strings.TrimPrefix(line, "VERVE_PR_CREATED:")
	var prInfo struct {
		URL    string `json:"url"`
		Number int    `json:"number"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &prInfo); err != nil {
		return "", 0
	}
	return prInfo.URL, prInfo.Number
}

// parseBranchMarker extracts the branch name from a VERVE_BRANCH_PUSHED marker line.
func parseBranchMarker(line string) string { //nolint:unused // used by tests
	if !strings.HasPrefix(line, "VERVE_BRANCH_PUSHED:") {
		return ""
	}
	jsonStr := strings.TrimPrefix(line, "VERVE_BRANCH_PUSHED:")
	var branchInfo struct {
		Branch string `json:"branch"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &branchInfo); err != nil {
		return ""
	}
	return branchInfo.Branch
}

// parseStatusMarker extracts the status JSON from a VERVE_STATUS marker line.
func parseStatusMarker(line string) string { //nolint:unused // used by tests
	if !strings.HasPrefix(line, "VERVE_STATUS:") {
		return ""
	}
	return strings.TrimPrefix(line, "VERVE_STATUS:")
}

// parseCostMarker extracts the cost value from a VERVE_COST marker line.
func parseCostMarker(line string) float64 { //nolint:unused // used by tests
	if !strings.HasPrefix(line, "VERVE_COST:") {
		return 0
	}
	costStr := strings.TrimPrefix(line, "VERVE_COST:")
	var cost float64
	if _, err := fmt.Sscanf(costStr, "%f", &cost); err != nil {
		return 0
	}
	return cost
}
