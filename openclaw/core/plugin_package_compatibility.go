package core

import (
	"fmt"
	"strconv"
	"strings"
)

var PluginPackageCompatibilityInstance = &PluginPackageCompatibility{}

type PluginPackageCompatibility struct {
	SupportedPluginApiVersion []int
	HostCompatibilityVersion  []int
}

func NewPluginPackageCompatibility() *PluginPackageCompatibility {
	return &PluginPackageCompatibility{
		SupportedPluginApiVersion: []int{2026, 5, 4},
		HostCompatibilityVersion:  []int{2026, 5, 4},
	}
}

func (c *PluginPackageCompatibility) Validate(plugin DiscoveredPlugin) []PluginCompatibilityDiagnostic {
	diagnostics := c.ValidateRange(
		plugin.PluginApiRange,
		plugin.MinHostVersion,
		plugin.Manifest.Id,
		plugin.RootPath,
	)

	if strings.TrimSpace(plugin.ExpectedIntegrity) != "" {
		diagnostics = append(diagnostics, PluginCompatibilityDiagnostic{
			Severity: "error",
			Code:     "package_integrity_unverified",
			Message:  fmt.Sprintf("Plugin '%s' declares package integrity, but extracted plugin directories cannot currently be verified against package-manager integrity metadata.", plugin.Manifest.Id),
			Surface:  "package_metadata",
			Path:     plugin.RootPath,
		})
	}

	return diagnostics
}

func (c *PluginPackageCompatibility) ValidateRange(pluginApiRange, minHostVersion, pluginId, path string) []PluginCompatibilityDiagnostic {
	var diagnostics []PluginCompatibilityDiagnostic

	c.validateFloor(
		pluginApiRange,
		c.SupportedPluginApiVersion,
		"plugin_api_version_unsupported",
		"plugin API",
		pluginId,
		path,
		&diagnostics,
	)
	c.validateFloor(
		minHostVersion,
		c.HostCompatibilityVersion,
		"host_version_unsupported",
		"host",
		pluginId,
		path,
		&diagnostics,
	)

	return diagnostics
}

func (c *PluginPackageCompatibility) validateFloor(
	declaredRange string,
	supportedVersion []int,
	diagnosticCode string,
	label string,
	pluginId string,
	path string,
	diagnostics *[]PluginCompatibilityDiagnostic,
) {
	if strings.TrimSpace(declaredRange) == "" {
		return
	}

	normalized := strings.TrimSpace(declaredRange)
	rangeOperators := []string{">=", "<=", "==", ">", "=", "^", "~"}
	for _, op := range rangeOperators {
		if strings.HasPrefix(normalized, op) {
			normalized = strings.TrimSpace(normalized[len(op):])
			break
		}
	}

	if strings.HasPrefix(normalized, "v") || strings.HasPrefix(normalized, "V") {
		normalized = normalized[1:]
	}

	var suffixIndices []int
	for _, char := range []byte{'-', '+', ' ', '<', '>', '|'} {
		if idx := strings.IndexByte(normalized, char); idx >= 0 {
			suffixIndices = append(suffixIndices, idx)
		}
	}
	if len(suffixIndices) > 0 {
		minIdx := suffixIndices[0]
		for _, idx := range suffixIndices[1:] {
			if idx < minIdx {
				minIdx = idx
			}
		}
		normalized = normalized[:minIdx]
	}

	if !strings.Contains(normalized, ".") && len(normalized) > 0 {
		normalized += ".0"
	}

	requiredVersion, err := parseVersion(normalized)
	if err != nil {
		*diagnostics = append(*diagnostics, PluginCompatibilityDiagnostic{
			Severity: "error",
			Code:     "invalid_plugin_version_range",
			Message:  fmt.Sprintf("Plugin '%s' declares an unsupported %s version range '%s'.", pluginId, label, declaredRange),
			Surface:  "package_metadata",
			Path:     path,
		})
		return
	}

	if compareVersions(requiredVersion, supportedVersion) <= 0 {
		return
	}

	*diagnostics = append(*diagnostics, PluginCompatibilityDiagnostic{
		Severity: "error",
		Code:     diagnosticCode,
		Message:  fmt.Sprintf("Plugin '%s' requires %s version %s, but this bridge supports %s.", pluginId, label, formatVersion(requiredVersion), formatVersion(supportedVersion)),
		Surface:  "package_metadata",
		Path:     path,
	})
}

func parseVersion(vStr string) ([]int, error) {
	parts := strings.Split(vStr, ".")
	var version []int
	for _, part := range parts {
		val, err := strconv.Atoi(part)
		if err != nil {
			return nil, err
		}
		version = append(version, val)
	}
	if len(version) == 0 {
		return nil, fmt.Errorf("empty version")
	}
	return version, nil
}

func compareVersions(v1, v2 []int) int {
	maxLen := max(len(v2), len(v1))
	for i := range maxLen {
		p1, p2 := 0, 0
		if i < len(v1) {
			p1 = v1[i]
		}
		if i < len(v2) {
			p2 = v2[i]
		}
		if p1 < p2 {
			return -1
		}
		if p1 > p2 {
			return 1
		}
	}
	return 0
}

func formatVersion(v []int) string {
	parts := make([]string, len(v))
	for i, val := range v {
		parts[i] = strconv.Itoa(val)
	}
	return strings.Join(parts, ".")
}
