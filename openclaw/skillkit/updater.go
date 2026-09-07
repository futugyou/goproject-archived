package skillkit

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/futugyou/openclaw/util"
)

type SkillTraceUpdater struct {
}

func (s *SkillTraceUpdater) Append(ctx context.Context, pkg SkillPackage, message string) error {
	sanitizeTraceMessage := func(message string) string {
		sb := strings.Builder{}
		for _, ch := range message {
			if unicode.IsControl(ch) {
				sb.WriteByte(' ')
			} else {
				sb.WriteRune(ch)
			}
		}

		return strings.TrimSpace(sb.String())
	}
	tracePath, err := ResolvePackageFilePath(pkg.RootPath, "trace.md")
	if err != nil {
		return err
	}
	var sanitizedMessage = sanitizeTraceMessage(message)
	var line = fmt.Sprintf("- %s: %s\n", time.Now().Format(time.RFC3339Nano), sanitizedMessage)
	if !util.FileExists(tracePath) {
		var renderer = &SkillTemplateRenderer{}
		util.SaveFile(ctx, tracePath, renderer.RenderTrace(pkg.Manifest, "trace recreated"))
	}

	f, err := os.OpenFile(tracePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(line)
	return err
}
