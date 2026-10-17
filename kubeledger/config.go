package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func CreateDirectoryIfNotExists(path string) error {
	return os.MkdirAll(path, 0755)
}

func GetBackendConfigEnv(varName string, defaultValue *string) string {
	klVar := fmt.Sprintf("KL_%s", varName)
	if val, exists := os.LookupEnv(klVar); exists {
		return val
	}

	koaVar := fmt.Sprintf("KOA_%s", varName)
	if val, exists := os.LookupEnv(koaVar); exists {
		return val
	}

	if defaultValue != nil {
		return *defaultValue
	}
	return ""
}

type Config struct {
	Version               string
	DbRoundDecimals       int
	DbNonAllocatable      string
	DbBillingHourlyRate   string
	StaticContentLocation string
	FrontendDataLocation  string
	K8sApiEndpoint        string
	K8sVerifySsl          bool
	DbLocation            string
	PollingIntervalSec    int
	CostModel             string
	BillingCurrency       string
	EnableDebug           bool
	K8sAuthTokenFile      string
	K8sAuthToken          string
	K8sAuthTokenType      string
	K8sAuthUsername       string
	K8sAuthPassword       string
	K8sSslCacert          string
	K8sSslClientCert      string
	K8sSslClientCertKey   string
	IncludedNamespaces    []string
	ExcludedNamespaces    []string
	NvidiaDcgmEndpoint    string
	BillingHourlyRate     float64
	ListenerPort          int
	KoaVerifySslOption    any // string or bool
	K8sRbacAuthToken      string
}

func NewConfig() *Config {
	cfg := &Config{
		Version:               "26.05.0",
		DbRoundDecimals:       6,
		DbNonAllocatable:      "non-allocatable",
		DbBillingHourlyRate:   ".billing-hourly-rate",
		StaticContentLocation: "/static",
	}

	cfg.FrontendDataLocation = fmt.Sprintf(".%s/data", cfg.StaticContentLocation)

	defaultK8sEndpoint := "http://127.0.0.1:8001"
	cfg.K8sApiEndpoint = GetBackendConfigEnv("K8S_API_ENDPOINT", &defaultK8sEndpoint)

	verifySslStr := GetBackendConfigEnv("K8S_API_VERIFY_SSL", new("true"))
	cfg.K8sVerifySsl = strings.ToLower(verifySslStr) == "yes" || strings.ToLower(verifySslStr) == "true"

	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = "/tmp"
	}
	defaultDbLoc := fmt.Sprintf("%s/.kubeledger/db", homeDir)
	cfg.DbLocation = GetBackendConfigEnv("DB_LOCATION", &defaultDbLoc)

	pollingSecStr := GetBackendConfigEnv("POLLING_INTERVAL_SEC", new("300"))
	if v, err := strconv.Atoi(pollingSecStr); err == nil {
		cfg.PollingIntervalSec = v
	} else {
		cfg.PollingIntervalSec = 300
	}

	defaultCostModel := "CUMULATIVE_RATIO"
	cfg.CostModel = GetBackendConfigEnv("COST_MODEL", &defaultCostModel)

	defaultCurrency := "$"
	cfg.BillingCurrency = GetBackendConfigEnv("BILLING_CURRENCY_SYMBOL", &defaultCurrency)

	debugStr := GetBackendConfigEnv("ENABLE_DEBUG", new("false"))
	cfg.EnableDebug = strings.ToLower(debugStr) == "yes" || strings.ToLower(debugStr) == "true"

	defaultTokenFile := "/var/run/secrets/kubernetes.io/serviceaccount/token"
	cfg.K8sAuthTokenFile = GetBackendConfigEnv("K8S_AUTH_TOKEN_FILE", &defaultTokenFile)

	defaultAuthToken := "NO_ENV_AUTH_TOKEN"
	cfg.K8sAuthToken = GetBackendConfigEnv("K8S_AUTH_TOKEN", &defaultAuthToken)

	defaultTokenType := "Bearer"
	cfg.K8sAuthTokenType = GetBackendConfigEnv("K8S_AUTH_TOKEN_TYPE", &defaultTokenType)

	defaultUsername := "NO_ENV_AUTH_USERNAME"
	cfg.K8sAuthUsername = GetBackendConfigEnv("K8S_AUTH_USERNAME", &defaultUsername)

	defaultPassword := "NO_ENV_AUTH_PASSWORD"
	cfg.K8sAuthPassword = GetBackendConfigEnv("K8S_AUTH_PASSWORD", &defaultPassword)

	cfg.K8sSslCacert = GetBackendConfigEnv("K8S_CACERT", nil)

	defaultClientCert := "NO_ENV_CLIENT_CERT"
	cfg.K8sSslClientCert = GetBackendConfigEnv("K8S_AUTH_CLIENT_CERT", &defaultClientCert)

	defaultClientCertKey := "NO_ENV_CLIENT_CERT_CERT"
	cfg.K8sSslClientCertKey = GetBackendConfigEnv("K8S_AUTH_CLIENT_CERT_KEY", &defaultClientCertKey)

	includedStr := GetBackendConfigEnv("INCLUDED_NAMESPACES", new(""))
	cfg.IncludedNamespaces = parseNamespaces(includedStr)

	excludedStr := GetBackendConfigEnv("EXCLUDED_NAMESPACES", new(""))
	cfg.ExcludedNamespaces = parseNamespaces(excludedStr)

	cfg.NvidiaDcgmEndpoint = GetBackendConfigEnv("NVIDIA_DCGM_ENDPOINT", nil)

	cfg.ProcessBillingHourlyRateConfig()
	cfg.LoadRbacAuthToken()

	_ = CreateDirectoryIfNotExists(cfg.FrontendDataLocation)
	costModelLabel, costModelUnit := cfg.ProcessCostModelConfig()
	backendConfig := map[string]string{
		"cost_model": costModelLabel,
		"currency":   costModelUnit,
	}
	if fileData, err := json.Marshal(backendConfig); err == nil {
		_ = os.WriteFile(filepath.Join(cfg.FrontendDataLocation, "backend.json"), fileData, 0644)
	}

	listenerPortStr := GetBackendConfigEnv("LISTENER_PORT", nil)
	if v, err := strconv.Atoi(listenerPortStr); err == nil {
		cfg.ListenerPort = v
	} else {
		cfg.ListenerPort = 5483
	}

	if cfg.K8sVerifySsl && cfg.K8sSslCacert != "" {
		if _, err := os.Stat(cfg.K8sSslCacert); err == nil {
			cfg.KoaVerifySslOption = cfg.K8sSslCacert
		} else {
			cfg.KoaVerifySslOption = cfg.K8sVerifySsl
		}
	} else {
		cfg.KoaVerifySslOption = cfg.K8sVerifySsl
	}

	return cfg
}

func parseNamespaces(val string) []string {
	val = strings.ReplaceAll(val, " ", ",")
	parts := strings.Split(val, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func (c *Config) ProcessCostModelConfig() (string, string) {
	costModelLabel := "cumulative"
	costModelUnit := "%"
	switch c.CostModel {
	case "CHARGE_BACK":
		costModelLabel = "costs"
		costModelUnit = c.BillingCurrency
	case "RATIO":
		costModelLabel = "normalized"
		costModelUnit = "%"
	}
	return costModelLabel, costModelUnit
}

func (c *Config) ProcessBillingHourlyRateConfig() {
	rateStr := GetBackendConfigEnv("BILLING_HOURLY_RATE", new("-1"))
	if v, err := strconv.ParseFloat(rateStr, 64); err == nil {
		c.BillingHourlyRate = v
	} else {
		c.BillingHourlyRate = -1.0
	}
}

func (c *Config) LoadRbacAuthToken() {
	if data, err := os.ReadFile(c.K8sAuthTokenFile); err == nil {
		c.K8sRbacAuthToken = string(data)
	} else {
		c.K8sRbacAuthToken = "NO_ENV_TOKEN_FILE"
	}
}

func (c *Config) Match(items []string, pattern string) bool {
	for _, item := range items {
		if matched, err := filepath.Match(pattern, item); err == nil && matched {
			return true
		}
	}
	return false
}

func (c *Config) AllowNamespace(namespace string) bool {
	if c.Match(c.ExcludedNamespaces, namespace) {
		return false
	}

	noNamespaceIncluded := len(c.IncludedNamespaces) == 0
	allNamespacesEnabled := c.Match(c.IncludedNamespaces, "*")
	namespaceMatched := c.Match(c.IncludedNamespaces, namespace)

	return noNamespaceIncluded || allNamespacesEnabled || namespaceMatched
}

func RequestEfficiencyDbFileExtension() string {
	return "__rf"
}

func UsageEfficiencyDb(ns string) string {
	return ns + RequestEfficiencyDbFileExtension()
}

func GpuDbFileExtension() string {
	return "__gpu"
}

func GpuMetricsDb(ns string) string {
	return ns + GpuDbFileExtension()
}

func configureLogger(debugEnabled bool) *slog.Logger {
	var level slog.Level
	if debugEnabled {
		level = slog.LevelDebug
	} else {
		level = slog.LevelWarn
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewTextHandler(os.Stdout, opts)

	logger := slog.New(handler).With("name", "kubeledger")

	return logger
}

var KOA_CONFIG = NewConfig()
var KOA_LOGGER = configureLogger(KOA_CONFIG.EnableDebug)
