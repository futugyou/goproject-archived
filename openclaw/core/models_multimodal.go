package core

import "time"

const (
	AudioTranscriptionConfig_MaxAudioBytes   = 25 * 1024 * 1024
	AudioTranscriptionConfig_TimeoutSeconds  = 30
	VideoProcessingConfig_MaxVideoBytes      = 100 * 1024 * 1024
	VideoProcessingConfig_MaxDurationSeconds = 120
	VideoProcessingConfig_MaxFrames          = 8
	VideoProcessingConfig_FrameWidth         = 768
)

type MultimodalConfig struct {
	Enabled        bool                     `json:"enabled"`
	MediaCachePath string                   `json:"media_cache_path"`
	LiveProvider   string                   `json:"live_provider"`
	VisionProvider string                   `json:"vision_provider"`
	VisionModel    string                   `json:"vision_model"`
	Transcription  AudioTranscriptionConfig `json:"transcription"`
	Video          VideoProcessingConfig    `json:"video"`
	TextToSpeech   TextToSpeechConfig       `json:"text_to_speech"`
	GeminiLive     GeminiLiveConfig         `json:"gemini_live"`
	ElevenLabs     ElevenLabsConfig         `json:"eleven_labs"`
}

func NewMultimodalConfig() *MultimodalConfig {
	return &MultimodalConfig{
		Enabled:        true,
		MediaCachePath: "./memory/media-cache",
		LiveProvider:   "gemini",
		VisionProvider: "gemini",
		VisionModel:    "gemini-2.5-flash",
		Transcription:  *NewAudioTranscriptionConfig(),
		Video:          *NewVideoProcessingConfig(),
		TextToSpeech:   *NewTextToSpeechConfig(),
		GeminiLive:     *NewGeminiLiveConfig(),
		ElevenLabs:     *NewElevenLabsConfig(),
	}
}

type AudioTranscriptionConfig struct {
	Enabled           bool   `json:"enabled"`
	Provider          string `json:"provider"`
	Model             string `json:"model"`
	MaxAudioBytes     int    `json:"max_audio_bytes"`
	TimeoutSeconds    int    `json:"timeout_seconds"`
	InjectAudioMarker bool   `json:"inject_audio_marker"`
	FailureMode       string `json:"failure_mode"`
}

func NewAudioTranscriptionConfig() *AudioTranscriptionConfig {
	return &AudioTranscriptionConfig{
		Enabled:           true,
		Provider:          "gemini",
		Model:             "gemini-2.5-flash",
		MaxAudioBytes:     AudioTranscriptionConfig_MaxAudioBytes,
		TimeoutSeconds:    AudioTranscriptionConfig_TimeoutSeconds,
		InjectAudioMarker: true,
		FailureMode:       "degrade",
	}
}

type VideoProcessingConfig struct {
	Enabled                bool    `json:"enabled"`
	FfmpegPath             string  `json:"ffmpeg_path"`
	FfprobePath            string  `json:"ffprobe_path"`
	MaxVideoBytes          int     `json:"max_video_bytes"`
	MaxDurationSeconds     int     `json:"max_duration_seconds"`
	MaxFrames              int     `json:"max_frames"`
	FrameIntervalSeconds   float64 `json:"frame_interval_seconds"`
	FrameWidth             int     `json:"frame_width"`
	ExtractAudioTranscript bool    `json:"extract_audio_transcript"`
	FailureMode            string  `json:"failure_mode"`
}

func NewVideoProcessingConfig() *VideoProcessingConfig {
	return &VideoProcessingConfig{
		Enabled:                true,
		FfmpegPath:             "ffmpeg",
		FfprobePath:            "ffprobe",
		MaxVideoBytes:          VideoProcessingConfig_MaxVideoBytes,
		MaxDurationSeconds:     VideoProcessingConfig_MaxDurationSeconds,
		MaxFrames:              VideoProcessingConfig_MaxFrames,
		FrameIntervalSeconds:   5.0,
		FrameWidth:             VideoProcessingConfig_FrameWidth,
		ExtractAudioTranscript: false,
		FailureMode:            "degrade",
	}
}

type TextToSpeechConfig struct {
	Enabled   bool   `json:"enabled"`
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	VoiceName string `json:"voice_name"`
	VoiceID   string `json:"voice_id"`
}

func NewTextToSpeechConfig() *TextToSpeechConfig {
	return &TextToSpeechConfig{
		Enabled:   true,
		Provider:  "gemini",
		Model:     "gemini-2.5-flash-preview-tts",
		VoiceName: "Kore",
	}
}

type GeminiLiveConfig struct {
	Enabled             bool     `json:"enabled"`
	Model               string   `json:"model"`
	Endpoint            string   `json:"endpoint"`
	ResponseModalities  []string `json:"response_modalities"`
	VoiceName           string   `json:"voice_name"`
	InputTranscription  bool     `json:"input_transcription"`
	OutputTranscription bool     `json:"output_transcription"`
}

func NewGeminiLiveConfig() *GeminiLiveConfig {
	return &GeminiLiveConfig{
		Enabled:             true,
		Model:               "gemini-2.0-flash-live-001",
		Endpoint:            "wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent",
		ResponseModalities:  []string{"TEXT"},
		InputTranscription:  true,
		OutputTranscription: true,
	}
}

type ElevenLabsConfig struct {
	Enabled      bool   `json:"enabled"`
	Endpoint     string `json:"endpoint"`
	ApiKey       string `json:"api_key"`
	VoiceID      string `json:"voice_id"`
	Model        string `json:"model"`
	OutputFormat string `json:"output_format"`
}

func NewElevenLabsConfig() *ElevenLabsConfig {
	return &ElevenLabsConfig{
		Enabled:      true,
		Endpoint:     "https://api.elevenlabs.io",
		VoiceID:      "JBFqnCBsd6RMkjVDRZzb",
		Model:        "eleven_multilingual_v2",
		OutputFormat: "mp3_44100_128",
	}
}

type StoredMediaAsset struct {
	ID           string    `json:"id"`
	MediaType    string    `json:"media_type"`
	FileName     string    `json:"file_name"`
	Path         string    `json:"path"`
	SizeBytes    int64     `json:"size_bytes"`
	CreatedAtUtc time.Time `json:"created_at_utc"`
}

func NewStoredMediaAsset() *StoredMediaAsset {
	return &StoredMediaAsset{
		CreatedAtUtc: time.Now().UTC(),
	}
}

type LiveSessionOpenRequest struct {
	Provider           string   `json:"provider"`
	Model              string   `json:"model"`
	ResponseModalities []string `json:"response_modalities"`
	SystemInstruction  string   `json:"system_instruction"`
	VoiceName          string   `json:"voice_name"`
}

type LiveSessionOpened struct {
	SessionID          string   `json:"session_id"`
	Provider           string   `json:"provider"`
	Model              string   `json:"model"`
	ResponseModalities []string `json:"response_modalities"`
}

type LiveClientEnvelope struct {
	Type         string `json:"type"`
	Text         string `json:"text"`
	Base64Data   string `json:"base64_data"`
	MimeType     string `json:"mime_type"`
	TurnComplete bool   `json:"turn_complete"`
}

type LiveServerEnvelope struct {
	Type         string `json:"type"`
	Text         string `json:"text"`
	Base64Data   string `json:"base64_data"`
	MimeType     string `json:"mime_type"`
	TurnComplete bool   `json:"turn_complete"`
	Interrupted  bool   `json:"interrupted"`
	Error        string `json:"error"`
}
