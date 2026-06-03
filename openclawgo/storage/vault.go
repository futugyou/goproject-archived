package storage

import (
	"context"
	"errors"
)

type IVault interface {
	Resolve(ctx context.Context, name string, vaultCallerContext VaultCallerContext) (string, error)
}

var _ IVault = (*VaultService)(nil)

type VaultService struct {
	store    ISecretsStore
	auditor  ISecretAccessAuditor
	redactor IVaultSecretRedactor
}

func NewVaultService(
	store ISecretsStore,
	auditor ISecretAccessAuditor,
	redactor IVaultSecretRedactor,
) *VaultService {
	return &VaultService{
		store:    store,
		auditor:  auditor,
		redactor: redactor,
	}
}

// Resolve implements [IVault].
func (v *VaultService) Resolve(ctx context.Context, name string, vaultCallerContext VaultCallerContext) (string, error) {
	if len(name) == 0 {
		if err := v.auditor.Record(ctx, "<invalid>", vaultCallerContext, false); err != nil {
			return "", err
		}
		return "", errors.New("vault secret reference is invalid")
	}

	value, err := v.store.Get(ctx, name, -1)
	if err != nil {
		return "", err
	}
	if len(value) > 0 {
		if err := v.auditor.Record(ctx, name, vaultCallerContext, true); err != nil {
			return "", err
		}
		if err := v.redactor.TrackResolvedValue(value); err != nil {
			return "", err
		}
	}
	return value, nil
}
