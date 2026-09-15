package pluginkit

import (
	"github.com/futugyou/openclaw/pluginkit/reduction"
	"github.com/futugyou/openclaw/pluginkit/rules"
)

func CreateTokenJuiceInterceptor(rs []rules.TokenJuiceRule, density *reduction.SemanticDensityCalculator, maxInlineChars *int) *reduction.TokenJuiceInterceptor {
	if len(rs) == 0 {
		rs = rules.LoadMergedRules(nil)
	}

	return reduction.NewTokenJuiceInterceptor(rs, density, maxInlineChars)
}
