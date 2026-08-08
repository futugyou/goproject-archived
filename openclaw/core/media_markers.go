package core

import "strings"

type MediaMarkerKind byte

const (
	MediaMarkerImageUrl MediaMarkerKind = iota
	MediaMarkerImagePath
	MediaMarkerFileUrl
	MediaMarkerFilePath
	MediaMarkerTelegramImageFileId
	MediaMarkerTelegramVideoFileId
	MediaMarkerTelegramAudioFileId
	MediaMarkerTelegramDocumentFileId
	MediaMarkerTelegramStickerFileId
	MediaMarkerVideoUrl
	MediaMarkerAudioUrl
	MediaMarkerDocumentUrl
	MediaMarkerStickerUrl
)

type MediaMarker struct {
	Kind  MediaMarkerKind
	Value string
}

func MediaMarkerExtract(text string) ([]MediaMarker, string) {
	if text == "" {
		return []MediaMarker{}, ""
	}

	var markers []MediaMarker
	var remainingLines []string

	normalizedText := strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.SplitSeq(normalizedText, "\n")

	for line := range lines {
		trimmed := strings.TrimSpace(line)
		if marker, ok := TryParseMarker(trimmed); ok {
			markers = append(markers, marker)
			continue
		}
		remainingLines = append(remainingLines, line)
	}

	remaining := strings.TrimSpace(strings.Join(remainingLines, "\n"))
	return markers, remaining
}

func TryParseMarker(line string) (MediaMarker, bool) {
	if strings.TrimSpace(line) == "" {
		return MediaMarker{}, false
	}

	prefixes := []struct {
		prefix string
		kind   MediaMarkerKind
	}{
		{"IMAGE_URL:", MediaMarkerImageUrl},
		{"IMAGE_PATH:", MediaMarkerImagePath},
		{"FILE_URL:", MediaMarkerFileUrl},
		{"FILE_PATH:", MediaMarkerFilePath},
		{"VIDEO_URL:", MediaMarkerVideoUrl},
		{"AUDIO_URL:", MediaMarkerAudioUrl},
		{"DOCUMENT_URL:", MediaMarkerDocumentUrl},
		{"STICKER_URL:", MediaMarkerStickerUrl},
	}

	for _, p := range prefixes {
		if val, ok := tryParseBracketValue(line, p.prefix); ok {
			return MediaMarker{Kind: p.kind, Value: val}, true
		}
	}

	tgTypes := []struct {
		mediaType string
		kind      MediaMarkerKind
	}{
		{"IMAGE", MediaMarkerTelegramImageFileId},
		{"VIDEO", MediaMarkerTelegramVideoFileId},
		{"AUDIO", MediaMarkerTelegramAudioFileId},
		{"DOCUMENT", MediaMarkerTelegramDocumentFileId},
		{"STICKER", MediaMarkerTelegramStickerFileId},
	}

	for _, tg := range tgTypes {
		if val, ok := tryParseTelegramFileId(line, tg.mediaType); ok {
			return MediaMarker{Kind: tg.kind, Value: val}, true
		}
	}

	return MediaMarker{}, false
}

func tryParseBracketValue(line, prefix string) (string, bool) {
	if len(line) < len(prefix)+2 {
		return "", false
	}

	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
		return "", false
	}

	inner := line[1 : len(line)-1]
	if !strings.HasPrefix(inner, prefix) {
		return "", false
	}

	value := strings.TrimSpace(inner[len(prefix):])
	if value == "" {
		return "", false
	}

	return value, true
}

func tryParseTelegramFileId(line, mediaType string) (string, bool) {
	prefix := "[" + mediaType + ":telegram:file_id="
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, "]") {
		return "", false
	}

	value := strings.TrimSpace(line[len(prefix) : len(line)-1])
	if value == "" {
		return "", false
	}

	return value, true
}
