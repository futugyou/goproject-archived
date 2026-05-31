package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

type ModelDownloadVerificationResult struct {
	IsValid         bool
	ActualSha256Hex string
	ActualBytes     int64
	FailureReason   string
}

type IModelDownloadVerifier interface {
	Verify(ctx context.Context, content io.Reader, expectedSha256Hex string, expectedBytes int64) ModelDownloadVerificationResult
}

const Sha256ModelDownloadVerifierBufferSize = 64 * 1024

var _ IModelDownloadVerifier = (*Sha256ModelDownloadVerifier)(nil)

type Sha256ModelDownloadVerifier struct {
}

// Verify implements [IModelDownloadVerifier].
func (s *Sha256ModelDownloadVerifier) Verify(ctx context.Context, content io.Reader, expectedSHA string, expectedBytes int64) ModelDownloadVerificationResult {
	if content == nil {
		return ModelDownloadVerificationResult{
			FailureReason: "content reader was nil",
		}
	}

	expectedSHA = strings.TrimSpace(expectedSHA)

	if len(expectedSHA) != 64 {
		return ModelDownloadVerificationResult{
			FailureReason: "expected sha256 must be 64 hex chars",
		}
	}

	h := sha256.New()

	buf := make([]byte, Sha256ModelDownloadVerifierBufferSize)

	var total int64

	for {
		n, err := content.Read(buf)

		if n > 0 {
			_, _ = h.Write(buf[:n])
			total += int64(n)
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return ModelDownloadVerificationResult{
				ActualBytes:   total,
				FailureReason: fmt.Sprintf("stream read failure: %T", err),
			}
		}
	}

	actualSHA := hex.EncodeToString(h.Sum(nil))

	if !strings.EqualFold(actualSHA, expectedSHA) {
		return ModelDownloadVerificationResult{
			ActualSha256Hex: actualSHA,
			ActualBytes:     total,
			FailureReason: fmt.Sprintf(
				"sha256 mismatch: expected %s, got %s",
				expectedSHA,
				actualSHA,
			),
		}
	}

	if total != expectedBytes {
		return ModelDownloadVerificationResult{
			ActualSha256Hex: actualSHA,
			ActualBytes:     total,
			FailureReason: fmt.Sprintf(
				"byte count mismatch: expected %d, got %d",
				expectedBytes,
				total,
			),
		}
	}

	return ModelDownloadVerificationResult{
		IsValid:         true,
		ActualSha256Hex: actualSHA,
		ActualBytes:     total,
	}
}
