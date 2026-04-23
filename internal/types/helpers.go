package types

import (
	"sort"
	"strconv"
	"strings"

	"defgraph/internal/collections"
)

func SortNumericStrings(values []string) []string {
	sort.Slice(values, func(left int, right int) bool {
		leftNumber, leftErr := strconv.Atoi(values[left])
		rightNumber, rightErr := strconv.Atoi(values[right])
		if leftErr == nil && rightErr == nil {
			return leftNumber < rightNumber
		}

		if leftErr == nil {
			return true
		}

		if rightErr == nil {
			return false
		}

		return values[left] < values[right]
	})

	return values
}

func CanonicalizeStrings(values []string) []string {
	set := collections.NewOrderedSet[string]()
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}

		set.Add(trimmed)
	}

	out := set.Values()

	sort.Strings(out)
	return out
}
