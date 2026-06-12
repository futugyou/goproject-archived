package core

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
)

type IEvidenceBundleStore interface {
	Save(ctx context.Context, bundle EvidenceBundle) error
	Get(ctx context.Context, id string) (*EvidenceBundle, error)
	List(ctx context.Context, query EvidenceBundleListQuery) ([]EvidenceBundle, error)
	Delete(ctx context.Context, id string) error
}

var _ IEvidenceBundleStore = (*FileEvidenceBundleStore)(nil)

type FileEvidenceBundleStore struct {
	evidencePath       string
	evidencePathPrefix string
}

func NewFileEvidenceBundleStore(storagePath string) (*FileEvidenceBundleStore, error) {
	root, err := filepath.Abs(storagePath)
	if err != nil {
		return nil, err
	}

	evidencePath := filepath.Clean(filepath.Join(root, "harness", "evidence"))
	evidencePathPrefix := evidencePath
	if !strings.HasSuffix(evidencePathPrefix, string(filepath.Separator)) {
		evidencePathPrefix += string(filepath.Separator)
	}

	if err := os.MkdirAll(evidencePath, 0755); err != nil {
		return nil, err
	}

	return &FileEvidenceBundleStore{
		evidencePath:       evidencePath,
		evidencePathPrefix: evidencePathPrefix,
	}, nil
}

func (s *FileEvidenceBundleStore) Save(ctx context.Context, bundle EvidenceBundle) error {
	if err := s.ensureSafeId(bundle.ID); err != nil {
		return err
	}

	fileInfo, err := s.fileForId(bundle.ID)
	if err != nil {
		return err
	}

	return s.saveOne(ctx, fileInfo, &bundle)
}

func (s *FileEvidenceBundleStore) Get(ctx context.Context, id string) (*EvidenceBundle, error) {
	if err := s.ensureSafeId(id); err != nil {
		return nil, err
	}

	fileInfo, err := s.fileForId(id)
	if err != nil {
		return nil, err
	}

	return s.loadOne(ctx, fileInfo)
}

func (s *FileEvidenceBundleStore) List(ctx context.Context, query EvidenceBundleListQuery) ([]EvidenceBundle, error) {
	files, err := os.ReadDir(s.evidencePath)
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			return []EvidenceBundle{}, nil
		}
		return []EvidenceBundle{}, nil
	}

	var results []EvidenceBundle

	for _, file := range files {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(s.evidencePath, file.Name())
		bundle, err := s.loadOne(ctx, filePath)
		if err != nil {
			log.Printf("Skipping invalid evidence bundle file '%s': %v", filePath, err)
			continue
		}

		if bundle != nil && s.matches(bundle, &query) {
			results = append(results, *bundle)
		}
	}

	// 排序逻辑：先按 UpdatedAtUtc 降序，再按 CreatedAtUtc 降序
	sort.Slice(results, func(i, j int) bool {
		if results[i].UpdatedAtUtc.Equal(results[j].UpdatedAtUtc) {
			return results[i].CreatedAtUtc.After(results[j].CreatedAtUtc)
		}
		return results[i].UpdatedAtUtc.After(results[j].UpdatedAtUtc)
	})

	// 限制返回条数
	limit := query.Limit
	if limit < 1 {
		limit = 1
	} else if limit > 5000 {
		limit = 5000
	}

	if len(results) > limit {
		results = results[:limit]
	}

	if results == nil {
		return []EvidenceBundle{}, nil
	}
	return results, nil
}

func (s *FileEvidenceBundleStore) Delete(ctx context.Context, id string) error {
	if err := s.ensureSafeId(id); err != nil {
		return err
	}

	filePath, err := s.fileForId(id)
	if err != nil {
		return err
	}

	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (s *FileEvidenceBundleStore) fileForId(id string) (string, error) {
	expectedFileName := s.encodeKey(id) + ".json"
	fileName := filepath.Base(expectedFileName)

	if strings.TrimSpace(fileName) == "" || fileName != expectedFileName {
		return "", errors.New("evidence bundle id resolves to an unsafe file name")
	}

	path, err := filepath.Abs(filepath.Join(s.evidencePath, fileName))
	if err != nil {
		return "", err
	}

	if !strings.HasPrefix(path, s.evidencePathPrefix) {
		return "", errors.New("evidence bundle id resolves outside the evidence store")
	}

	return path, nil
}

func (s *FileEvidenceBundleStore) matches(bundle *EvidenceBundle, query *EvidenceBundleListQuery) bool {
	if !s.isMatch(query.SourceSessionID, bundle.SourceSessionID, false) {
		return false
	}
	if !s.isMatch(query.HarnessContractID, bundle.HarnessContractID, false) {
		return false
	}
	if !s.isMatch(query.LearningProposalID, bundle.LearningProposalID, false) {
		return false
	}
	if !s.isMatch(query.ActorID, bundle.ActorID, false) {
		return false
	}
	if !s.isMatch(query.ChannelID, bundle.ChannelID, false) {
		return false
	}
	if !s.isMatch(query.Confidence, &bundle.Confidence, true) {
		return false
	}

	if query.Tag != nil && strings.TrimSpace(*query.Tag) != "" {
		if bundle.Tags == nil {
			return false
		}

		qTag := strings.TrimSpace(*query.Tag)
		found := false
		for _, tag := range bundle.Tags {
			if strings.EqualFold(tag, qTag) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if query.CreatedFromUtc != nil && bundle.CreatedAtUtc.Before(*query.CreatedFromUtc) {
		return false
	}
	if query.CreatedToUtc != nil && bundle.CreatedAtUtc.After(*query.CreatedToUtc) {
		return false
	}

	return true
}

func (s *FileEvidenceBundleStore) isMatch(queryStr, bundleStr *string, caseInsensitive bool) bool {
	// 1. 如果 query 没传这个过滤条件（为 nil 或全是空格），视为“不校验该字段”，直接放行
	if queryStr == nil || strings.TrimSpace(*queryStr) == "" {
		return true
	}
	// 2. 如果 query 传了有效值，但 bundle 却是 nil，说明不匹配
	if bundleStr == nil {
		return false
	}
	// 3. 两个都有值，安全地解引用并比较
	qVal := strings.TrimSpace(*queryStr)
	bVal := strings.TrimSpace(*bundleStr) // 顺便帮 bundle 也做个 Trim，更健壮

	if caseInsensitive {
		return strings.EqualFold(bVal, qVal)
	}
	return bVal == qVal
}

func (s *FileEvidenceBundleStore) loadOne(ctx context.Context, filePath string) (*EvidenceBundle, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) || os.IsPermission(err) {
			return nil, nil
		}
		return nil, nil // 仿照 C# 吞掉特定的 IO 异常返回 default
	}
	defer file.Close()

	// Go 标准库没有原生支持把 context 传给 json.Decoder，通过自定义 Reader 监听取消信号
	type contextReader struct {
		ctx context.Context
		r   io.Reader
	}
	cr := &contextReader{ctx: ctx, r: file}
	readFn := func(p []byte) (int, error) {
		if err := cr.ctx.Err(); err != nil {
			return 0, err
		}
		return cr.r.Read(p)
	}

	var bundle EvidenceBundle
	// 这里通过自定义的 readFn 间接模拟了针对 Context 的读取取消
	decoder := json.NewDecoder(interfaceReader{readFn})
	if err := decoder.Decode(&bundle); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, nil
	}

	return &bundle, nil
}

func (s *FileEvidenceBundleStore) saveOne(ctx context.Context, filePath string, bundle *EvidenceBundle) error {
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	u, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	tempPath := fmt.Sprintf("%s.%s.tmp", filePath, hex.EncodeToString(u[:]))

	defer func() {
		_ = os.Remove(tempPath)
	}()

	tempFile, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0644)
	if err != nil {
		return err
	}

	// 序列化并写入
	err = json.NewEncoder(tempFile).Encode(bundle)
	_ = tempFile.Close() // 必须在 Rename 前关闭文件指针
	if err != nil {
		return err
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	// 原子替换
	return os.Rename(tempPath, filePath)
}

func (s *FileEvidenceBundleStore) ensureSafeId(id string) error {
	if strings.TrimSpace(id) == "" {
		return errors.New("evidence bundle id is required")
	}
	if len(id) > 128 {
		return errors.New("evidence bundle id is too long")
	}

	for _, ch := range id {
		isLetterOrDigit := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
		if !isLetterOrDigit && ch != '_' && ch != '-' && ch != '.' {
			return errors.New("evidence bundle id contains unsafe characters")
		}
	}
	return nil
}

func (s *FileEvidenceBundleStore) encodeKey(key string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(key))
	encoded = strings.ReplaceAll(encoded, "+", "-")
	encoded = strings.ReplaceAll(encoded, "/", "_")
	return strings.TrimRight(encoded, "=")
}

// 辅助结构体，用于把普通的 func 转换成 io.Reader
type interfaceReader struct {
	readFn func(p []byte) (int, error)
}

func (i interfaceReader) Read(p []byte) (int, error) {
	return i.readFn(p)
}
