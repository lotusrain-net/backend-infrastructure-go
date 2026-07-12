package iam

import "strings"

func HasPermission(granted []string, required string) bool {
	if required == "" {
		return false
	}
	module, _, hasAction := strings.Cut(required, ":")
	for _, permission := range granted {
		if permission == "*" || permission == required {
			return true
		}
		if hasAction && permission == module+":*" {
			return true
		}
	}
	return false
}
