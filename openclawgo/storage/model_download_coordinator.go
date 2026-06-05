package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
)

type ModelDownloadResult struct {
	Success       bool
	FinalPath     string
	FailureReason string
}

type ModelDownloadCoordinator struct {
	verifier IModelDownloadVerifier
	quota    IModelStorageQuota
}

func NewModelDownloadCoordinator(verifier IModelDownloadVerifier, quota IModelStorageQuota) *ModelDownloadCoordinator {
	return &ModelDownloadCoordinator{
		verifier: verifier,
		quota:    quota,
	}
}

func (m *ModelDownloadCoordinator) safeDelete(path string) {
	_, err := os.Stat(path)
	if err == nil || !os.IsNotExist(err) {
		os.Remove(path)
	}
}

func (m *ModelDownloadCoordinator) Download(ctx context.Context, fileName string, sourceStream io.Reader, expectedSha256Hex string, expectedBytes int64) (*ModelDownloadResult, error) {
	if sourceStream == nil {
		return nil, errors.New("stream can not be nil")
	}

	finalPath, err := ResolveSafeModelPath(fileName)
	if err != nil {
		return nil, err
	}
	var modelsRoot = filepath.Dir(finalPath)
	var tempPath = finalPath + ".tmp"

	quotaResult, err := m.quota.Check(ctx, modelsRoot, expectedBytes)
	if err != nil {
		m.safeDelete(tempPath)
		return nil, err
	}
	if !quotaResult.Allowed {
		return &ModelDownloadResult{Success: false, FailureReason: quotaResult.DenyReason}, nil
	}

	_, err = copyToWithContext(ctx, tempPath, sourceStream, 64*1024)
	if err != nil {
		m.safeDelete(tempPath)
		return &ModelDownloadResult{Success: false, FailureReason: err.Error()}, nil
	}

	reader, file, err := openFileForRead(tempPath)
	if err != nil {
		m.safeDelete(tempPath)
		return &ModelDownloadResult{Success: false, FailureReason: err.Error()}, nil
	}
	defer file.Close()

	verification := m.verifier.Verify(ctx, reader, expectedSha256Hex, expectedBytes)

	if !verification.IsValid {
		m.safeDelete(tempPath)
		return &ModelDownloadResult{Success: false, FailureReason: verification.FailureReason}, nil
	}

	if err = MoveFile(tempPath, finalPath); err != nil {
		m.safeDelete(tempPath)
		return &ModelDownloadResult{Success: false, FailureReason: verification.FailureReason}, nil
	}

	switch checker := m.quota.(type) {
	case *QuotaChecker:
		checker.InvalidateCache()
	}

	return &ModelDownloadResult{Success: true, FailureReason: "", FinalPath: finalPath}, nil

}
