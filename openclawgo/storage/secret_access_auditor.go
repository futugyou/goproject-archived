package storage

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type VaultCallerContext struct {
	CallerType VaultCallerType
	CallerId   string
	SessionId  string
}

type VaultCallerType string

const (
	VaultCallerTypeTool          VaultCallerType = "Tool"
	VaultCallerTypeConfiguration VaultCallerType = "Configuration"
	VaultCallerTypeCli           VaultCallerType = "Cli"
	VaultCallerTypeSystem        VaultCallerType = "System"
)

type ISecretAccessAuditor interface {
	Record(ctx context.Context, secretName string, vaultCallerContext VaultCallerContext, success bool) error
}

var _ ISecretAccessAuditor = (*SecretAccessAuditor)(nil)

type SecretAccessAuditor struct {
	db *gorm.DB
}

func NewSecretAccessAuditor(db *gorm.DB) *SecretAccessAuditor {
	return &SecretAccessAuditor{db: db}
}

// Record implements [ISecretAccessAuditor].
func (s *SecretAccessAuditor) Record(ctx context.Context, secretName string, vaultCallerContext VaultCallerContext, success bool) error {
	var nextSequence int64 = 0
	err := s.db.Model(&SecretAccessAuditEntity{}).
		Select("max(sequence)").
		First(&nextSequence).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	} else {
		nextSequence = nextSequence + 1
	}

	previous := SecretAccessGenesisHash
	err = s.db.Model(&SecretAccessAuditEntity{}).
		Select("row_hash").
		Order("sequence desc").
		First(&previous).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var accessedAt = time.Now().UTC()
	var entity = SecretAccessAuditEntity{
		Id:              uuid.NewString(),
		Sequence:        nextSequence,
		SecretName:      secretName,
		CallerType:      string(vaultCallerContext.CallerType),
		CallerId:        vaultCallerContext.CallerId,
		SessionId:       vaultCallerContext.SessionId,
		AccessedAt:      accessedAt,
		Success:         success,
		PreviousRowHash: previous,
		RowHash: SecretAccessComputeRowHash(
			&previous,
			accessedAt,
			string(vaultCallerContext.CallerType),
			vaultCallerContext.CallerId,
			&vaultCallerContext.SessionId,
			secretName,
			success),
	}

	return s.db.Save(entity).Error
}
