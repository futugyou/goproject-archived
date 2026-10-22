
-- 1. base table
create table if not exists public.node_metrics_categories ( 
    category text not null, 

    constraint node_metrics_categories_pkey primary key (category), 
) partition by range (category);

CREATE TABLE IF NOT EXISTS public.node_metrics (
    time        timestamptz NOT NULL,
    category    text NOT NULL,
    cpu_usage   double precision,
    mem_usage   double precision,

    CONSTRAINT node_metrics_pkey
        PRIMARY KEY (time, category),

    CONSTRAINT node_metrics_cpu_range
        CHECK (
            cpu_usage IS NULL OR
            (cpu_usage >= 0 AND cpu_usage <= 100)
        ),

    CONSTRAINT node_metrics_mem_range
        CHECK (
            mem_usage IS NULL OR
            (mem_usage >= 0 AND mem_usage <= 100)
        )
) PARTITION BY RANGE (time);

CREATE INDEX IF NOT EXISTS node_metrics_node_time_idx
    ON public.node_metrics (category, time DESC);



-- 2. partition_maintenance

CREATE OR REPLACE FUNCTION public.ensure_weekly_partitions(
    p_parent regclass,
    p_past_weeks integer DEFAULT 2,
    p_future_weeks integer DEFAULT 8
)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    v_parent_schema text;
    v_parent_name   text;
    v_parent_kind   "char";
    v_week_start    timestamptz;
    v_from          timestamptz;
    v_to            timestamptz;
    v_partition     text;
    i               integer;
BEGIN
    IF p_past_weeks < 0 OR p_future_weeks < 0 THEN
        RAISE EXCEPTION 'week counts must be non-negative';
    END IF;

    SELECT n.nspname, c.relname, c.relkind
      INTO v_parent_schema, v_parent_name, v_parent_kind
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE c.oid = p_parent;

    IF NOT FOUND OR v_parent_kind <> 'p' THEN
        RAISE EXCEPTION '% is not a partitioned table', p_parent;
    END IF;

    -- Calculate the start of the current week in UTC.
    v_week_start :=
        date_trunc('week', now() AT TIME ZONE 'UTC')
        AT TIME ZONE 'UTC';

    FOR i IN -p_past_weeks..p_future_weeks LOOP
        v_from := v_week_start + make_interval(weeks => i);
        v_to   := v_from + interval '7 days';

        v_partition := format(
            '%s_%s',
            v_parent_name,
            to_char(v_from AT TIME ZONE 'UTC', 'YYYYMMDD')
        );

        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS %I.%I
             PARTITION OF %I.%I
             FOR VALUES FROM (%L) TO (%L)',
            v_parent_schema,
            v_partition,
            v_parent_schema,
            v_parent_name,
            v_from,
            v_to
        );
    END LOOP;
END;
$$;

-- Create initial partitions before allowing the application to write.
SELECT public.ensure_weekly_partitions(
    'public.node_metrics'::regclass,
    2,
    8
);


-- 3. schedule 

-- 每天 UTC 02:00 检查并创建未来分区。
SELECT cron.schedule(
    'node-metrics-partition-maintenance',
    '0 2 * * *',
    $job$
        SELECT public.ensure_weekly_partitions(
            'public.node_metrics'::regclass,
            2,
            8
        );
    $job$
);

-- 4. metrics_3m

CREATE TABLE IF NOT EXISTS public.node_metrics_3m (
    bucket_time timestamptz NOT NULL,
    category     text NOT NULL,
    cpu_avg     double precision,
    mem_avg     double precision,

    PRIMARY KEY (bucket_time, category)
) PARTITION BY RANGE (bucket_time);

CREATE INDEX IF NOT EXISTS node_metrics_3m_node_time_idx
    ON public.node_metrics_3m (category, bucket_time DESC);

SELECT public.ensure_weekly_partitions(
    'public.node_metrics_3m'::regclass,
    2,
    8
);


-- 5. archive_metrics

CREATE OR REPLACE FUNCTION public.archive_node_metrics()
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
    v_now timestamptz := now();
    v_until timestamptz;
    v_from timestamptz;
BEGIN
    -- Only archive complete 3-minute buckets.
    v_until := date_bin(
        interval '3 minutes',
        v_now,
        timestamptz '2000-01-01 00:00:00+00'
    );

    v_from := v_until - interval '15 minutes';

    INSERT INTO public.node_metrics_3m (
        bucket_time,
        category,
        cpu_avg,
        mem_avg
    )
    SELECT
        date_bin(
            interval '3 minutes',
            time,
            timestamptz '2000-01-01 00:00:00+00'
        ),
        category,
        avg(cpu_usage),
        avg(mem_usage)
    FROM public.node_metrics
    WHERE time >= v_from
      AND time < v_until
    GROUP BY 1, 2
    ON CONFLICT (bucket_time, category)
    DO UPDATE SET
        cpu_avg = EXCLUDED.cpu_avg,
        mem_avg = EXCLUDED.mem_avg;

    -- Keep approximately 16h48m of raw samples.
    DELETE FROM public.node_metrics
    WHERE time < v_now - interval '16 hours 48 minutes';

    -- Keep approximately 18.5 days of aggregated data.
    DELETE FROM public.node_metrics_3m
    WHERE bucket_time < v_now - interval '18 days 12 hours';
END;
$$;

SELECT cron.schedule(
    'node-metrics-archive',
    '*/3 * * * *',
    $job$
        SELECT public.archive_node_metrics();
        SELECT public.ensure_weekly_partitions(
            'public.node_metrics_3m'::regclass,
            2,
            8
        );
    $job$
);
