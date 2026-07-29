package core

type EmptyResult struct {
	Meta       map[string]any `json:"_meta,omitempty" yaml:"_meta,omitempty" mapstructure:"_meta,omitempty"`
	ResultType string         `json:"resultType,omitempty" yaml:"resultType,omitempty" mapstructure:"resultType,omitempty"`
}

func NewEmptyResult() EmptyResult {
	return EmptyResult{
		ResultType: "complete",
	}
}
