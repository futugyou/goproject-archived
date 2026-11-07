package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/futugyou/openclaw/util"
)

type UpdateAsset struct {
	Rid    string `json:"Rid"`
	Url    string `json:"Url"`
	Sha256 string `json:"Sha256"`
	Size   int64  `json:"Size"`
}

type UpdateRelease struct {
	Version string        `json:"Version"`
	Channel string        `json:"Channel"`
	Assets  []UpdateAsset `json:"Assets"`
}

type UpdateFeed struct {
	SchemaVersion int             `json:"SchemaVersion"`
	ExpiresAtUtc  time.Time       `json:"ExpiresAtUtc"`
	Releases      []UpdateRelease `json:"Releases"`
}

type UpdateTrust struct {
	ManifestUrl  string `json:"ManifestUrl"`
	PublicKeyPem string `json:"PublicKeyPem"`
}

type BundleActivation struct {
	Current  string  `json:"Current"`
	Previous *string `json:"Previous,omitempty"`
}

type BundleUpdater struct {
	httpClient   *http.Client
	root         string
	timeProvider func() time.Time
	smokeCheck   func(string, context.Context) error
}

func NewBundleUpdater(client *http.Client, root string, timeProvider func() time.Time, smokeCheck func(string, context.Context) error) (*BundleUpdater, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if timeProvider == nil {
		timeProvider = time.Now
	}
	return &BundleUpdater{
		httpClient:   client,
		root:         absRoot,
		timeProvider: timeProvider,
		smokeCheck:   smokeCheck,
	}, nil
}

func BundleUpdaterDefaultRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".openclaw", "updates")
}

func (u *BundleUpdater) TrustPath() string {
	return filepath.Join(u.root, "trust.json")
}

func (u *BundleUpdater) activationPath() string {
	return filepath.Join(u.root, "active.json")
}

func (u *BundleUpdater) ReadActivation() (*BundleActivation, error) {
	data, err := os.ReadFile(u.activationPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var act BundleActivation
	if err := json.Unmarshal(data, &act); err != nil {
		return nil, err
	}
	return &act, nil
}

func requireHttps(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return errors.New("update downloads require HTTPS without embedded credentials")
	}
	return nil
}

func writeAtomic(path, text string) error {
	temp := path + "." + util.CleanUUID()
	if err := os.WriteFile(temp, []byte(text), 0600); err != nil {
		return err
	}
	defer os.Remove(temp)
	return os.Rename(temp, path)
}

func (u *BundleUpdater) ConfigureTrust(trust UpdateTrust) error {
	if err := requireHttps(trust.ManifestUrl); err != nil {
		return err
	}

	rsaPub, err := util.ParseRsaPublicKeyFromPem(trust.PublicKeyPem)
	if err != nil || rsaPub.Size()*8 < 3072 {
		return errors.New("update signing keys must be RSA 3072 bits or larger")
	}

	pemBytes, err := util.ParseDataFromRsaPublicKey(rsaPub)
	if err != nil {
		return err
	}

	publicTrust := UpdateTrust{
		ManifestUrl:  trust.ManifestUrl,
		PublicKeyPem: string(pemBytes),
	}

	if err := os.MkdirAll(u.root, 0755); err != nil {
		return err
	}

	data, err := json.Marshal(publicTrust)
	if err != nil {
		return err
	}

	return writeAtomic(u.TrustPath(), string(data))
}
