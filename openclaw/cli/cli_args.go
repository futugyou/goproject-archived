package cli

import (
	"strings"
)

type CliArgs struct {
	Options     map[string][]string
	flags       map[string]struct{}
	Positionals []string
	Files       []string
	Images      []string
	ShowHelp    bool
}

func NewCliArgs() *CliArgs {
	return &CliArgs{
		Positionals: make([]string, 0),
		flags:       make(map[string]struct{}),
		Files:       make([]string, 0),
		Images:      make([]string, 0),
		Options:     make(map[string][]string),
	}
}
func (c *CliArgs) HasFlag(name string) bool {
	_, ok := c.flags[name]
	return ok
}

func (c *CliArgs) GetOption(name string) *string {
	v, ok := c.Options[name]
	if ok && len(v) > 0 {
		return &v[len(v)-1]
	}
	return nil
}

func IsFlagOption(value string) bool {
	switch value {
	case "--no-stream", "--apply", "--non-interactive", "--offline", "--strict", "--require-provider", "--with-companion", "--open-browser", "--skip-verify", "--json", "--anonymize", "--test", "--dry-run", "--yes", "--accept-license", "--no-optional-files":
		return true
	}

	return false
}

func CliArgsParse(args []string) *CliArgs {
	parsed := NewCliArgs()

	for i := 0; i < len(args); i++ {
		a := args[i]

		if a == "-h" || a == "--help" {
			parsed.ShowHelp = true
			continue
		}

		if a == "--" {
			parsed.Positionals = append(parsed.Positionals, args[i+1:]...)
			break
		}

		if !strings.HasPrefix(a, "--") {
			parsed.Positionals = append(parsed.Positionals, a)
			continue
		}

		if IsFlagOption(a) {
			parsed.flags[a] = struct{}{}
			continue
		}

		if i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			return nil
		}

		i++
		value := args[i]

		if a == "--file" {
			parsed.Files = append(parsed.Files, value)
			continue
		}

		if a == "--image" {
			parsed.Images = append(parsed.Images, value)
			continue
		}

		parsed.Options[a] = append(parsed.Options[a], value)
	}

	return parsed
}
