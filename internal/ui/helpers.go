package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/damien/circleci-watch/internal/api"
)

func calcJobDuration(j api.Job) string {
	if j.StartedAt == nil {
		return ""
	}
	end := time.Now()
	if j.StoppedAt != nil && !j.StoppedAt.IsZero() {
		end = *j.StoppedAt
	}
	d := end.Sub(*j.StartedAt)
	if d < 0 {
		return ""
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", m, s)
}

// wordWrap splits a single line into substrings of at most maxWidth runes each,
// breaking at spaces where possible. Handles lines without spaces by hard-breaking.
func wordWrap(line string, maxWidth int) []string {
	if maxWidth <= 0 {
		maxWidth = 80
	}
	if utf8.RuneCountInString(line) <= maxWidth {
		return []string{line}
	}

	var result []string
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}

	current := ""
	for _, word := range words {
		wordLen := utf8.RuneCountInString(word)
		curLen := utf8.RuneCountInString(current)

		if curLen == 0 {
			if wordLen > maxWidth {
				// Hard-break a single word that's too long
				runes := []rune(word)
				for len(runes) > maxWidth {
					result = append(result, string(runes[:maxWidth]))
					runes = runes[maxWidth:]
				}
				current = string(runes)
			} else {
				current = word
			}
		} else if curLen+1+wordLen <= maxWidth {
			current += " " + word
		} else {
			result = append(result, current)
			if wordLen > maxWidth {
				runes := []rune(word)
				for len(runes) > maxWidth {
					result = append(result, string(runes[:maxWidth]))
					runes = runes[maxWidth:]
				}
				current = string(runes)
			} else {
				current = word
			}
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
