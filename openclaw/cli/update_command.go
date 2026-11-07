package cli

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/futugyou/openclaw/core"
)

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
	default:
		return 2, fmt.Errorf("Unknown update command: %s", command)
	}
}
