package filetags

import (
	"sort"
	"strings"
)

const All = "*"
const AllAlias = "all"

// Normalize turns comma/space separated user input into stable lowercase tags.
// Empty input means "all platforms".
func Normalize(input []string) []string {
	seen := map[string]bool{}
	for _, raw := range input {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
			return r == ',' || r == ';' || r == '\n' || r == '\t' || r == ' '
		}) {
			tag := strings.ToLower(strings.TrimSpace(part))
			if tag == "" {
				continue
			}
			if tag == AllAlias {
				tag = All
			}
			seen[tag] = true
		}
	}
	if len(seen) == 0 {
		return []string{All}
	}
	if seen[All] {
		return []string{All}
	}
	out := make([]string, 0, len(seen))
	for tag := range seen {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

func Join(tags []string) string {
	return strings.Join(Normalize(tags), ",")
}

func Split(s string) []string {
	return Normalize([]string{s})
}

func Target(osName, arch string, extra []string) []string {
	tags := append([]string{}, extra...)
	osName = strings.ToLower(strings.TrimSpace(osName))
	arch = strings.ToLower(strings.TrimSpace(arch))
	if osName != "" {
		tags = append(tags, osName)
	}
	if arch != "" {
		tags = append(tags, arch)
	}
	if osName != "" && arch != "" {
		tags = append(tags, osName+"-"+arch)
	}
	return Normalize(tags)
}

func Matches(fileTags, targetTags []string) bool {
	fileTags = Normalize(fileTags)
	for _, tag := range fileTags {
		if tag == All {
			return true
		}
		for _, target := range targetTags {
			if tag == target {
				return true
			}
		}
	}
	return false
}
