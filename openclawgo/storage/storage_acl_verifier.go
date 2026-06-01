package storage

import "context"

type AclVerificationResult struct {
	IsSecure  bool
	Findings  []string
	ScopeRoot string
}

func SecureAclVerificationResult(scopeRoot string) *AclVerificationResult {
	return &AclVerificationResult{
		IsSecure:  true,
		Findings:  []string{},
		ScopeRoot: scopeRoot,
	}
}

type IStorageAclVerifier interface {
	Verify(ctx context.Context, scopeRoot string) (*AclVerificationResult, error)
}

var _ IStorageAclVerifier = (*NoopStorageAclVerifier)(nil)

type NoopStorageAclVerifier struct {
}

// Verify implements [IStorageAclVerifier].
func (n *NoopStorageAclVerifier) Verify(ctx context.Context, scopeRoot string) (*AclVerificationResult, error) {
	return SecureAclVerificationResult(scopeRoot), nil
}
