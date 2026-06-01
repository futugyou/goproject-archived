package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const SecretAccessGenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"

func SecretAccessComputeRowHash(
	previousRowHash *string,
	accessedAt time.Time,
	callerType string,
	callerID string,
	sessionID *string,
	secretName string,
	success bool,
) string {

	status := "failure"
	if success {
		status = "success"
	}

	canonical := fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%s",
		secretAccessNormalizeHash(previousRowHash),
		secretAccessNormalizeUTC(accessedAt).Format(time.RFC3339Nano),
		secretAccessNormalize(callerType),
		secretAccessNormalize(callerID),
		secretAccessNormalizePtr(sessionID),
		secretAccessNormalize(secretName),
		status,
	)

	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

func secretAccessNormalize(value string) string {
	return value
}

func secretAccessNormalizePtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func secretAccessNormalizeHash(value *string) string {
	if value == nil {
		return SecretAccessGenesisHash
	}
	v := strings.TrimSpace(*value)
	if v == "" {
		return SecretAccessGenesisHash
	}
	return strings.ToLower(v)
}

func secretAccessNormalizeUTC(t time.Time) time.Time {
	if t.Location() == nil {
		return t.UTC()
	}
	return t.UTC()
}
