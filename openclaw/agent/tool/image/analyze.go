package image

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type ImageAnalyzeTool struct {
	config *core.ImageAnalyzeConfig
}

func NewImageAnalyzeTool(config *core.ImageAnalyzeConfig) *ImageAnalyzeTool {
	if config == nil {
		config = &core.ImageAnalyzeConfig{}
	}
	return &ImageAnalyzeTool{config: config}
}

func (a *ImageAnalyzeTool) Name() string {
	return "image_analyze"
}

func (a *ImageAnalyzeTool) Description() string {
	return "Analyze or describe the content of one or more images. " +
		"Accepts local file paths or public URLs. " +
		"Returns a detailed description or answer based on the provided prompt."
}

func (a *ImageAnalyzeTool) ParameterSchema() string {
	return `{
          "type": "object",
          "properties": {
            "image_urls": {
              "type": "array",
              "items": { "type": "string" },
              "description": "Public URLs of images to analyze."
            },
            "image_paths": {
              "type": "array",
              "items": { "type": "string" },
              "description": "Local file paths of images to analyze (read and sent as base64)."
            },
            "prompt": {
              "type": "string",
              "description": "What to ask about the image(s), e.g. 'Describe the image' or 'Extract all text'.",
              "default": "Please describe the image(s) in detail."
            }
          }
        }`
}

type ImageAnalyzeModel struct {
	ImageUrls  []string `json:"image_urls"`
	ImagePaths []string `json:"image_paths"`
	Prompt     string   `json:"Prompt"`
}

func (a *ImageAnalyzeTool) Execute(ctx context.Context, argumentsJson string) string {
	var args ImageAnalyzeModel
	if err := json.Unmarshal([]byte(argumentsJson), &args); err != nil {
		return fmt.Sprintf("Error: Invalid argument JSON: %v", err)
	}

	var imageUrls []string
	for _, u := range args.ImageUrls {
		if strings.TrimSpace(u) != "" {
			imageUrls = append(imageUrls, u)
		}
	}

	var imagePaths []string
	for _, p := range args.ImagePaths {
		if strings.TrimSpace(p) != "" {
			imagePaths = append(imagePaths, p)
		}
	}

	prompt := strings.TrimSpace(args.Prompt)
	if prompt == "" {
		prompt = "Please describe the image(s) in detail."
	}

	if len(imageUrls) == 0 && len(imagePaths) == 0 {
		return "Error: No images provided. Supply at least one image_url or image_path."
	}

	totalImages := len(imageUrls) + len(imagePaths)
	if totalImages > a.config.MaxImagesPerCall {
		return fmt.Sprintf("Error: Too many images (%d). Maximum allowed per call is %d.", totalImages, a.config.MaxImagesPerCall)
	}

	apiKey := a.resolveKey()
	if strings.TrimSpace(apiKey) == "" {
		return "Error: Vision API key not configured. Set Plugins:Native:ImageAnalyze:ApiKey."
	}

	var parts []openai.ChatCompletionContentPartUnionParam
	parts = append(parts, openai.ChatCompletionContentPartUnionParam{
		OfText: &openai.ChatCompletionContentPartTextParam{
			Text: prompt,
		},
	})

	for _, url := range imageUrls {
		parts = append(parts, openai.ChatCompletionContentPartUnionParam{
			OfImageURL: &openai.ChatCompletionContentPartImageParam{
				ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
					URL: url,
				},
			},
		})
	}

	for _, path := range imagePaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return fmt.Sprintf("Error: File not found: %s", path)
		}

		bytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Sprintf("Error: Could not read image file '%s': %v", path, err)
		}

		mimeType := inferMimeType(path)
		base64Data := base64.StdEncoding.EncodeToString(bytes)
		dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

		parts = append(parts, openai.ChatCompletionContentPartUnionParam{
			OfImageURL: &openai.ChatCompletionContentPartImageParam{
				ImageURL: openai.ChatCompletionContentPartImageImageURLParam{
					URL: dataURL,
				},
			},
		})
	}

	return a.callWithSdk(ctx, parts, apiKey)
}

func (t *ImageAnalyzeTool) callWithSdk(
	ctx context.Context,
	parts []openai.ChatCompletionContentPartUnionParam,
	apiKey string,
) string {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	endpoint := t.resolveEndpoint()
	if strings.TrimSpace(endpoint) != "" {
		opts = append(opts, option.WithBaseURL(endpoint))
	}

	client := openai.NewClient(opts...)

	reqCtx := ctx
	if t.config.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, time.Duration(t.config.TimeoutSeconds)*time.Second)
		defer cancel()
	}
	userMessage := openai.ChatCompletionMessageParamUnion{
		OfUser: &openai.ChatCompletionUserMessageParam{
			Content: openai.ChatCompletionUserMessageParamContentUnion{
				OfArrayOfContentParts: parts,
			},
		},
	}

	completion, err := client.Chat.Completions.New(reqCtx, openai.ChatCompletionNewParams{
		Model:     t.config.Model,
		Messages:  []openai.ChatCompletionMessageParamUnion{userMessage},
		MaxTokens: openai.Int(1024),
	})

	if err != nil {
		if errors.Is(reqCtx.Err(), context.DeadlineExceeded) {
			return "Error: Vision API request timed out."
		}
		return fmt.Sprintf("Error: Vision API request failed — %v", err)
	}

	if len(completion.Choices) == 0 {
		return ""
	}

	text := strings.TrimSpace(completion.Choices[0].Message.Content)
	return util.Truncate(text, t.config.MaxOutputChars)
}

func (a *ImageAnalyzeTool) resolveKey() string {
	if a.config.ApiKey != "" {
		return core.SecretResolverInstance.Resolve(a.config.ApiKey)
	}

	return os.Getenv("VISION_API_KEY")
}

func (a *ImageAnalyzeTool) resolveEndpoint() string {
	if a.config.Endpoint != "" {
		return a.config.Endpoint
	}

	switch strings.ToLower(a.config.Provider) {
	case "ollama":
		return "http://localhost:11434/v1"
	default:
		return "https://api.openai.com/v1"
	}
}

func inferMimeType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}
