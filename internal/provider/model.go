package provider

import "strings"

// resolveModel returns requestModel when non-empty, otherwise defaultModel.
func resolveModel(requestModel, defaultModel string) string {
	if m := strings.TrimSpace(requestModel); m != "" {
		return m
	}
	return defaultModel
}
