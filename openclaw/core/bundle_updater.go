package core

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

func (u *BundleUpdater) downloadBytes(ctx context.Context, urlStr string, max int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	if err := requireHttps(resp.Request.URL.String()); err != nil {
		return nil, err
	}

	limited := io.LimitReader(resp.Body, max+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, errors.New("update download exceeds its allowed size")
	}
	return data, nil
}

func (u *BundleUpdater) Check(ctx context.Context, channel string, version *string) (UpdateRelease, error) {
	if channel != "stable" && channel != "beta" {
		return UpdateRelease{}, errors.New("channel must be stable or beta")
	}

	trustData, err := os.ReadFile(u.TrustPath())
	if err != nil {
		return UpdateRelease{}, errors.New("configure a manifest URL and independently verified publisher public key first")
	}

	var trust UpdateTrust
	if err := json.Unmarshal(trustData, &trust); err != nil {
		return UpdateRelease{}, errors.New("invalid update trust configuration")
	}

	if err := requireHttps(trust.ManifestUrl); err != nil {
		return UpdateRelease{}, err
	}

	manifestBytes, err := u.downloadBytes(ctx, trust.ManifestUrl, 1024*1024)
	if err != nil {
		return UpdateRelease{}, err
	}

	sigBytes, err := u.downloadBytes(ctx, trust.ManifestUrl+".sig", 16*1024)
	if err != nil {
		return UpdateRelease{}, err
	}

	feed, err := VerifyFeed(manifestBytes, sigBytes, trust.PublicKeyPem, u.timeProvider())
	if err != nil {
		return UpdateRelease{}, err
	}

	return SelectRelease(feed, channel, version)
}

func VerifyFeed(manifest, signature []byte, pemData string, now time.Time) (UpdateFeed, error) {
	rsaPub, err := util.ParseRsaPublicKeyFromPem(pemData)
	if err != nil || rsaPub.Size()*8 < 3072 {
		return UpdateFeed{}, errors.New("RSA key must be at least 3072 bits")
	}

	hashed := sha256.Sum256(manifest)
	err = rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, hashed[:], signature)
	if err != nil {
		return UpdateFeed{}, errors.New("update manifest signature verification failed")
	}

	var feed UpdateFeed
	if err := json.Unmarshal(manifest, &feed); err != nil {
		return UpdateFeed{}, errors.New("update manifest is invalid or expired")
	}

	if feed.SchemaVersion != 1 || feed.ExpiresAtUtc.Before(now) || feed.Releases == nil {
		return UpdateFeed{}, errors.New("update manifest is invalid or expired")
	}

	return feed, nil
}

func compareSemanticVersions(left, right string) (int, error) {
	parse := func(val string) ([]int, []string, error) {
		parts := strings.SplitN(val, "-", 2)
		mainParts := strings.Split(parts[0], ".")
		if len(mainParts) != 3 {
			return nil, nil, errors.New("invalid semver")
		}
		main := make([]int, 3)
		for i, p := range mainParts {
			if len(p) == 0 || (len(p) > 1 && p[0] == '0') {
				return nil, nil, errors.New("invalid semver")
			}
			var n int
			if _, err := fmt.Sscanf(p, "%d", &n); err != nil {
				return nil, nil, err
			}
			main[i] = n
		}
		var pre []string
		if len(parts) > 1 {
			pre = strings.Split(parts[1], ".")
		}
		return main, pre, nil
	}

	lm, lp, err := parse(left)
	if err != nil {
		return 0, err
	}
	rm, rp, err := parse(right)
	if err != nil {
		return 0, err
	}

	for i := 0; i < 3; i++ {
		if lm[i] != rm[i] {
			if lm[i] < rm[i] {
				return -1, nil
			}
			return 1, nil
		}
	}
	if len(lp) == 0 || len(rp) == 0 {
		if len(lp) == len(rp) {
			return 0, nil
		}
		if len(lp) == 0 {
			return 1, nil
		}
		return -1, nil
	}
	return 0, nil
}

func validateVersion(version string) error {
	if version == "" || len(version) > 80 || strings.Contains(version, "..") {
		return errors.New("invalid release version")
	}
	for _, c := range version {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-') {
			return errors.New("invalid release version")
		}
	}
	_, err := compareSemanticVersions(version, version)
	return err
}

func SelectRelease(feed UpdateFeed, channel string, version *string) (UpdateRelease, error) {
	var found *UpdateRelease
	for _, r := range feed.Releases {
		if r.Channel == channel && (version == nil || r.Version == *version) {
			found = &r
			break
		}
	}
	if found == nil {
		return UpdateRelease{}, errors.New("the signed feed has no matching release")
	}
	if err := validateVersion(found.Version); err != nil {
		return UpdateRelease{}, err
	}
	return *found, nil
}
