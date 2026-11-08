package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
)

func resolveLaunchComponent(args []string) string {
	re := positionalArguments(args)
	if len(re) <= 1 {
		return "companion"
	}

	return re[1]
}

func positionalArguments(args []string) []string {
	ressult := []string{}
	optionsWithValues := map[string]struct{}{
		"--root": {}, "--manifest": {}, "--key": {}, "--channel": {}, "--version": {},
	}
	for i := 0; i < len(args) && args[i] != "--"; i++ {
		if _, ok := optionsWithValues[args[i]]; ok {
			i++
			continue
		}

		if !strings.HasPrefix(args[i], "--") {
			ressult = append(ressult, args[i])
		}
	}
	return ressult
}

func UpdateCommandRun(args []string) (int, error) {
	option := func(key string) string {
		i := slices.Index(args, key)
		if i >= 0 && i+1 < len(args) {
			return args[i+1]
		}
		return ""
	}

	httpclient := &http.Client{Timeout: 15 * time.Second}
	root := option("--root")
	if root == "" {
		root = core.BundleUpdaterDefaultRoot()
	}

	updater, err := core.NewBundleUpdater(httpclient, root, nil, nil)
	if err != nil {
		return 1, err
	}
	command := ""
	if len(args) > 0 {
		command = args[0]
	}

	switch command {
	case "trust":
		manifest := option("--manifest")
		if manifest == "" {
			return 1, errors.New("--manifest is required")
		}
		keypath := option("--key")
		if keypath == "" {
			return 1, errors.New("--key PEM path is required")
		}

		data, err := os.ReadFile(keypath)
		if err != nil {
			return 1, err
		}

		if err := updater.ConfigureTrust(core.UpdateTrust{ManifestUrl: manifest, PublicKeyPem: string(data)}); err != nil {
			return 1, err
		}

		fmt.Println("Publisher trust saved. Verify the key fingerprint through an independent channel.")
		return 0, nil
	case "check":
		channel := option("--channel")
		if channel == "" {
			channel = "stable"
		}
		version := option("--version")
		release, err := updater.Check(context.Background(), channel, &version)
		if err != nil {
			return 1, err
		}

		fmt.Printf("%s: %s\n", release.Channel, release.Version)
		return 0, nil
	case "install":
		if !slices.Contains(args, "--yes") {
			return 1, errors.New("Review update check, then pass --yes to install and activate the bundle.")
		}
		channel := option("--channel")
		if channel == "" {
			channel = "stable"
		}
		version := option("--version")
		result, err := updater.Install(context.Background(), channel, &version, slices.Contains(args, "--allow-downgrade"))
		if err != nil {
			return 1, err
		}

		fmt.Println(result)
		fmt.Println("Bundle activated. Stop the old gateway before launching the new gateway. Use openclaw update launch companion (or cli/gateway).")
		return 0, nil
	case "rollback":
		if !slices.Contains(args, "--yes") {
			return 1, errors.New("Pass --yes to activate the previous bundle.")
		}
		result, err := updater.Rollback()
		if err != nil {
			return 1, err
		}

		fmt.Println(result)
		return 0, nil
	case "launch":
		component := resolveLaunchComponent(args)
		command, err := updater.GetActiveExecutable(component)
		if err != nil {
			return 1, err
		}
		separator := slices.Index(args, "--")
		cmdparam := []string{}
		for i := separator + 1; i < len(args); i++ {
			cmdparam = append(cmdparam, args[i])
		}
		cmd := exec.Command(command, cmdparam...)
		if err := cmd.Start(); err != nil {
			return 1, err
		}

		if err := cmd.Wait(); err != nil {
			return 1, err
		}

		exitCode := cmd.ProcessState.ExitCode()
		if exitCode == 0 {
			return 0, nil
		}
		return 1, errors.New("cannot launch active bundle")
	case "", "help", "--help", "-h":
		fmt.Println("openclaw update trust --manifest <https URL> --key <publisher.pem>\nopenclaw update check|install [--channel stable|beta] [--version <version>] [--allow-downgrade] [--yes]\nopenclaw update rollback --yes\nopenclaw update launch cli|gateway|companion [-- <arguments>]\nAll commands accept --root <managed update directory>.")
		return 0, nil

	default:
		return 2, fmt.Errorf("Unknown update command: %s", command)
	}
}
