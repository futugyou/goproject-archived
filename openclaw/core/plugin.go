package core

import (
	"slices"
	"strings"
)

type ExecutionHostKind uint8

const (
	ExecutionHostKind_Bridge ExecutionHostKind = iota
	ExecutionHostKind_NativeDynamic
)

const (
	Plugin_Const_Tools         = "tools"
	Plugin_Const_Services      = "services"
	Plugin_Const_Skills        = "skills"
	Plugin_Const_Channels      = "channels"
	Plugin_Const_Commands      = "commands"
	Plugin_Const_Providers     = "providers"
	Plugin_Const_Hooks         = "hooks"
	Plugin_Const_Memory        = "memory"
	Plugin_Const_NativeDynamic = "native_dynamic"
)

type PluginCapabilityPolicy struct{}

var PluginCapabilityPolicyInstance = &PluginCapabilityPolicy{}

func (p *PluginCapabilityPolicy) Normalize(capabilities []string) []string {
	tmps := map[string]struct{}{}
	resuls := []string{}
	for _, cap := range capabilities {
		if isBlank(cap) {
			continue
		}

		cap = strings.ToLower(strings.TrimSpace(cap))
		if _, ok := tmps[cap]; !ok {
			tmps[cap] = struct{}{}
			resuls = append(resuls, cap)
		}
	}

	slices.Sort(resuls)

	return resuls
}

func (p *PluginCapabilityPolicy) GetBlockedCapabilities(capabilities []string, hostKind ExecutionHostKind) []string {
	var normalized = p.Normalize(capabilities)
	switch hostKind {
	case ExecutionHostKind_Bridge:
		return []string{}
	case ExecutionHostKind_NativeDynamic:
		return normalized
	default:
		return normalized
	}
}
