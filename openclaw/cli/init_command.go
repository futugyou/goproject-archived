package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

func InitCommandRun(args []string) (int, error) {
	outputDir, err := filepath.Abs(".openclaw-init")
	if err != nil {
		return 1, err
	}
	preset := "both"
	force := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			initCommandPrintHelp()
			return 0, nil

		case "--output":
			if i+1 >= len(args) {
				return 1, errors.New("Missing value for --output")
			}
			i++
			outputDir, err = filepath.Abs(args[i])
			if err != nil {
				return 1, err
			}

		case "--preset":
			if i+1 >= len(args) {
				return 1, errors.New("Missing value for --output")
			}
			i++

			preset = strings.ToLower(strings.TrimSpace(args[i]))
			if preset != "local" && preset != "public" && preset != "both" {
				return 1, errors.New("Invalid value for --preset (expected: local|public|both)")
			}
		case "--force":
			force = true
		default:
			return 1, fmt.Errorf("Unknown option: %s", arg)
		}
	}

	if util.IsDirNotEmptyFast(outputDir) && !force {
		return 1, fmt.Errorf("Output directory '%s' is not empty. Re-run with --force to overwrite generated files.", outputDir)
	}

	if err := os.MkdirAll(outputDir, 0700); err != nil {
		return 1, err
	}
	if err := os.MkdirAll(filepath.Join(outputDir, "workspace"), 0700); err != nil {
		return 1, err
	}
	if err := os.MkdirAll(filepath.Join(outputDir, "memory"), 0700); err != nil {
		return 1, err
	}
	if err := os.MkdirAll(filepath.Join(outputDir, "deploy"), 0700); err != nil {
		return 1, err
	}

	initCommandWrite(filepath.Join(outputDir, ".env.example"), initCommandBuildEnvExample(outputDir))

	if preset == "local" || preset == "both" {
		initCommandWrite(filepath.Join(outputDir, "config.local.json"), initCommandBuildLocalConfig())
	}

	if preset == "public" || preset == "both" {
		initCommandWrite(filepath.Join(outputDir, "config.public.json"), initCommandBuildPublicConfig())
		initCommandWrite(filepath.Join(outputDir, "deploy", "Caddyfile.sample"), initCommandBuildCaddyfileSample())
		initCommandWrite(filepath.Join(outputDir, "deploy", "docker-compose.override.sample.yml"), initCommandBuildDockerOverrideSample())
	}

	fmt.Printf("Initialized OpenClaw bootstrap files in %s\n", outputDir)
	fmt.Printf("- preset: %s\n", preset)
	fmt.Println("- workspace/: ready for file tools")
	fmt.Println("- memory/: ready for memory/session persistence")
	fmt.Println("- .env.example: provider/auth placeholders (fill in secrets there instead of passing them on the CLI)")
	fmt.Println("- config.local.json/config.public.json: ready-to-edit presets")
	return 0, nil
}

func initCommandPrintHelp() {
	fmt.Println(
		`
            openclaw init

            Usage:
              openclaw init [--preset local|public|both] [--output <dir>] [--force]

            Defaults:
              --preset both
              --output ./.openclaw-init
            `)
}

func initCommandWrite(path, contents string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(contents), 0644)
}

func initCommandBuildEnvExample(outputDir string) string {
	return fmt.Sprintf(`
MODEL_PROVIDER_KEY=replace-me
OPENCLAW_AUTH_TOKEN=replace-me
OPENCLAW_WORKSPACE=%s
`, filepath.Join(outputDir, "workspace"))
}

func initCommandBuildLocalConfig() string {
	f := core.GatewaySetupProfileFactory{}
	return initCommandSerializeConfig(
		f.CreateProfileConfig(
			"local",
			"127.0.0.1",
			18789,
			"",
			"./workspace",
			"./memory",
			"openai",
			"gpt-4o",
			"env:MODEL_PROVIDER_KEY", "", nil))
}

func initCommandBuildPublicConfig() string {
	f := core.GatewaySetupProfileFactory{}
	return initCommandSerializeConfig(
		f.CreateProfileConfig(
			"public",
			"0.0.0.0",
			18789,
			"",
			"/app/workspace",
			"/app/memory",
			"openai",
			"gpt-4o",
			"env:MODEL_PROVIDER_KEY", "", nil))
}

func initCommandSerializeConfig(config *core.GatewayConfig) string {
	doc := map[string]any{}
	doc["OpenClaw"] = config
	data, _ := json.MarshalIndent(doc, "", "  ")
	return string(data)
}

func initCommandBuildCaddyfileSample() string {
	return `
:443 {
    encode zstd gzip

    reverse_proxy 127.0.0.1:18789
}
`
}

func initCommandBuildDockerOverrideSample() string {
	return `
services:
  openclaw:
    environment:
      - OPENCLAW_AUTH_TOKEN=${OPENCLAW_AUTH_TOKEN}
      - OpenClaw__BindAddress=0.0.0.0
      - OpenClaw__Tooling__AllowShell=false
      - OpenClaw__Tooling__AllowedReadRoots__0=/app/workspace
      - OpenClaw__Tooling__AllowedWriteRoots__0=/app/workspace
      - OpenClaw__Plugins__Enabled=false
`
}
