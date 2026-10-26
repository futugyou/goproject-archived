package mcp

const (
	DEFAULT_DATA_DIR        = "./static/data"
	BACKEND_CONFIG_FILENAME = "backend.json"

	// Histogram periods written by backend.py:dump_histogram_analytics.
	// These are not granularities: they are retrieval windows. backend.py groups
	// by month for the year window and by day for the 14-day window. See spec §2.1.
	PERIOD_14_DAYS_SEC = 1209600
	PERIOD_YEAR_SEC    = 31968000

	// Public scale identifiers exposed by the MCP tools.
	SCALE_DAILY_14D   = "daily_14d"
	SCALE_MONTHLY_12M = "monthly_12m"

	// Warn when the most recent data file is older than this many seconds.
	// backend.py dumps every ~5 minutes; 30 minutes leaves ample headroom.
	FRESHNESS_WARN_AFTER_SECONDS = 1800

	// Efficiency / requests handling (spec §4.3).
	//
	// requests is derived in backend.py as ``usage / rf`` (request factor), so a
	// very high rf yields an *infinitesimal* but non-zero requests value. The
	// exact ``requests == 0`` test is therefore insufficient: any value below
	// this threshold must be treated as zero, otherwise usage/requests produces
	// numerical aberrations.
	REQUESTS_ZERO_THRESHOLD = 1e-6

	// Efficiency classification thresholds (descriptive, not prescriptive).
	// - ratio < 0.5  → over_provisioned (usage much smaller than requests)
	// - 0.5 ≤ ratio ≤ 1.0 → balanced
	// - ratio > 1.0  → under_provisioned (usage exceeds requests)
	EFFICIENCY_OVER_PROVISIONED_BELOW  = 0.5
	EFFICIENCY_UNDER_PROVISIONED_ABOVE = 1.0

	// Relative change threshold for ``compare_periods`` trend classification:
	// a noise filter, applied to the difference between first-half and second-half
	// means of the monthly series. 10 % of the baseline.
	TREND_CHANGE_THRESHOLD = 0.10

	// Maximum length of a glob pattern accepted by ``tool_group_namespaces``.
	// fnmatch internally compiles to a regex; pathological patterns built from
	// many wildcards (e.g. ``"*?" * 1000``) can cause catastrophic backtracking.
	// 256 chars is far above any realistic namespace selector and below the
	// point where regex compilation cost matters.
	GROUP_PATTERN_MAX_LENGTH = 256
)

var SCALE_TO_PERIOD = map[string]int{
	SCALE_DAILY_14D:   PERIOD_14_DAYS_SEC,
	SCALE_MONTHLY_12M: PERIOD_YEAR_SEC,
}
var SCALE_GRANULARITY = map[string]string{
	SCALE_DAILY_14D:   "daily",
	SCALE_MONTHLY_12M: "monthly",
}
var SCALE_DEFAULT_DEPTH = map[string]string{
	SCALE_DAILY_14D:   "14 days",
	SCALE_MONTHLY_12M: "~12 months",
}

// Translation from the cost_model written by backend.py to its semantic unit
// (used in the metadata block of every MCP response: spec §4.6).
var COST_MODEL_UNIT = map[string]string{
	"cumulative": "percent_of_cluster_capacity",
	"normalized": "relative_share_percent",
	"costs":      "monetary_cost",
}

var METRICS = []string{"cpu", "memory"}
var DIMENSIONS = []string{"usage", "requests"}

// Prefixes used to classify namespaces as "system" rather than "application"
// (spec §4.1, §2.4). OpenShift is the primary context (operators in
// openshift-*); the standard upstream Kubernetes namespaces (kube-system,
// kube-public, kube-node-lease) share the same role.
var SYSTEM_NAMESPACE_PREFIXES = []string{"openshift-", "kube-"}

// Special entries that are NOT namespaces: see spec §2.4. Always listed
// in list_namespaces' special_entries block, regardless of whether they
// happen to appear in the data on this particular run.
var SPECIAL_ENTRIES = map[string]map[string]string{
	"non-allocatable": {
		"role": "cluster_overhead",
		"description": "Cluster capacity reserved for system overhead, not allocatable " +
			"to pods. Excluded from usage analyses by default, but exposed as " +
			"a separate signal: e.g. cluster_overhead in get_namespace_breakdown.",
	},
	".billing-hourly-rate": {
		"role":        "billing_config",
		"description": "Hourly billing rate, present only when cost_model=costs. Not a usage data point: a configuration value.",
	},
}
