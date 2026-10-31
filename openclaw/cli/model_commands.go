package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

func ModelCommandRun(subcommand string, args []string, output io.Writer, stderr io.Writer) (int, error) {
	if output == nil {
		output = os.Stdout
	}
	if stderr == nil {
		stderr = os.Stderr
	}

	packids := getPositionals(args)
	packageId := ""
	if len(packids) > 0 {
		packageId = packids[0]
	}

	parsed := CliArgsParse(args[1:])
	modelsRoot := util.Deref(parsed.GetOption("--models-root"))

	switch subcommand {
	case "packages":
		for _, pack := range core.LocalModelPackageDefinitionPackages {
			fmt.Fprintf(output, "- %s | preset=%s | model=%s | %s\n", pack.Id, pack.PresetId, pack.ModelId, pack.DisplayName)
			fmt.Fprintf(output, "  backend=%s format=%s quant=%s context=%d experimental=%t ram=%d-%dGB license=%s\n", pack.Runtime.Backend, pack.Format, pack.Quantization, pack.ContextWindow, pack.Experimental, pack.MinRamGb, pack.RecommendedRamGb, pack.LicenseUrl)
			if pack.Runtime.Backend == "litert" {
				fmt.Fprintf(output, "  prerequisite=OpenClaw:LocalInference:LiteRtRuntimePath must point to an OpenClaw-compatible LiteRT adapter binary\n")
			}
			for _, file := range core.LocalModelCacheInstace.GetPackageFiles(&pack) {
				fmt.Fprintf(output, "  file[%s] %s required=%t sha256=%s\n", file.Role, file.FileName, file.Required, file.ExpectedSha256)
			}
		}
		return 0, nil
	case "status":
		if packageId == "" {
			for _, status := range core.LocalModelCacheInstace.ListStatuses(modelsRoot) {
				printModelCommandStatus(&status, output)
			}
			return 0, nil
		}

		statusPackage, ok := tryGetPackage(packageId, stderr)
		if !ok {
			return 2, nil
		}

		printModelCommandStatus(core.LocalModelCacheInstace.GetStatus(statusPackage, modelsRoot), output)
		return 0, nil

	case "install":
		installPackage, ok := tryGetPackage(packageId, stderr)
		if !ok {
			return 2, nil
		}

		install, err := core.LocalModelCacheInstace.Install(context.Background(),
			installPackage,
			&core.LocalModelInstallRequest{
				SourcePath:              util.Deref(parsed.GetOption("--path")),
				MultimodalProjectorPath: util.Deref(parsed.GetOption("--mmproj-path")),
				DraftModelPath:          util.Deref(parsed.GetOption("--draft-path")),
				SourceUrl:               util.Deref(parsed.GetOption("--download-url")),
				BearerToken:             util.Deref(parsed.GetOption("--token")),
				AcceptLicense:           parsed.HasFlag("--accept-license"),
				ModelsRoot:              modelsRoot,
				DownloadOptionalFiles:   !parsed.HasFlag("--no-optional-files"),
			})
		if err != nil {
			return 2, err
		}
		output.Write([]byte(install.Message))
		if install.Status != nil {
			printModelCommandStatus(install.Status, output)
		}
		if install.Success {
			return 0, nil
		}
		return 1, nil

	case "verify":
		verifyPackage, ok := tryGetPackage(packageId, stderr)
		if !ok {
			return 2, nil
		}

		verified, err := core.LocalModelCacheInstace.Verify(context.Background(), verifyPackage, modelsRoot)
		if err != nil {
			return 2, err
		}
		printModelCommandStatus(verified, output)
		if verified.Verified {
			return 0, nil
		}
		return 1, nil
	case "remove":
		removePackage, ok := tryGetPackage(packageId, stderr)
		if !ok {
			return 2, nil
		}

		removed := core.LocalModelCacheInstace.Remove(removePackage, modelsRoot)
		if removed {
			output.Write([]byte("Removed " + removePackage.Id))
		} else {
			output.Write([]byte(removePackage.Id + " was not installed."))
		}
		return 0, nil
	default:
		return 2, nil
	}
}

func getPositionals(args []string) []string {
	result := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			result = append(result, arg)
			continue
		}

		if arg == "--accept-license" || arg == "--no-optional-files" {
			continue
		}

		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
			i++
		}
	}

	return result
}

func printModelCommandStatus(status *core.LocalModelPackageStatus, output io.Writer) {
	fmt.Fprintf(output, "- %s | installed=%t | verified=%t | model=%s\n", status.PackageId, status.Installed, status.Verified, status.ModelId)
	if pack, ok := core.TryGetLocalModelPackage(status.PackageId); ok {
		fmt.Fprintf(output, "  backend=%s format=%s context=%d experimental=%t\n", pack.Runtime.Backend, pack.Format, pack.ContextWindow, pack.Experimental)
		if pack.Runtime.Backend == "litert" {
			fmt.Fprintf(output, "  prerequisite=OpenClaw:LocalInference:LiteRtRuntimePath must point to an OpenClaw-compatible LiteRT adapter binary\n")
		}
	}
	fmt.Fprintf(output, "  path=%s\n", status.ModelPath)

	if status.Sha256 != "" {
		fmt.Fprintf(output, "  sha256=%s\n", status.Sha256)
	}

	for _, file := range status.Files {
		fmt.Fprintf(output, "  file[%s] installed=%t} verified=%t required=%t path=%s\n", file.Role, file.Installed, file.Verified, file.Required, file.Path)
		if file.Sha256 != "" {
			fmt.Fprintf(output, "    sha256=%s\n", file.Sha256)
		}
		if file.Issue != "" {
			fmt.Fprintf(output, "    Issue=%s\n", file.Issue)
		}
	}

	if status.Issue != "" {
		fmt.Fprintf(output, "  Issue=%s\n", status.Issue)
	}
}

func tryGetPackage(packageId string, output io.Writer) (*core.LocalModelPackageDefinition, bool) {
	if pack, ok := core.TryGetLocalModelPackage(packageId); ok {
		return pack, true
	}

	if packageId == "" {
		output.Write([]byte("Package id is required."))
	} else {
		fmt.Fprintf(output, "Unknown local model package: %s\n", packageId)
	}

	return nil, false
}
