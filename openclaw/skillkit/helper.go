package skillkit

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func SkillIdGenerate(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "general.untitled_skill"
	}

	var tokens = skillIdTokenize(name)
	if len(tokens) == 0 {
		return "general.untitled_skill"
	}

	if len(tokens) >= 2 && tokens[0] == "asp" && tokens[1] == "net" {
		tokens[0] = "aspnet"
		tokens = append(tokens[:1], tokens[2:]...)
	}

	if len(tokens) > 2 && tokens[len(tokens)-1] == "extractor" {
		tokens = tokens[:len(tokens)-1]
	}

	var prefix = tokens[0]
	suffixTokens := []string{"skill"}
	if len(tokens) > 1 {
		suffixTokens = tokens[1:]
	}

	return fmt.Sprintf("%s.%s", prefix, strings.Join(suffixTokens, "_"))
}

func skillIdTokenize(value string) []string {
	// 1. Unicode Normalization (Form KD)
	normalized := norm.NFKD.String(value)

	var builder strings.Builder
	previousWasSeparator := true

	for _, ch := range normalized {
		// 2. filter NonSpacingMark
		if unicode.In(ch, unicode.Mn) { // Mn = Mark, nonspacing
			continue
		}

		// 3. filter Letter Digit
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) {
			builder.WriteRune(unicode.ToLower(ch))
			previousWasSeparator = false
			continue
		}

		// 4. sep
		if !previousWasSeparator {
			builder.WriteRune(' ')
			previousWasSeparator = true
		}
	}

	return strings.Fields(builder.String())
}
