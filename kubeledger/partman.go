package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func OpenDB(ctx context.Context, dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL is not set")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	return db, nil
}

func UpsertMetricsCategory(ctx context.Context, db *sql.DB, category string) {
	_, err := db.ExecContext(ctx, `
        INSERT INTO public.node_metrics_categories
            (category)
        VALUES ($1)
        ON CONFLICT (category)
        DO UPDATE SET
    `, category)

	if err != nil {
		KOA_LOGGER.Error(fmt.Sprintf("insert node_metrics_categories at %s: %s", time.Now(), err.Error()))
	}
}

func GetMetricsCategories(ctx context.Context, db *sql.DB) []string {
	query := `SELECT category FROM public.node_metrics_categories`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		KOA_LOGGER.Error(fmt.Sprintf("fetch metrics: %s", err.Error()))
		return nil
	}

	defer rows.Close()

	categories := make([]string, 0)
	for rows.Next() {
		var category string

		if err := rows.Scan(&category); err != nil {
			KOA_LOGGER.Error(fmt.Sprintf("scan metrics_categories: %s", err.Error()))
			return nil
		}

		categories = append(categories, category)
	}

	return categories
}

type Point struct {
	Timestamp int64
	CPU       float64
	Memory    float64
}

type Partman struct {
	db       *sql.DB
	category string
	step     int64 // seconds, equivalent to RRD --step
}

func NewPartman(db *sql.DB, category string, step int64) (*Partman, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	if category == "" {
		return nil, fmt.Errorf("category is empty")
	}
	if step <= 0 {
		return nil, fmt.Errorf("step must be positive")
	}

	UpsertMetricsCategory(context.Background(), db, category)

	return &Partman{
		db:       db,
		category: category,
		step:     step,
	}, nil
}

// Create verifies that the SQL resources were initialized.
// Run the SQL setup script from Supabase SQL Editor first.
func (s *Partman) Create(ctx context.Context) error {
	var tableName sql.NullString

	err := s.db.QueryRowContext(ctx,
		`SELECT to_regclass('public.node_metrics')::text`,
	).Scan(&tableName)
	if err != nil {
		return fmt.Errorf("check node_metrics: %w", err)
	}
	if !tableName.Valid {
		return fmt.Errorf(
			"public.node_metrics does not exist; run the SQL setup script first",
		)
	}

	var isPartitioned bool
	err = s.db.QueryRowContext(ctx, `
        SELECT c.relkind = 'p'
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relname = 'node_metrics'
    `).Scan(&isPartitioned)
	if err != nil {
		return fmt.Errorf("check partitioned table: %w", err)
	}
	if !isPartitioned {
		return fmt.Errorf("public.node_metrics is not partitioned")
	}

	return nil
}

func (s *Partman) Update(
	ctx context.Context,
	timestampEpoch int64,
	cpuUsage float64,
	memUsage float64,
) error {
	if timestampEpoch < 0 {
		return fmt.Errorf("timestamp must be non-negative")
	}
	if cpuUsage < 0 || cpuUsage > 100 {
		return fmt.Errorf("cpu usage must be between 0 and 100")
	}
	if memUsage < 0 || memUsage > 100 {
		return fmt.Errorf("memory usage must be between 0 and 100")
	}

	ts := time.Unix(timestampEpoch, 0).UTC()

	_, err := s.db.ExecContext(ctx, `
        INSERT INTO public.node_metrics
            (time, category, cpu_usage, mem_usage)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (time, category)
        DO UPDATE SET
            cpu_usage = EXCLUDED.cpu_usage,
            mem_usage = EXCLUDED.mem_usage
    `, ts, s.category, cpuUsage, memUsage)

	if err != nil {
		return fmt.Errorf("insert metric at %d: %w", timestampEpoch, err)
	}
	return nil
}

func (s *Partman) Fetch(
	ctx context.Context,
	startEpoch int64,
	endEpoch int64,
	step int64,
) ([]Point, error) {
	if step <= 0 {
		return nil, fmt.Errorf("step must be positive")
	}
	if endEpoch < startEpoch {
		return nil, fmt.Errorf("end must not be before start")
	}

	// 使用 generate_series 生成对齐到 step 边界的完整时间轴，
	// 并通过 LEFT JOIN 确保即使某一个时间段没有数据，也会返回这一行（值为 NULL）。
	query := `
		WITH aligned_bounds AS (
			SELECT 
				($2 / $4) * $4 AS start_ts,
				($3 / $4) * $4 AS end_ts
		),
		time_buckets AS (
			SELECT generate_series(
				to_timestamp(ab.start_ts),
				to_timestamp(ab.end_ts),
				($4 * interval '1 second')
			) AS bucket_time
			FROM aligned_bounds ab
		),
		aggregated_metrics AS (
			SELECT
				(floor(extract(epoch FROM time) / $4) * $4)::bigint AS bucket_epoch,
				AVG(cpu_usage) AS cpu_avg,
				AVG(mem_usage) AS mem_avg
			FROM public.node_metrics
			WHERE category = $1
			  AND time >= (SELECT to_timestamp(start_ts) FROM aligned_bounds)
			  AND time <= (SELECT to_timestamp(end_ts) FROM aligned_bounds)
			GROUP BY bucket_epoch
		)
		SELECT
			extract(epoch FROM t.bucket_time)::bigint AS bucket_epoch,
			m.cpu_avg,
			m.mem_avg
		FROM time_buckets t
		LEFT JOIN aggregated_metrics m 
			ON extract(epoch FROM t.bucket_time)::bigint = m.bucket_epoch
		ORDER BY bucket_epoch
	`

	rows, err := s.db.QueryContext(
		ctx,
		query,
		s.category,
		startEpoch,
		endEpoch,
		step,
	)
	if err != nil {
		return nil, fmt.Errorf("fetch metrics: %w", err)
	}
	defer rows.Close()

	points := make([]Point, 0)
	for rows.Next() {
		var timestamp int64
		var cpuSql, memSql sql.NullFloat64

		if err := rows.Scan(
			&timestamp,
			&cpuSql,
			&memSql,
		); err != nil {
			return nil, fmt.Errorf("scan metric: %w", err)
		}

		p := Point{
			Timestamp: timestamp,
			CPU:       getIntOrZero(cpuSql),
			Memory:    getIntOrZero(memSql),
		}
		points = append(points, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate metrics: %w", err)
	}

	return points, nil
}

func getIntOrZero(ns sql.NullFloat64) float64 {
	if ns.Valid {
		return ns.Float64
	}
	return 0.0
}

func getDateGroup(timeUTC time.Time, period int) string {
	if period == PERIOD_YEAR_SEC {
		return timeUTC.Format("200601")
	}
	return timeUTC.Format("20060102")
}

func round(val float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(val*pow) / pow
}

func (s *Partman) AddSample(ctx context.Context,
	timestampEpoch int64,
	cpuUsage float64,
	memUsage float64) {
	KOA_LOGGER.Debug(fmt.Sprintf("[puller][sample] node_metrics, %f, %f", cpuUsage, memUsage))

	cpu_rounded := round(cpuUsage, KOA_CONFIG.DbRoundDecimals)
	mem_rounded := round(memUsage, KOA_CONFIG.DbRoundDecimals)
	if err := s.Update(ctx, timestampEpoch, cpu_rounded, mem_rounded); err != nil {
		KOA_LOGGER.Error(fmt.Sprintf("failing adding rrd sample => %s", err.Error()))
	}
}

func (s *Partman) DumpTrendData(ctx context.Context, period, step_in int) ([]TrendDataDTO, []TrendDataDTO, error) {
	now_epoch_utc := time.Now().Unix()
	step := step_in
	if step < 0 {
		step = PERIOD_1_HOUR_SEC
	}

	rrd_end_ts_in := now_epoch_utc / int64(step) * int64(step)
	rrd_start_ts_in := rrd_end_ts_in - int64(period)
	rrd_result, err := s.Fetch(ctx, rrd_start_ts_in, rrd_end_ts_in, int64(step))
	if err != nil {
		return nil, nil, err
	}

	cpu_usage := []TrendDataDTO{}
	mem_usage := []TrendDataDTO{}
	sum_res_usage := map[string]float64{}
	for i := range rrd_result {
		cdp := rrd_result[i]
		current_cpu_usage := round(100*cdp.CPU, KOA_CONFIG.DbRoundDecimals) / 100
		current_mem_usage := round(100*cdp.Memory, KOA_CONFIG.DbRoundDecimals) / 100
		dateUTC := time.Unix(cdp.Timestamp, 0).Format("2006-01-02T15:04:05")

		cpu_usage = append(cpu_usage, TrendDataDTO{
			Name:    "node_metrics",
			DateUTC: dateUTC,
			Usage:   current_cpu_usage,
		})
		mem_usage = append(mem_usage, TrendDataDTO{
			Name:    "node_metrics",
			DateUTC: dateUTC,
			Usage:   current_mem_usage,
		})
		sum_res_usage["cpu"] = sum_res_usage["cpu"] + current_cpu_usage
		sum_res_usage["mem"] = sum_res_usage["mem"] + current_mem_usage

		if cdp.Timestamp == now_epoch_utc/PERIOD_1_HOUR_SEC*PERIOD_1_HOUR_SEC {
			PROMETHEUS_HOURLY_USAGE_EXPORTER.WithLabelValues("node_metrics", "CPU").Set(current_cpu_usage)
			PROMETHEUS_HOURLY_USAGE_EXPORTER.WithLabelValues("node_metrics", "MEMORY").Set(current_mem_usage)
		}
	}

	if sum_res_usage["cpu"] > 0 && sum_res_usage["mem"] > 0 {
		return cpu_usage, mem_usage, nil
	} else if step_in > 0 {
		return s.DumpTrendData(ctx, period, PERIOD_5_MINS_SEC)
	}

	return nil, nil, errors.New("some error in DumpTrendData()")
}

func (s *Partman) DumpHistogramData(ctx context.Context, period, step_in int) (map[string]float64, map[string]float64, error) {
	step := step_in
	if step < 0 {
		step = PERIOD_1_HOUR_SEC
	}

	rrd_end_ts_in := time.Now().Unix() / int64(step) * int64(step)
	rrd_start_ts_in := rrd_end_ts_in - int64(period)
	rrd_result, err := s.Fetch(ctx, rrd_start_ts_in, rrd_end_ts_in, int64(step))
	if err != nil {
		return nil, nil, err
	}

	periodic_cpu_usage := map[string]float64{}
	periodic_mem_usage := map[string]float64{}
	for i := range rrd_result {
		cdp := rrd_result[i]
		date_group := getDateGroup(time.Now(), period)
		current_cpu_usage := round(100*cdp.CPU, KOA_CONFIG.DbRoundDecimals) / 100
		current_mem_usage := round(100*cdp.Memory, KOA_CONFIG.DbRoundDecimals) / 100
		periodic_cpu_usage[date_group] += current_cpu_usage
		periodic_mem_usage[date_group] += current_mem_usage
	}

	return periodic_cpu_usage, periodic_mem_usage, nil
}
