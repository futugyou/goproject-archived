package git

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type GitTool struct {
	config core.GitToolsConfig
}

func New(config core.GitToolsConfig) *GitTool {
	return &GitTool{config: config}
}

func (a *GitTool) Name() string {
	return "git"
}

func (a *GitTool) Description() string {
	return "Run git operations on the local repository. Supports: status, diff, log, add, commit, branch, checkout, stash."
}

func (a *GitTool) ParameterSchema() string {
	return `{
          "type": "object",
          "properties": {
            "subcommand": {
              "type": "string",
              "description": "Git subcommand: status, diff, log, add, commit, branch, checkout, stash, show",
              "enum": ["status", "diff", "log", "add", "commit", "branch", "checkout", "stash", "show", "push", "pull", "reset"]
            },
            "args": {
              "type": "string",
              "description": "Additional arguments for the git command (optional)",
              "default": ""
            },
            "cwd": {
              "type": "string",
              "description": "Working directory (optional, defaults to current)"
            }
          },
          "required": ["subcommand"]
        }`
}

type GitModel struct {
	Subcommand string `json:"subcommand"`
	Args       string `json:"args"`
	Cwd        string `json:"cwd"`
}

var safeSubcommands = map[string]struct{}{
	"status": {}, "diff": {}, "log": {}, "add": {}, "commit": {}, "branch": {}, "checkout": {},
	"stash": {}, "show": {}, "fetch": {}, "merge": {}, "rebase": {}, "cherry-pick": {}, "tag": {},
}

var destructiveSubcommands = map[string]struct{}{
	"push": {}, "pull": {}, "reset": {},
}

func tokenizeGitArgs(args string) []string {
	var tokens = []string{}
	var current = strings.Builder{}
	var inQuotes = false
	var quoteChar = '\x00'

	for _, c := range args {

		if inQuotes {
			if c == quoteChar {
				inQuotes = false
			} else {
				current.WriteRune(c)
			}
		} else if c == '"' || c == '\'' {
			inQuotes = true
			quoteChar = c
		} else if unicode.IsSpace(c) {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(c)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

func (a *GitTool) runGit(ctx context.Context, gitArgs, cwd string) string {
	args := tokenizeGitArgs(gitArgs)
	result := util.RunProcess(ctx, "git", args, cwd, 30, int64(a.config.MaxDiffBytes), 8192)

	if result.Error != "" {
		return result.Error
	}

	var sb strings.Builder
	stdout := strings.TrimSpace(result.StdoutText)
	if stdout != "" {
		sb.WriteString(result.StdoutText)
		sb.WriteString("\n")
	}

	stderr := strings.TrimSpace(result.StderrText)
	if stderr != "" {
		if sb.Len() > 0 && !strings.HasSuffix(sb.String(), "\n") {
			sb.WriteString("\n")
		}
		fmt.Fprintf(&sb, "[stderr] %s", result.StderrText)
	}

	if sb.Len() > 0 {
		fmt.Fprintf(&sb, "\n(git %s completed with exit code %d)", gitArgs, result.ExitCode)
	}
	return sb.String()
}

func (a *GitTool) Execute(ctx context.Context, argumentsJson string) string {
	var args GitModel

	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return err.Error()
	}

	args.Subcommand = strings.ToLower(args.Subcommand)

	_, ok := destructiveSubcommands[args.Subcommand]
	if ok && !a.config.AllowPush {
		return fmt.Sprintf("Error: '%s' is disabled. Set GitTools.AllowPush = true to enable destructive operations.", args.Subcommand)
	}

	_, ok1 := safeSubcommands[args.Subcommand]

	if !ok && !ok1 {
		return fmt.Sprintf("Error: Unsupported git subcommand '%s'.", args.Subcommand)
	}

	// Block dangerous flag combinations even when destructive ops are allowed
	if args.Subcommand == "reset" && strings.Contains(args.Args, "--hard") && !a.config.AllowPush {
		return "Error: 'git reset --hard' is disabled. Set GitTools.AllowPush = true to enable."
	}

	// Validate args.Args against shell metacharacter injection
	if args.Args != "" {
		if err := core.Sanitizer.CheckShellMetaChars(args.Args, "args"); err != nil {
			return err.Error()
		}
	}

	// Build the git command
	fullArgs := args.Subcommand
	if args.Args != "" {
		fullArgs = fmt.Sprintf("%s %s", args.Subcommand, args.Args)
	}

	// For log, add sensible defaults if no format specified
	if args.Subcommand == "log" && !strings.Contains(args.Args, "--format") && !strings.Contains(args.Args, "--oneline") {
		fullArgs = strings.TrimSpace(fmt.Sprintf("log --oneline -20 %s", args.Args))
	}

	// For diff, use --stat first if no specific options
	if args.Subcommand == "diff" && args.Args == "" {
		fullArgs = "diff --stat"
	}

	return a.runGit(ctx, fullArgs, args.Cwd)
}
