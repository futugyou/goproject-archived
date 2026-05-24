package graphify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pterm/pterm"
)

type ConfigPersistence struct{}

const localConfigFileName = ".env"
const secretConfigFileName = ".graphify_secrets.json"

func (cp ConfigPersistence) GetLocalConfigPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exePath), localConfigFileName), nil
}

func (cp ConfigPersistence) getSecretConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, secretConfigFileName), nil
}

func (cp ConfigPersistence) Save(config *GraphifyConfig) {
	path, err := cp.GetLocalConfigPath()
	if err != nil {
		pterm.Error.Printf("unable to retrieve configuration path: %v\n", err)
		return
	}

	if strings.TrimSpace(config.OpenAI.ApiKey) != "" {
		stored := cp.storeApiKeyInSecrets(config.OpenAI.ApiKey)
		if stored {
			pterm.Success.Println("🔑 API key has been securely stored in a local hidden secrets file.")
		} else {
			pterm.Warning.Printf("⚠️  Unable to store API key. Please manually create and configure %s in your user directory.\n", secretConfigFileName)
		}
	}

	wrapper := map[string]any{
		"Graphify": cp.buildSerializableConfig(config),
	}

	jsonBytes, err := json.MarshalIndent(wrapper, "", "  ")
	if err == nil {
		err = os.WriteFile(path, jsonBytes, 0644)
	}

	if err != nil {
		pterm.DefaultBox.
			WithTitle(pterm.Red("⚠ Save Failed")).
			Printf("%s %s\n%s %v\n\n%s Consider using environment variables (e.g., GRAPHIFY_PROVIDER) instead.",
				pterm.Red("Unable to write to:"), path,
				pterm.Red("Error reason:"), err,
				pterm.Yellow("Tip:"))
		return
	}

	pterm.Success.Printf("✅ Configuration saved to: %s\n", pterm.Gray(path))
	pterm.Gray("   (API keys are stored separately and will not be exposed in this file)\n")
}

func (cp ConfigPersistence) Load() *GraphifyConfig {
	path, err := cp.GetLocalConfigPath()
	if err != nil {
		return nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	jsonBytes, err := os.ReadFile(path)
	if err != nil {
		pterm.Warning.Printf("⚠️  Unable to read configuration file: %v\n", err)
		return nil
	}

	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(jsonBytes, &wrapper); err != nil {
		pterm.Warning.Printf("⚠️  Failed to parse JSON: %v\n", err)
		return nil
	}

	graphifyRaw, exists := wrapper["Graphify"]
	if !exists {
		return nil
	}

	var config GraphifyConfig
	if err := json.Unmarshal(graphifyRaw, &config); err != nil {
		pterm.Warning.Printf("⚠️  Failed to deserialize configuration: %v\n", err)
		return nil
	}

	return &config
}

func (cp ConfigPersistence) storeApiKeyInSecrets(apiKey string) bool {
	secretPath, err := cp.getSecretConfigPath()
	if err != nil {
		return false
	}

	secretData := map[string]string{
		"Graphify:OpenAI:ApiKey": apiKey,
	}

	jsonBytes, err := json.MarshalIndent(secretData, "", "  ")
	if err != nil {
		return false
	}

	done := make(chan bool, 1)
	go func() {
		err := os.WriteFile(secretPath, jsonBytes, 0600)
		done <- (err == nil)
	}()

	select {
	case success := <-done:
		return success
	case <-time.After(10 * time.Second):
		return false
	}
}

func (cp ConfigPersistence) buildSerializableConfig(config *GraphifyConfig) map[string]any {
	result := make(map[string]any)

	if config.Provider != "" {
		result["Provider"] = config.Provider
	}
	if config.WorkingFolder != "" {
		result["WorkingFolder"] = config.WorkingFolder
	}
	if config.OutputFolder != "" {
		result["OutputFolder"] = config.OutputFolder
	}
	if len(config.ExportFormats) > 0 {
		result["ExportFormats"] = config.ExportFormats
	}

	switch strings.ToLower(config.Provider) {
	case "openai":
		result["OpenAI"] = map[string]string{
			"Endpoint": config.OpenAI.Endpoint,
			"ModelId":  config.OpenAI.ModelId,
		}
	}

	return result
}
