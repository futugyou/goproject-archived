package image

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/futugyou/openclaw/agent/tool/pathpolicy"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type ImageGenTool struct {
	config        *core.ImageGenConfig
	modelProfiles core.IModelProfileRegistry
	tooling       *core.ToolingConfig
	httpClient    *http.Client
}

func NewImageGenTool(config *core.ImageGenConfig,
	modelProfiles core.IModelProfileRegistry,
	tooling *core.ToolingConfig) *ImageGenTool {
	return &ImageGenTool{config: config, modelProfiles: modelProfiles, tooling: tooling}
}

func (a *ImageGenTool) Name() string {
	return "image_gen"
}

func (a *ImageGenTool) Description() string {
	return "Generate an image from a text prompt. Returns the generated image as Markdown " +
		"(![generated image](url)) plus its URL. Include the returned Markdown image verbatim " +
		"in your reply so the user can see the picture."
}

func (a *ImageGenTool) ParameterSchema() string {
	return `{
          "type": "object",
          "properties": {
            "prompt": {
              "type": "string",
              "description": "Text description of the image to generate"
            },
            "size": {
              "type": "string",
              "description": "Image size: 1024x1024, 1792x1024, 1024x1792",
              "default": "1024x1024"
            },
            "quality": {
              "type": "string",
              "description": "Image quality: standard or hd",
              "default": "standard"
            }
          },
          "required": ["prompt"]
        }`
}

type ImageGenModel struct {
	Prompt  string `json:"Prompt"`
	Size    string `json:"size"`
	Quality string `json:"quality"`
}

func (a *ImageGenTool) Execute(ctx context.Context, argumentsJson string) string {
	var args ImageGenModel
	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return fmt.Sprintf("Error: Invalid argument JSON: %v", err)
	}

	if args.Size == "" {
		args.Size = a.config.Size
	}

	if args.Quality == "" {
		args.Quality = a.config.Quality
	}

	var effective = a.resolveEffectiveConfig()
	switch strings.ToLower(effective.Provider) {
	case "openai":
		return a.generateOpenAi(ctx, args.Prompt, args.Size, args.Quality, effective)
	default:
		return fmt.Sprintf("Error: Unsupported image generation provider '%s'.", effective.Provider)
	}
}

func ResolveOpenAiEndpoint(configuredEndpoint string) string {
	if configuredEndpoint != "" {
		return configuredEndpoint
	}

	return "https://api.openai.com/v1"
}

func (a *ImageGenTool) generateOpenAi(ctx context.Context, prompt string, size string, quality string, effective EffectiveImageGenConfig) string {
	var apiKey = core.SecretResolverInstance.Resolve(effective.ApiKey)
	if apiKey == "" {
		return "Error: API key not configured. Set ImageGen.ApiKey or bind ImageGen.ModelProfileId to a profile with ApiKey."
	}

	var endpoint = ResolveOpenAiEndpoint(effective.Endpoint)
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	if strings.TrimSpace(endpoint) != "" {
		opts = append(opts, option.WithBaseURL(endpoint))
	}

	client := openai.NewClient(opts...)
	reqCtx := ctx
	if a.config.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, time.Duration(a.config.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	res, err := client.Images.Generate(reqCtx, openai.ImageGenerateParams{
		Prompt:         prompt,
		Quality:        openai.ImageGenerateParamsQuality(quality),
		ResponseFormat: openai.ImageGenerateParamsResponseFormatURL,
		Size:           openai.ImageGenerateParamsSize(size),
	})

	if err != nil {
		return "Error: Image generation request failed"
	}

	if len(res.Data) == 0 {
		return "no image generated"
	}

	url := res.Data[0].URL
	if url != "" {
		return formatImageResult(url, res.Data[0].RevisedPrompt)
	}

	b64json := res.Data[0].B64JSON
	if b64json != "" {
		imgBytes, err := base64.StdEncoding.DecodeString(b64json)
		if err != nil {
			return fmt.Sprintf("base64 decode failed: %s", err.Error())
		}

		var savedPath = a.saveImageBytes(ctx, imgBytes, "image/png")
		if savedPath == "" {
			return "Error: image data was returned but could not be saved. Ensure Tooling.WorkspaceRoot or an AllowedWriteRoot is configured."
		}

		return formatImagePathResult(savedPath)
	}

	return "Image generated but no URL or data returned."
}

func (a *ImageGenTool) saveImageBytes(_ context.Context, imgBytes []byte, mimeType string) string {
	if len(imgBytes) == 0 {
		return ""
	}

	var downloadsDir = a.resolveDownloadsDirectory()
	if downloadsDir == "" {
		return ""
	}

	os.MkdirAll(downloadsDir, 0755)

	var fileName = fmt.Sprintf("image_%s%s", util.CleanUUID()[:8], mimeToExtension(mimeType))
	var destPath = filepath.Join(downloadsDir, fileName)
	err := os.WriteFile(destPath, imgBytes, 0644)
	if err != nil {
		return fmt.Sprintf("write file failed: %s", err.Error())
	}

	return destPath

}

func mimeToExtension(mimeType string) string {
	switch strings.ToLower(mimeType) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

func (a *ImageGenTool) resolveDownloadsDirectory() string {
	if a.tooling == nil {
		return ""
	}

	var workspaceRaw = core.SecretResolverInstance.Resolve(a.tooling.WorkspaceRoot)
	if workspaceRaw == "" {
		dir, err := os.Getwd()
		if err != nil {
			return ""
		}
		workspaceRaw = dir
	}

	path, err := filepath.Abs(workspaceRaw)
	if err != nil {
		return ""
	}
	var downloadsDir = filepath.Join(path, ".downloads")
	if pathpolicy.IsWriteAllowed(*a.tooling, downloadsDir) {
		return downloadsDir
	}
	return ""
}

func formatImagePathResult(path string) string {
	return fmt.Sprintf("Image generated.\n[IMAGE_PATH:%s]\n", path)
}

func formatImageResult(url, revisedPrompt string) string {
	var sb = strings.Builder{}
	fmt.Fprintf(&sb, "![generated image](%s)\n", url)
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "Image URL: %s\n", url)
	if revisedPrompt != "" {
		fmt.Fprintf(&sb, "Revised prompt: %s\n", revisedPrompt)
	}
	return sb.String()
}

type EffectiveImageGenConfig struct {
	Provider string
	Model    string
	Endpoint string
	ApiKey   string
}

func Normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func mapImageProvider(providerId string) string {
	var provider = Normalize(providerId)
	switch provider {
	case "openai", "openai-compatible", "azure-openai", "aperture", "groq", "together", "lmstudio":
		return "openai"
	case "dashscope", "qwen":
		return "dashscope"
	default:
		return ""
	}
}

func (a *ImageGenTool) resolveEffectiveConfig() EffectiveImageGenConfig {
	var provider = a.config.Provider
	var model = a.config.Model
	var endpoint = a.config.Endpoint
	var apiKey = a.config.ApiKey

	var profileId = Normalize(a.config.ModelProfileId)
	if profileId == "" {
		if a.modelProfiles != nil {
			profileId = Normalize(a.modelProfiles.DefaultProfileId())
		}

	}

	if a.modelProfiles != nil && profileId != "" {
		if profile, ok := a.modelProfiles.TryGet(profileId); ok && profile != nil {
			var mappedProvider = mapImageProvider(profile.ProviderId)
			if strings.TrimSpace(mappedProvider) != "" {
				provider = mappedProvider
				model = profile.ModelId
				endpoint = profile.BaseUrl
				apiKey = profile.ApiKey
			}
		}
	}

	return EffectiveImageGenConfig{Provider: provider, Model: model, Endpoint: endpoint, ApiKey: apiKey}
}
