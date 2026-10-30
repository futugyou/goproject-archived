package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type MetaRunProposalReviewStore struct {
	reviewsPath string
}

func NewMetaRunProposalReviewStore(memoryPath string) (*MetaRunProposalReviewStore, error) {
	p, err := filepath.Abs(memoryPath)
	if err != nil {
		return nil, err
	}

	m := &MetaRunProposalReviewStore{}
	m.reviewsPath = filepath.Join(p, "meta-run-proposal-reviews")
	os.MkdirAll(m.reviewsPath, 0755)
	return m, nil
}

func encodeKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "item"
	}

	return base64.RawURLEncoding.EncodeToString([]byte(key))
}

func (m *MetaRunProposalReviewStore) getSessionPath(sessionId string) string {
	return filepath.Join(m.reviewsPath, fmt.Sprintf("%s.json", encodeKey(sessionId)))
}

func (m *MetaRunProposalReviewStore) saveSessionRecords(ctx context.Context, sessionId string, records []core.MetaRunProposalReviewRecord) error {
	path := m.getSessionPath(sessionId)
	var tempPath = fmt.Sprintf("%s.tmp", path)
	data, err := json.Marshal(records)
	if err != nil {
		return err
	}

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.Rename(tempPath, path)
}

func (m *MetaRunProposalReviewStore) loadSessionRecords(ctx context.Context, sessionId string) []core.MetaRunProposalReviewRecord {
	path := m.getSessionPath(sessionId)
	result, err := util.LoadOneFile[[]core.MetaRunProposalReviewRecord](ctx, path)

	if err != nil || result == nil {
		return []core.MetaRunProposalReviewRecord{}
	}

	return *result
}

func (m *MetaRunProposalReviewStore) Save(ctx context.Context, record core.MetaRunProposalReviewRecord) error {
	records := m.loadSessionRecords(ctx, record.SessionId)
	existingIndex := -1
	for i := 0; i < len(records); i++ {
		if records[i].ProposalId == record.ProposalId {
			existingIndex = i
			break
		}
	}

	if existingIndex >= 0 {
		records[existingIndex] = record
	} else {
		records = append(records, record)
	}

	return m.saveSessionRecords(ctx, record.SessionId, records)
}

func (m *MetaRunProposalReviewStore) Get(ctx context.Context, sessionId, proposalId string) *core.MetaRunProposalReviewRecord {
	records := m.loadSessionRecords(ctx, sessionId)
	for _, v := range records {
		if v.ProposalId == proposalId {
			return &v
		}
	}
	return nil
}

func (m *MetaRunProposalReviewStore) LoadBySession(ctx context.Context, sessionId string) map[string]core.MetaRunProposalReviewRecord {
	records := m.loadSessionRecords(ctx, sessionId)
	result := map[string]core.MetaRunProposalReviewRecord{}
	for _, v := range records {
		result[v.ProposalId] = v
	}
	return result
}
