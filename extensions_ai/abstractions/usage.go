package abstractions

type UsageDetails struct {
	InputTokenCount       *int64           `json:"input_token_count,omitempty"`
	OutputTokenCount      *int64           `json:"output_token_count,omitempty"`
	TotalTokenCount       *int64           `json:"total_token_count,omitempty"`
	CachedInputTokenCount *int64           `json:"cached_input_token_count,omitempty"`
	ReasoningTokenCount   *int64           `json:"reasoning_token_count,omitempty"`
	InputAudioTokenCount  *int64           `json:"input_audio_token_count,omitempty"`
	InputTextTokenCount   *int64           `json:"input_text_token_count,omitempty"`
	OutputAudioTokenCount *int64           `json:"output_audio_token_count,omitempty"`
	OutputTextTokenCount  *int64           `json:"output_text_token_count,omitempty"`
	AdditionalCounts      map[string]int64 `json:"additional_counts,omitempty"`
}

func DefaultUsageDetails() *UsageDetails {
	return &UsageDetails{
		AdditionalCounts: make(map[string]int64),
	}
}

// Add 将另一个 UsageDetails 的数据累加到当前实例中
func (u *UsageDetails) Add(usage *UsageDetails) {
	if usage == nil {
		return
	}

	u.InputTokenCount = nullableSum(u.InputTokenCount, usage.InputTokenCount)
	u.OutputTokenCount = nullableSum(u.OutputTokenCount, usage.OutputTokenCount)
	u.TotalTokenCount = nullableSum(u.TotalTokenCount, usage.TotalTokenCount)
	u.CachedInputTokenCount = nullableSum(u.CachedInputTokenCount, usage.CachedInputTokenCount)
	u.ReasoningTokenCount = nullableSum(u.ReasoningTokenCount, usage.ReasoningTokenCount)
	u.InputAudioTokenCount = nullableSum(u.InputAudioTokenCount, usage.InputAudioTokenCount)
	u.InputTextTokenCount = nullableSum(u.InputTextTokenCount, usage.InputTextTokenCount)
	u.OutputAudioTokenCount = nullableSum(u.OutputAudioTokenCount, usage.OutputAudioTokenCount)
	u.OutputTextTokenCount = nullableSum(u.OutputTextTokenCount, usage.OutputTextTokenCount)

	if len(usage.AdditionalCounts) > 0 {
		if u.AdditionalCounts == nil {
			u.AdditionalCounts = make(map[string]int64)
		}
		for k, v := range usage.AdditionalCounts {
			u.AdditionalCounts[k] += v
		}
	}
}

func nullableSum(a, b *int64) *int64 {
	if a == nil && b == nil {
		return nil
	}
	var sum int64
	if a != nil {
		sum += *a
	}
	if b != nil {
		sum += *b
	}
	return &sum
}
