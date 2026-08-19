package app

import (
	"path/filepath"
	"strings"

	contracts "wails-app/internal/contracts"
)

func filterEntries(entries []contracts.FileInfo, filter contracts.FilterQuery) []contracts.FileInfo {
	if filter.Text == "" && len(filter.Criteria) == 0 {
		return entries
	}

	result := make([]contracts.FileInfo, 0, len(entries))
	for _, entry := range entries {
		if matchesFilter(entry, filter) {
			result = append(result, entry)
		}
	}
	return result
}

func matchesFilter(entry contracts.FileInfo, filter contracts.FilterQuery) bool {
	if filter.Text != "" {
		name := entry.Name
		text := filter.Text
		if !filter.CaseSensitive {
			name = strings.ToLower(name)
			text = strings.ToLower(text)
		}
		if !strings.Contains(name, text) {
			return false
		}
	}

	for _, criteria := range filter.Criteria {
		if !matchesCriteria(entry, criteria, filter.CaseSensitive) {
			return false
		}
	}

	return true
}

func matchesCriteria(entry contracts.FileInfo, criteria contracts.FilterCriteria, caseSensitive bool) bool {
	switch criteria.Type {
	case contracts.FilterName:
		return compareStrings(entry.Name, criteria.Value, criteria.Operator, caseSensitive)
	case contracts.FilterExtension:
		ext := strings.TrimPrefix(filepath.Ext(entry.Name), ".")
		return compareStrings(ext, criteria.Value, criteria.Operator, caseSensitive)
	case contracts.FilterGlob:
		matched, _ := filepath.Match(criteria.Value, entry.Name)
		return matched
	case contracts.FilterType_:
		return strings.EqualFold(string(entry.Type), criteria.Value)
	}
	return true
}

func compareStrings(a, b, operator string, caseSensitive bool) bool {
	if !caseSensitive {
		a = strings.ToLower(a)
		b = strings.ToLower(b)
	}
	switch operator {
	case "equals":
		return a == b
	case "startsWith":
		return strings.HasPrefix(a, b)
	case "endsWith":
		return strings.HasSuffix(a, b)
	case "contains", "":
		return strings.Contains(a, b)
	}
	return strings.Contains(a, b)
}