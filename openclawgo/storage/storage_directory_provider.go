package storage

import "context"

type IStorageDirectoryProvider interface {
	GetStorageDirectory(ctx context.Context, agentName string) (string, error)
}
