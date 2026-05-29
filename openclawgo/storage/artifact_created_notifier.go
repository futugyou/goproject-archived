package storage

type IArtifactCreatedNotifier interface {
	NotifyArtifactCreated(jobId, runId, artifactId string) error
}

type NullArtifactCreatedNotifier struct{}

// NotifyArtifactCreated implements [IArtifactCreatedNotifier].
func (n *NullArtifactCreatedNotifier) NotifyArtifactCreated(jobId string, runId string, artifactId string) error {
	return nil
}

var _ IArtifactCreatedNotifier = (*NullArtifactCreatedNotifier)(nil)
