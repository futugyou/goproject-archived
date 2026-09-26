package capability

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/futugyou/openclaw/agent/tool/mcpnative"
	"github.com/futugyou/openclaw/core"
)

type LocalCapabilityProvider struct {
	tools        func() []core.ITool
	runtimeTools []core.ITool
}

func NewLocalCapabilityProvider(tools func() []core.ITool) *LocalCapabilityProvider {
	return &LocalCapabilityProvider{tools: tools}
}

func (l *LocalCapabilityProvider) SetTools(available []core.ITool) {
	l.runtimeTools = nil
	runtimeTools := []core.ITool{}
	for _, t := range available {
		if _, ok := t.(*mcpnative.McpNativeTool); !ok {
			runtimeTools = append(runtimeTools, t)
		}
	}

	l.runtimeTools = runtimeTools
}

func (l *LocalCapabilityProvider) Bind(ctx context.Context, target string, tool *string) (*core.CapabilityTarget, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	tools := l.runtimeTools
	if len(tools) == 0 && l.tools != nil {
		tools = l.tools()
	}

	name := target
	if tool != nil {
		name = *tool
	}

	var match core.ITool
	for _, v := range tools {
		if v.Name() == name && v.Name() != "resolve_capability" {
			match = v
			break
		}
	}

	if match == nil {
		return nil, errors.New("no matching cap")
	}

	retrySafe := false // TODO
	return core.NewCapabilityTarget(target, match, retrySafe), nil
}

func (l *LocalCapabilityProvider) Discover(ctx context.Context, request core.ResolveCapabilityRequest) ([]core.CapabilityCandidate, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	term := request.TaskDescription
	if t := request.KeyWords; t != nil && *t != "" {
		term = *t
	}

	matches := []core.CapabilityCandidate{}
	tools := l.runtimeTools
	if len(tools) == 0 && l.tools != nil {
		tools = l.tools()
	}

	terms := strings.Split(term, ", ")

	slices.SortFunc(tools, func(a, b core.ITool) int {
		return strings.Compare(a.Name(), b.Name())
	})
	for i, tool := range tools {
		f := false
		for _, v := range terms {
			if strings.Contains(tool.Name()+" "+tool.Description(), v) {
				f = true
				break
			}
		}
		if tool.Name() != "resolve_capability" && (tool.Name() == request.TaskDescription || f) {
			matches = append(matches, core.CapabilityCandidate{
				Name:        tool.Name(),
				Description: tool.Description(),
				Rank:        i + 1,
			})
		}

	}

	return matches, nil

}

func (l *LocalCapabilityProvider) Id() string {
	return "local"
}
