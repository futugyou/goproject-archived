package core

import (
	"fmt"
	"strings"
)

type SandboxCommandLine struct{}

func (s SandboxCommandLine) Quote(value string) string {
	escaped := strings.ReplaceAll(value, "'", "'\"'\"'")
	return "'" + escaped + "'"
}

func (s SandboxCommandLine) BuildCommand(command string, arguments []string) string {
	var builder strings.Builder

	builder.WriteString(s.Quote(command))

	if len(arguments) == 0 {
		return builder.String()
	}

	for _, argument := range arguments {
		builder.WriteByte(' ')
		builder.WriteString(s.Quote(argument))
	}

	return builder.String()
}

func (s SandboxCommandLine) WrapWithTimeout(command string, arguments []string, timeoutSeconds int) string {
	effectiveTimeout := timeoutSeconds
	if effectiveTimeout < 1 {
		effectiveTimeout = 1
	} else if effectiveTimeout > 3600 {
		effectiveTimeout = 3600
	}

	baseCommand := s.BuildCommand(command, arguments)

	return fmt.Sprintf(
		"if command -v timeout >/dev/null 2>&1; then exec timeout %ds %s; else exec %s; fi",
		effectiveTimeout,
		baseCommand,
		baseCommand,
	)
}
