package core

import "strings"

var LocalModelPackageDefinitionPackages []LocalModelPackageDefinition = []LocalModelPackageDefinition{
	{
		Id:                        "gemma-local-small-q4",
		PresetId:                  "embedded-gemma-small-q4",
		DisplayName:               "Gemma 3 4B IT QAT Q4",
		Description:               "Instruction-tuned Gemma GGUF package for OpenClaw embedded local mode.",
		Provider:                  "embedded",
		ModelId:                   "gemma-local-small-q4",
		Family:                    "gemma",
		Format:                    "gguf",
		Quantization:              "Q4_0",
		FileName:                  "gemma-3-4b-it-q4_0.gguf",
		DownloadUrl:               "https://huggingface.co/google/gemma-3-4b-it-qat-q4_0-gguf/resolve/main/gemma-3-4b-it-q4_0.gguf",
		ModelPageUrl:              "https://huggingface.co/google/gemma-3-4b-it-qat-q4_0-gguf",
		LicenseUrl:                "https://ai.google.dev/gemma/terms",
		RequiresLicenseAcceptance: true,
		RequiresDownloadToken:     true,
		MinRamGb:                  8,
		RecommendedRamGb:          16,
		ContextWindow:             4096,
		MaxOutputTokens:           1024,
		Tags:                      []string{"local", "private", "offline", "cheap"},
		Capabilities: ModelCapabilities{
			SupportsTools:             false,
			SupportsVision:            false,
			SupportsJsonSchema:        false,
			SupportsStructuredOutputs: false,
			SupportsStreaming:         true,
			SupportsParallelToolCalls: false,
			SupportsReasoningEffort:   false,
			SupportsSystemMessages:    true,
			SupportsImageInput:        false,
			SupportsAudioInput:        false,
			MaxContextTokens:          4096,
			MaxOutputTokens:           1024,
		},
		Runtime: LocalModelRuntimeDefaults{
			Backend:     "llama.cpp",
			Threads:     "auto",
			GpuLayers:   "auto",
			ContextSize: 4096,
		},
	},

	{
		Id:                        "gemma-4-e2b",
		PresetId:                  "embedded-gemma-4-e2b",
		DisplayName:               "Gemma 4 E2B Q8",
		Description:               "Gemma 4 E2B instruction-tuned GGUF package for ultra-mobile/edge multimodal local inference.",
		Provider:                  "embedded",
		ModelId:                   "gemma-4-e2b",
		Family:                    "gemma",
		Format:                    "gguf",
		Quantization:              "Q8_0",
		FileName:                  "gemma-4-E2B-it-Q8_0.gguf",
		DownloadUrl:               "https://huggingface.co/ggml-org/gemma-4-E2B-it-GGUF/resolve/main/gemma-4-E2B-it-Q8_0.gguf",
		ExpectedSha256:            "e049411c01fb7a81161768c52e38828970e55a64e22738957adcbe51d20f1c8e",
		ModelPageUrl:              "https://huggingface.co/ggml-org/gemma-4-E2B-it-GGUF",
		LicenseUrl:                "https://ai.google.dev/gemma/terms",
		RequiresLicenseAcceptance: true,
		MinRamGb:                  4,
		RecommendedRamGb:          8,
		ContextWindow:             128000,
		MaxOutputTokens:           4096,
		Tags:                      []string{"local", "private", "offline", "cheap", "gemma4"},
		Capabilities: ModelCapabilities{
			SupportsTools:             true,
			SupportsVision:            true,
			SupportsJsonSchema:        false,
			SupportsStructuredOutputs: false,
			SupportsStreaming:         true,
			SupportsParallelToolCalls: true,
			SupportsReasoningEffort:   true,
			SupportsSystemMessages:    true,
			SupportsImageInput:        true,
			SupportsVideoInput:        true,
			SupportsAudioInput:        true,
			MaxContextTokens:          128000,
			MaxOutputTokens:           4096,
		},
		Files: []LocalModelPackageFileDefinition{
			{
				Role:             "model",
				FileName:         "gemma-4-E2B-it-Q8_0.gguf",
				DownloadUrl:      "https://huggingface.co/ggml-org/gemma-4-E2B-it-GGUF/resolve/main/gemma-4-E2B-it-Q8_0.gguf",
				ExpectedSha256:   "e049411c01fb7a81161768c52e38828970e55a64e22738957adcbe51d20f1c8e",
				Required:         true,
				InstallByDefault: true,
			},

			{
				Role:             "mmproj",
				FileName:         "mmproj-gemma-4-E2B-it-Q8_0.gguf",
				DownloadUrl:      "https://huggingface.co/ggml-org/gemma-4-E2B-it-GGUF/resolve/main/mmproj-gemma-4-E2B-it-Q8_0.gguf",
				ExpectedSha256:   "8a82e0fd831bb7cb5c8898b86393eb14042986b950a60e1034bf21d061aac8a8",
				Required:         true,
				InstallByDefault: true,
			},
		},
		Runtime: LocalModelRuntimeDefaults{
			Backend:                     "llama.cpp",
			Threads:                     "auto",
			GpuLayers:                   "auto",
			ContextSize:                 128000,
			EnableJinja:                 true,
			ChatTemplate:                "gemma",
			MultimodalProjectorFileName: "mmproj-gemma-4-E2B-it-Q8_0.gguf",
			ReasoningMode:               "auto",
		},
	},

	{
		Id:               "gemma-4-litert-e2b",
		PresetId:         "embedded-gemma-4-litert-e2b",
		DisplayName:      "Gemma 4 E2B LiteRT",
		Description:      "Experimental Gemma 4 E2B LiteRT-LM package for edge adapters.",
		Provider:         "embedded",
		ModelId:          "gemma-4-litert-e2b",
		Family:           "gemma",
		Format:           "litertlm",
		Quantization:     "int4",
		FileName:         "gemma-4-E2B-it.litertlm",
		DownloadUrl:      "https://huggingface.co/litert-community/gemma-4-E2B-it-litert-lm/resolve/main/gemma-4-E2B-it.litertlm",
		ExpectedSha256:   "181938105e0eefd105961417e8da75903eacda102c4fce9ce90f50b97139a63c",
		ModelPageUrl:     "https://huggingface.co/litert-community/gemma-4-E2B-it-litert-lm",
		LicenseUrl:       "https://www.apache.org/licenses/LICENSE-2.0",
		Experimental:     true,
		MinRamGb:         4,
		RecommendedRamGb: 8,
		ContextWindow:    32768,
		MaxOutputTokens:  4096,
		Tags:             []string{"local", "private", "offline", "edge", "gemma4", "litert", "experimental"},
		Capabilities: ModelCapabilities{
			SupportsTools:             false,
			SupportsVision:            false,
			SupportsJsonSchema:        false,
			SupportsStructuredOutputs: false,
			SupportsStreaming:         true,
			SupportsParallelToolCalls: false,
			SupportsReasoningEffort:   false,
			SupportsSystemMessages:    true,
			SupportsImageInput:        false,
			SupportsVideoInput:        false,
			SupportsAudioInput:        false,
			MaxContextTokens:          32768,
			MaxOutputTokens:           4096,
		},
		Files: []LocalModelPackageFileDefinition{

			{
				Role:             "model",
				FileName:         "gemma-4-E2B-it.litertlm",
				DownloadUrl:      "https://huggingface.co/litert-community/gemma-4-E2B-it-litert-lm/resolve/main/gemma-4-E2B-it.litertlm",
				ExpectedSha256:   "181938105e0eefd105961417e8da75903eacda102c4fce9ce90f50b97139a63c",
				Required:         true,
				InstallByDefault: true,
			},
		},
		Runtime: LocalModelRuntimeDefaults{
			Backend:      "litert",
			Threads:      "auto",
			GpuLayers:    "auto",
			ContextSize:  32768,
			EnableJinja:  false,
			ChatTemplate: "gemma",
		},
	},

	{
		Id:                        "gemma-4-e4b",
		PresetId:                  "embedded-gemma-4-e4b",
		DisplayName:               "Gemma 4 E4B Q4_K_M",
		Description:               "Gemma 4 E4B instruction-tuned GGUF package for mobile/edge multimodal local inference.",
		Provider:                  "embedded",
		ModelId:                   "gemma-4-e4b",
		Family:                    "gemma",
		Format:                    "gguf",
		Quantization:              "Q4_K_M",
		FileName:                  "gemma-4-E4B-it-Q4_K_M.gguf",
		DownloadUrl:               "https://huggingface.co/ggml-org/gemma-4-E4B-it-GGUF/resolve/main/gemma-4-E4B-it-Q4_K_M.gguf",
		ExpectedSha256:            "90ce98129eb3e8cc57e62433d500c97c624b1e3af1fcc85dd3b55ad7e0313e9f",
		ModelPageUrl:              "https://huggingface.co/ggml-org/gemma-4-E4B-it-GGUF",
		LicenseUrl:                "https://ai.google.dev/gemma/terms",
		RequiresLicenseAcceptance: true,
		MinRamGb:                  6,
		RecommendedRamGb:          16,
		ContextWindow:             128000,
		MaxOutputTokens:           4096,
		Tags:                      []string{"local", "private", "offline", "cheap", "gemma4"},
		Capabilities: ModelCapabilities{
			SupportsTools:             true,
			SupportsVision:            true,
			SupportsJsonSchema:        false,
			SupportsStructuredOutputs: false,
			SupportsStreaming:         true,
			SupportsParallelToolCalls: true,
			SupportsReasoningEffort:   true,
			SupportsSystemMessages:    true,
			SupportsImageInput:        true,
			SupportsVideoInput:        true,
			SupportsAudioInput:        true,
			MaxContextTokens:          128000,
			MaxOutputTokens:           4096,
		},
		Files: []LocalModelPackageFileDefinition{

			{
				Role:             "model",
				FileName:         "gemma-4-E4B-it-Q4_K_M.gguf",
				DownloadUrl:      "https://huggingface.co/ggml-org/gemma-4-E4B-it-GGUF/resolve/main/gemma-4-E4B-it-Q4_K_M.gguf",
				ExpectedSha256:   "90ce98129eb3e8cc57e62433d500c97c624b1e3af1fcc85dd3b55ad7e0313e9f",
				Required:         true,
				InstallByDefault: true,
			},

			{
				Role:             "mmproj",
				FileName:         "mmproj-gemma-4-E4B-it-Q8_0.gguf",
				DownloadUrl:      "https://huggingface.co/ggml-org/gemma-4-E4B-it-GGUF/resolve/main/mmproj-gemma-4-E4B-it-Q8_0.gguf",
				ExpectedSha256:   "51d4b7fd825e4569f746b200fccc5332bf914e8ef7cbe447272ce4fec6df3db6",
				Required:         true,
				InstallByDefault: true,
			},
		},
		Runtime: LocalModelRuntimeDefaults{
			Backend:                     "llama.cpp",
			Threads:                     "auto",
			GpuLayers:                   "auto",
			ContextSize:                 128000,
			EnableJinja:                 true,
			ChatTemplate:                "gemma",
			MultimodalProjectorFileName: "mmproj-gemma-4-E4B-it-Q8_0.gguf",
			ReasoningMode:               "auto",
		},
	},

	{
		Id:                        "gemma-4-31b",
		PresetId:                  "embedded-gemma-4-31b",
		DisplayName:               "Gemma 4 31B Dense Q4_K_M",
		Description:               "Gemma 4 31B dense instruction-tuned GGUF package for workstation/server local inference.",
		Provider:                  "embedded",
		ModelId:                   "gemma-4-31b",
		Family:                    "gemma",
		Format:                    "gguf",
		Quantization:              "Q4_K_M",
		FileName:                  "gemma-4-31B-it-Q4_K_M.gguf",
		DownloadUrl:               "https://huggingface.co/ggml-org/gemma-4-31B-it-GGUF/resolve/main/gemma-4-31B-it-Q4_K_M.gguf",
		ExpectedSha256:            "4f369f8fe0e1bedc5caee9abb89316887f548f80f3035398a5d222a737e699e6",
		ModelPageUrl:              "https://huggingface.co/ggml-org/gemma-4-31B-it-GGUF",
		LicenseUrl:                "https://ai.google.dev/gemma/terms",
		RequiresLicenseAcceptance: true,
		MinRamGb:                  20,
		RecommendedRamGb:          32,
		ContextWindow:             256000,
		MaxOutputTokens:           4096,
		Tags:                      []string{"local", "private", "offline", "gemma4"},
		Capabilities: ModelCapabilities{
			SupportsTools:             true,
			SupportsVision:            true,
			SupportsJsonSchema:        false,
			SupportsStructuredOutputs: false,
			SupportsStreaming:         true,
			SupportsParallelToolCalls: true,
			SupportsReasoningEffort:   true,
			SupportsSystemMessages:    true,
			SupportsImageInput:        true,
			SupportsVideoInput:        true,
			SupportsAudioInput:        false,
			MaxContextTokens:          256000,
			MaxOutputTokens:           4096,
		},
		Files: []LocalModelPackageFileDefinition{

			{
				Role:             "model",
				FileName:         "gemma-4-31B-it-Q4_K_M.gguf",
				DownloadUrl:      "https://huggingface.co/ggml-org/gemma-4-31B-it-GGUF/resolve/main/gemma-4-31B-it-Q4_K_M.gguf",
				ExpectedSha256:   "4f369f8fe0e1bedc5caee9abb89316887f548f80f3035398a5d222a737e699e6",
				Required:         true,
				InstallByDefault: true,
			},

			{
				Role:             "mmproj",
				FileName:         "mmproj-gemma-4-31B-it-Q8_0.gguf",
				DownloadUrl:      "https://huggingface.co/ggml-org/gemma-4-31B-it-GGUF/resolve/main/mmproj-gemma-4-31B-it-Q8_0.gguf",
				ExpectedSha256:   "1e8de54a30a5d08fa400c8d956a5ef7f8ad5ba51a39b860d1ccb463d7c330c37",
				Required:         true,
				InstallByDefault: true,
			},
		},
		Runtime: LocalModelRuntimeDefaults{
			Backend:                     "llama.cpp",
			Threads:                     "auto",
			GpuLayers:                   "auto",
			ContextSize:                 256000,
			EnableJinja:                 true,
			ChatTemplate:                "gemma",
			MultimodalProjectorFileName: "mmproj-gemma-4-31B-it-Q8_0.gguf",
			ReasoningMode:               "auto",
		},
	},

	{
		Id:                        "gemma-4-26b-a4b",
		PresetId:                  "embedded-gemma-4-26b-a4b",
		DisplayName:               "Gemma 4 26B A4B MoE Q4_K_M",
		Description:               "Gemma 4 26B A4B MoE instruction-tuned GGUF package for efficient advanced local inference.",
		Provider:                  "embedded",
		ModelId:                   "gemma-4-26b-a4b",
		Family:                    "gemma",
		Format:                    "gguf",
		Quantization:              "Q4_K_M",
		FileName:                  "gemma-4-26B-A4B-it-Q4_K_M.gguf",
		DownloadUrl:               "https://huggingface.co/ggml-org/gemma-4-26B-A4B-it-GGUF/resolve/main/gemma-4-26B-A4B-it-Q4_K_M.gguf",
		ExpectedSha256:            "88f4a13b0bb95f031a7fad973e10854122fb67ebc34d214d39a2f65053046abc",
		ModelPageUrl:              "https://huggingface.co/ggml-org/gemma-4-26B-A4B-it-GGUF",
		LicenseUrl:                "https://ai.google.dev/gemma/terms",
		RequiresLicenseAcceptance: true,
		MinRamGb:                  18,
		RecommendedRamGb:          24,
		ContextWindow:             256000,
		MaxOutputTokens:           4096,
		Tags:                      []string{"local", "private", "offline", "moe", "gemma4"},
		Capabilities: ModelCapabilities{
			SupportsTools:             true,
			SupportsVision:            true,
			SupportsJsonSchema:        false,
			SupportsStructuredOutputs: false,
			SupportsStreaming:         true,
			SupportsParallelToolCalls: true,
			SupportsReasoningEffort:   true,
			SupportsSystemMessages:    true,
			SupportsImageInput:        true,
			SupportsVideoInput:        true,
			SupportsAudioInput:        false,
			MaxContextTokens:          256000,
			MaxOutputTokens:           4096,
		},
		Files: []LocalModelPackageFileDefinition{
			{
				Role:             "model",
				FileName:         "gemma-4-26B-A4B-it-Q4_K_M.gguf",
				DownloadUrl:      "https://huggingface.co/ggml-org/gemma-4-26B-A4B-it-GGUF/resolve/main/gemma-4-26B-A4B-it-Q4_K_M.gguf",
				ExpectedSha256:   "88f4a13b0bb95f031a7fad973e10854122fb67ebc34d214d39a2f65053046abc",
				Required:         true,
				InstallByDefault: true,
			},

			{
				Role:             "mmproj",
				FileName:         "mmproj-gemma-4-26B-A4B-it-Q8_0.gguf",
				DownloadUrl:      "https://huggingface.co/ggml-org/gemma-4-26B-A4B-it-GGUF/resolve/main/mmproj-gemma-4-26B-A4B-it-Q8_0.gguf",
				ExpectedSha256:   "1f2339eb6497bd69fde3c68e1592cd472f1ce176dfefe6e6d156d5a55719705e",
				Required:         true,
				InstallByDefault: true,
			},
		},
		Runtime: LocalModelRuntimeDefaults{
			Backend:                     "llama.cpp",
			Threads:                     "auto",
			GpuLayers:                   "auto",
			ContextSize:                 256000,
			EnableJinja:                 true,
			ChatTemplate:                "gemma",
			MultimodalProjectorFileName: "mmproj-gemma-4-26B-A4B-it-Q8_0.gguf",
			ReasoningMode:               "auto",
		},
	},
}

var LocalModelPresetDefinitionPackages []LocalModelPresetDefinition = []LocalModelPresetDefinition{
	{
		Id:             "embedded-gemma-small-q4",
		Label:          "Embedded Gemma Small Q4",
		Description:    "OpenClaw-managed local Gemma profile for private/offline helper tasks.",
		Provider:       "embedded",
		DefaultBaseUrl: "",
		PackageId:      "gemma-local-small-q4",
		ModelId:        "gemma-local-small-q4",
		Installable:    true,
		Tags:           []string{"local", "private", "offline", "cheap"},
		Capabilities: ModelCapabilities{
			SupportsTools:             false,
			SupportsVision:            false,
			SupportsJsonSchema:        false,
			SupportsStructuredOutputs: false,
			SupportsStreaming:         true,
			SupportsParallelToolCalls: false,
			SupportsReasoningEffort:   false,
			SupportsSystemMessages:    true,
			SupportsImageInput:        false,
			SupportsAudioInput:        false,
			MaxContextTokens:          4096,
			MaxOutputTokens:           1024,
		},
		RecommendedContextTokens: 4096,
		RecommendedOutputTokens:  1024,
		CompatibilityNotes: []string{
			"Requires a verified local GGUF model package and a llama.cpp llama-server runtime.",
			"Use a fallback profile for tool-heavy, structured-output, vision, or long-context routes.",
		},
		DoctorExpectations: []string{
			"Warn when the package is not installed or cannot be verified.",
			"Warn when routes require tool calling, structured outputs, vision, or larger context than the embedded profile advertises.",
		},
	},
	{
		Id:          "ollama-general",
		Label:       "Ollama General",
		Description: "Balanced local preset for everyday chat and mixed tasks.",
		Tags:        []string{"local", "private", "generalist"},
		Capabilities: ModelCapabilities{
			SupportsTools:             false,
			SupportsVision:            false,
			SupportsJsonSchema:        false,
			SupportsStructuredOutputs: false,
			SupportsStreaming:         true,
			SupportsParallelToolCalls: false,
			SupportsReasoningEffort:   false,
			SupportsSystemMessages:    true,
			SupportsImageInput:        false,
			SupportsAudioInput:        false,
			MaxContextTokens:          32768,
			MaxOutputTokens:           4096,
		},
		RecommendedContextTokens: 32768,
		RecommendedOutputTokens:  4096,
		CompatibilityNotes: []string{
			"Use native Ollama endpoints instead of the OpenAI-compatible /v1 shim.",
			"Add a fallback profile for tool-heavy routes.",
		},
		DoctorExpectations: []string{
			"Warn when this preset is selected for routes that require tools or structured outputs.",
			"Warn when recent prompt usage routinely approaches the preset context limit.",
		},
	},
	{
		Id:          "ollama-agentic",
		Label:       "Ollama Agentic",
		Description: "Local-first preset for tool calling with deterministic cloud fallback.",
		Tags:        []string{"local", "private", "agentic"},
		Capabilities: ModelCapabilities{
			SupportsTools:             true,
			SupportsVision:            false,
			SupportsJsonSchema:        false,
			SupportsStructuredOutputs: false,
			SupportsStreaming:         true,
			SupportsParallelToolCalls: false,
			SupportsReasoningEffort:   false,
			SupportsSystemMessages:    true,
			SupportsImageInput:        false,
			SupportsAudioInput:        false,
			MaxContextTokens:          65536,
			MaxOutputTokens:           8192,
		},
		RecommendedContextTokens: 65536,
		RecommendedOutputTokens:  8192,
		CompatibilityNotes: []string{
			"Best paired with a stronger fallback profile for structured outputs and long-context repairs.",
			"Recent prompt budget drift should be monitored more aggressively for this preset.",
		},
		DoctorExpectations: []string{
			"Warn when a route requires JSON schema or parallel tool calling.",
			"Warn when no fallback profile is configured for tool-heavy routes.",
		},
	},
	{
		Id:          "ollama-vision",
		Label:       "Ollama Vision",
		Description: "Local preset optimized for image-aware interactions with conservative tool expectations.",
		Tags:        []string{"local", "private", "vision"},
		Capabilities: ModelCapabilities{
			SupportsTools:             false,
			SupportsVision:            true,
			SupportsJsonSchema:        false,
			SupportsStructuredOutputs: false,
			SupportsStreaming:         true,
			SupportsParallelToolCalls: false,
			SupportsReasoningEffort:   false,
			SupportsSystemMessages:    true,
			SupportsImageInput:        true,
			SupportsAudioInput:        false,
			MaxContextTokens:          65536,
			MaxOutputTokens:           8192,
		},
		RecommendedContextTokens: 65536,
		RecommendedOutputTokens:  8192,
		CompatibilityNotes: []string{
			"Prefer explicit fallback for structured extraction and multi-tool routes.",
			"Large inline images can exhaust prompt budget quickly.",
		},
		DoctorExpectations: []string{
			"Warn when image-heavy recent turns exceed expected context headroom.",
			"Warn when vision is enabled but the configured local model appears text-only.",
		},
	},
}

func TryGetLocalModelPackage(s string) (*LocalModelPackageDefinition, bool) {
	for _, p := range LocalModelPackageDefinitionPackages {
		if p.Id == s || p.PresetId == s || p.ModelId == s {
			return &p, true
		}
	}

	return nil, false
}

func TryGetLocalModelPreset(presetId string) (*LocalModelPresetDefinition, bool) {
	for _, p := range LocalModelPresetDefinitionPackages {
		if p.Id == presetId {
			return &p, true
		}
	}

	d, f := TryGetLocalModelPackage(presetId)
	if f && d != nil {
		preset := toPreset(*d)
		return &preset, true
	}

	return nil, false
}

func LocalModelPresetDefinitionList() []LocalModelPresetDefinition {
	result := make([]LocalModelPresetDefinition, 0, len(LocalModelPresetDefinitionPackages)+len(LocalModelPackageDefinitionPackages))
	result = append(result, LocalModelPresetDefinitionPackages...)

	for _, pkg := range LocalModelPackageDefinitionPackages {
		exists := false
		for _, preset := range LocalModelPresetDefinitionPackages {
			if strings.EqualFold(preset.Id, pkg.PresetId) {
				exists = true
				break
			}
		}

		if !exists {
			result = append(result, toPreset(pkg))
		}
	}

	return result
}

func toPreset(pkg LocalModelPackageDefinition) LocalModelPresetDefinition {
	var compatibilityNotes []string

	if strings.EqualFold(pkg.Runtime.Backend, "litert") {
		compatibilityNotes = append(compatibilityNotes, "Experimental: requires a verified LiteRT-LM package and an OpenClaw-compatible LiteRT adapter binary.")
	} else {
		compatibilityNotes = append(compatibilityNotes, "Requires a verified local GGUF model package and a llama.cpp llama-server runtime.")
	}

	if pkg.Capabilities.SupportsTools {
		compatibilityNotes = append(compatibilityNotes, "Tool calling requires llama-server Jinja chat-template support and OpenClaw policy approval.")
	} else {
		compatibilityNotes = append(compatibilityNotes, "Use a fallback profile for tool-heavy or structured-output routes.")
	}

	if pkg.Capabilities.SupportsVision {
		compatibilityNotes = append(compatibilityNotes, "Multimodal input requires the package projector file or OpenClaw:LocalInference:MultimodalProjectorPath.")
	} else {
		compatibilityNotes = append(compatibilityNotes, "This package is text-only.")
	}

	return LocalModelPresetDefinition{
		Id:                       pkg.PresetId,
		Label:                    pkg.DisplayName,
		Description:              pkg.Description,
		Provider:                 pkg.Provider,
		DefaultBaseUrl:           "",
		PackageId:                pkg.Id,
		ModelId:                  pkg.ModelId,
		Installable:              true,
		Tags:                     pkg.Tags,
		Capabilities:             pkg.Capabilities,
		RecommendedContextTokens: pkg.ContextWindow,
		RecommendedOutputTokens:  pkg.MaxOutputTokens,
		CompatibilityNotes:       compatibilityNotes,
		DoctorExpectations: []string{
			"Warn when the package is not installed or cannot be verified.",
			"Warn when routes require capabilities outside the package profile.",
			"Warn when requested context routinely exceeds local RAM guidance.",
		},
	}
}
