package main

import "github.com/prometheus/client_golang/prometheus"

const (
	PERIOD_5_MINS_SEC  = 300
	PERIOD_1_HOUR_SEC  = 3600
	PERIOD_1_DAY_SEC   = 86400
	PERIOD_7_DAYS_SEC  = 604800
	PERIOD_14_DAYS_SEC = 1209600
	PERIOD_YEAR_SEC    = 31968000
)

var PROMETHEUS_HOURLY_USAGE_EXPORTER = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: "koa_namespace_daily_usage",
		Help: "Current daily resource usage per namespace",
	},
	[]string{"namespace", "resource"},
)

var PROMETHEUS_PERIODIC_USAGE_EXPORTERS = map[int]*prometheus.GaugeVec{
	PERIOD_14_DAYS_SEC: prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "koa_namespace_daily_usage",
			Help: "Current daily resource usage per namespace",
		},
		[]string{"namespace", "resource"},
	),
	PERIOD_YEAR_SEC: prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "koa_namespace_monthly_usage",
			Help: "Current monthly resource usage per namespace",
		},
		[]string{"namespace", "resource"},
	),
}

var PROMETHEUS_PERIODIC_REQUESTS_EXPORTERS = map[int]*prometheus.GaugeVec{
	PERIOD_14_DAYS_SEC: prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "koa_namespace_daily_requests",
			Help: "Current daily resource reservation per namespace",
		},
		[]string{"namespace", "resource"},
	),
	PERIOD_YEAR_SEC: prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "koa_namespace_monthly_requests",
			Help: "Current monthly resource reservation per namespace",
		},
		[]string{"namespace", "resource"},
	),
}
