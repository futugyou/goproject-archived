package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/futugyou/openclaw/skillkit"
	"github.com/futugyou/openclaw/util"
)

func skillKitCommandPrintHelp(output io.Writer) {
	output.Write([]byte(
		`
            openclaw skill

            Usage:
              openclaw skill new <name> [--category <category>] [--template <template>] [--output <path>] [--force]
              openclaw skill list [--output <path>]
              openclaw skill validate <skill-id|path> [--output <path>]
              openclaw skill critique <skill-id|path> [--output <path>]
              openclaw skill generate <skill-id|path> [--output <path>] [--force]
              openclaw skill package <skill-id|path> [--output <path>] [--package-output <path>] [--force]
              openclaw skill run <skill-id|path> --input <path> [--dry-run] [--output <path>]

            Defaults:
              --output .openclaw/skills
              --package-output .openclaw/packages
            `))
}

func firstPositional(args []string) string {
	requiresValue := func(arg string) bool {
		if arg == "--category" || arg == "--output" || arg == "--template" || arg == "--package-output" || arg == "--input" {
			return true
		}
		return false
	}

	for i := range args {
		if strings.HasPrefix(args[i], "-") {
			if requiresValue(args[i]) {
				i++
			}
			continue
		}
		return args[i]
	}

	return ""
}

func skillKitSResolveRoot(currentDirectory, output string) (string, error) {
	if output == "" {
		output = filepath.Join(".openclaw", "skills")
	}

	return skillKitResolvePath(currentDirectory, output)
}

func skillKitResolvePackagesRoot(currentDirectory, output string) (string, error) {
	if output == "" {
		output = filepath.Join(".openclaw", "packages")
	}

	return skillKitResolvePath(currentDirectory, output)
}

func skillKitResolvePath(currentDirectory, path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Abs(path)
	}

	return filepath.Abs(filepath.Join(currentDirectory, path))
}

func SkillKitSimpleCommandRun(args []string) (int, error) {
	return SkillKitCommandRun(args, os.Stdout, os.Stderr, ".")
}

func SkillKitCommandRun(
	args []string,
	output io.Writer,
	errwirter io.Writer,
	currentDirectory string) (int, error) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		skillKitCommandPrintHelp(output)
		return 0, nil
	}

	command := args[0]
	rest := args[1:]
	service := skillkit.CreateDefaultSkillPackageService()

	switch command {
	case "new":
		return skillKitCommandNew(service, rest, output, errwirter, currentDirectory)
	case "list", "ls":
		return skillKitCommandList(service, rest, output, errwirter, currentDirectory)
	case "validate":
		return skillKitCommandValidate(service, rest, output, errwirter, currentDirectory)
	}

	return 0, nil
}

func skillKitStatus(severity skillkit.SkillValidationSeverity) string {
	switch severity {
	case skillkit.SeverityError:
		return "FAIL"
	case skillkit.SeverityWarning:
		return "WARN"
	default:
		return "PASS"
	}
}

func skillKitPrintValidation(result skillkit.SkillValidationResult, output io.Writer) {
	passFail := "FAIL"
	if result.Passed() {
		passFail = "PASS"
	}
	fmt.Fprintf(output, "Skill validation: %s\n\n", passFail)
	groupedIssues := make(map[string][]skillkit.SkillValidationIssue)
	var areas []string
	for _, issue := range result.Issues {
		if _, exists := groupedIssues[issue.Area]; !exists {
			areas = append(areas, issue.Area)
		}
		groupedIssues[issue.Area] = append(groupedIssues[issue.Area], issue)
	}

	for _, area := range areas {
		fmt.Fprintf(output, "%s:\n", area)
		for _, issue := range groupedIssues[area] {
			fmt.Fprintf(output, "[%s] %s\n", skillKitStatus(issue.Severity), issue.Message)
		}
		fmt.Fprintln(output)
	}
}

func skillKitCommandValidate(service *skillkit.SkillPackageService, args []string, output, errwirter io.Writer, currentDirectory string) (int, error) {
	skillRef := firstPositional(args)
	if skillRef == "" {
		msg := "Usage: openclaw skill validate <skill-id|path> [--output <path>]"
		fmt.Fprintf(errwirter, "%s\n", msg)
		return 2, errors.New(msg)
	}

	skillsRoot, err := skillKitSResolveRoot(currentDirectory, getOptionValue(args, "--output"))
	if err != nil {
		return 1, err
	}
	result := service.Validate(context.Background(), skillRef, skillsRoot)
	skillKitPrintValidation(result, output)
	if result.Passed() {
		return 0, nil
	}

	return 1, errors.New("kit skill validate error ")
}

func skillKitCommandList(service *skillkit.SkillPackageService, args []string, output, errwirter io.Writer, currentDirectory string) (int, error) {
	skillsRoot, err := skillKitSResolveRoot(currentDirectory, getOptionValue(args, "--output"))
	if err != nil {
		return 1, err
	}
	pkgs, err := service.List(context.Background(), skillsRoot)
	if err != nil {
		return 1, err
	}
	if len(pkgs) == 0 {
		output.Write([]byte("No SkillKit skills found.\n"))
		return 0, nil
	}
	slices.SortFunc(pkgs, func(a, b skillkit.SkillPackage) int {
		return strings.Compare(a.Manifest.Id, b.Manifest.Id)
	})

	output.Write([]byte("ID                           Name                                  Category     Version\n"))
	for i := range pkgs {
		manifest := pkgs[i].Manifest
		fmt.Fprintf(output, "%-28s %-37s %-12s %s\n", util.Truncate(manifest.Id, 28), util.Truncate(manifest.Name, 37), util.Truncate(manifest.Category, 12), manifest.Version)
	}

	return 0, nil
}

func skillKitCommandNew(service *skillkit.SkillPackageService, args []string, output, errwirter io.Writer, currentDirectory string) (int, error) {
	name := firstPositional(args)
	if name == "" {
		msg := "Usage: openclaw skill new <name> [--category <category>] [--template <template>] [--output <path>] [--force]"
		errwirter.Write([]byte(msg))
		return 2, errors.New(msg)
	}

	category := getOptionValue(args, "--category")
	if category == "" {
		category = "general"
	}
	template := getOptionValue(args, "--template")
	if template == "" {
		template = category
	}
	skillsRoot, err := skillKitSResolveRoot(currentDirectory, getOptionValue(args, "--output"))
	if err != nil {
		return 1, err
	}
	force := slices.Contains(args, "--force")
	pkg, err := service.CreateNew(context.Background(), name, category, template, skillsRoot, force)
	if err != nil {
		return 1, err
	}

	fmt.Fprintf(output, "Created skill: %s\n", pkg.Manifest.Id)
	fmt.Fprintf(output, "Path: %s\n\n", pkg.RootPath)
	output.Write([]byte("Next steps:"))
	fmt.Fprintf(output, "- openclaw skill validate %s\n", pkg.Manifest.Id)
	fmt.Fprintf(output, "- openclaw skill validate %s\n", pkg.Manifest.Id)
	fmt.Fprintf(output, "- openclaw skill run %s --input <path> --dry-run\n", pkg.Manifest.Id)
	return 0, nil
}
