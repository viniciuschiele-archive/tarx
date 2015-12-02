package archive

import (
	"os"
	"strings"
)

func prepareFilters(filters []string) [][]string {
	if filters == nil {
		filters = []string{}
	}

	preparedFilters := make([][]string, len(filters))

	for i, filter := range filters {
		preparedFilters[i] = strings.Split(filter, string(os.PathSeparator))
	}

	return preparedFilters
}

func optimizedMatches(path string, filters [][]string) bool {
	if len(filters) == 0 {
		return true
	}

	pathDirs := strings.Split(path, string(os.PathSeparator))

	for _, filter := range filters {
		i := 0
		count := min(len(pathDirs), len(filter))

		for {
			if i == count {
				return true
			}

			if pathDirs[i] != filter[i] {
				break
			}

			i++
		}
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
