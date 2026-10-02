package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/futugyou/openclaw/core"
)

func BackupCommandsRun(args []string) (int, error) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Println("backup create <plan.json> <new-directory> --offline\nbackup validate <backup-directory>\nbackup restore <backup-directory> <new-isolated-directory>\nStop all writers before create. Plans must cover configuration, sessions, goals, schedules, governance, and secret references.")
		return 0, nil
	}

	cmd := args[0]
	switch {
	case cmd == "create" && len(args) == 4 && args[3] == "--offline":
		planPath, err := filepath.Abs(args[1])
		if err != nil {
			return 1, err
		}

		data, err := os.ReadFile(planPath)
		if err != nil {
			return 1, err
		}

		var plan core.InstanceBackupPlan
		if err := json.Unmarshal(data, &plan); err != nil {
			return 1, fmt.Errorf("invalid backup plan: %w", err)
		}

		planDir := filepath.Dir(planPath)
		for key, rootPath := range plan.Roots {
			expanded := core.GatewaySetupPathsIntance.ExpandPath(rootPath)
			if !filepath.IsAbs(expanded) {
				expanded = filepath.Join(planDir, expanded)
			}
			absPath, err := filepath.Abs(expanded)
			if err != nil {
				return 1, err
			}
			plan.Roots[key] = absPath
		}

		if err := core.CreateBackup(plan, args[2], true); err != nil {
			return 1, err
		}
		fmt.Println("Backup created and checksums validated.")
		return 0, nil

	case cmd == "validate" && len(args) == 2:
		manifest, err := core.ValidateBackup(args[1])
		if err != nil {
			return 1, err
		}
		fmt.Printf("Validated %d files. No instance was started.\n", len(manifest.Files))
		return 0, nil

	case cmd == "restore" && len(args) == 3:
		if err := core.RestoreBackup(args[1], args[2]); err != nil {
			return 1, err
		}
		fmt.Println("Restored and validated in isolation. Review paths and secret references before starting an instance.")
		return 0, nil

	default:
		return 1, fmt.Errorf("invalid backup command. Run: openclaw backup --help")
	}
}
