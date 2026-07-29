package stateless

import "github.com/futugyou/mcp/core"

type StatelessSessionId struct {
	ClientInfo  *core.Implementation `json:"clientInfo"`
	UserIdClaim *UserIdClaim         `json:"userIdClaim"`
}

type UserIdClaim struct {
	Issuer string `json:"issuer"`
	Type   string `json:"type"`
	Value  string `json:"value"`
}
