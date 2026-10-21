package main

import (
	"context"
	"database/sql"
	"fmt"
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

type Point struct {
	Timestamp int64
	CPU       sql.NullFloat64
	Memory    sql.NullFloat64
}

type Partman struct {
	db     *sql.DB
	nodeID string
	step   int64 // seconds, equivalent to RRD --step
}

func NewPartman(db *sql.DB, nodeID string, step int64) (*Partman, error) {
	if db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	if nodeID == "" {
		return nil, fmt.Errorf("nodeID is empty")
	}
	if step <= 0 {
		return nil, fmt.Errorf("step must be positive")
	}

	return &Partman{
		db:     db,
		nodeID: nodeID,
		step:   step,
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
            (time, node_id, cpu_usage, mem_usage)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (time, node_id)
        DO UPDATE SET
            cpu_usage = EXCLUDED.cpu_usage,
            mem_usage = EXCLUDED.mem_usage
    `, ts, s.nodeID, cpuUsage, memUsage)

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

	// The query covers [startEpoch, endEpoch].
	// Bucket boundaries are aligned to Unix epoch.
	query := `
        SELECT
            (floor(extract(epoch FROM time) / $4) * $4)::bigint
                AS bucket_epoch,
            AVG(cpu_usage) AS cpu_avg,
            AVG(mem_usage) AS mem_avg
        FROM public.node_metrics
        WHERE node_id = $1
          AND time >= to_timestamp($2)
          AND time <= to_timestamp($3)
        GROUP BY bucket_epoch
        ORDER BY bucket_epoch
    `

	rows, err := s.db.QueryContext(
		ctx,
		query,
		s.nodeID,
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
		var p Point
		if err := rows.Scan(
			&p.Timestamp,
			&p.CPU,
			&p.Memory,
		); err != nil {
			return nil, fmt.Errorf("scan metric: %w", err)
		}
		points = append(points, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate metrics: %w", err)
	}

	return points, nil
}
