package filesys

import "sort"

func uniqueStrings(input []string) []string {
	sort.Strings(input)
	unique := input[:0]
	for i, s := range input {
		if i == 0 || s != input[i-1] {
			unique = append(unique, s)
		}
	}
	return unique
}
