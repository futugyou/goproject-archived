-- 在 Supabase SQL Editor 中执行。
-- 1. base
create schema if not exists partman;

create extension if not exists pg_partman
    with schema partman;

create extension if not exists pg_cron;

-- 监控系统有多个节点，使用 category 区分。
create table if not exists public.node_metrics_categories ( 
    category text not null, 

    constraint node_metrics_categories_pkey primary key (category), 
) partition by range (category);

create table if not exists public.node_metrics (
    time        timestamptz not null,
    category    text        not null,
    cpu_usage   double precision,
    mem_usage   double precision,

    constraint node_metrics_pkey primary key (time, category),

    constraint node_metrics_cpu_range
        check (cpu_usage is null or
               (cpu_usage >= 0 and cpu_usage <= 100)),

    constraint node_metrics_mem_range
        check (mem_usage is null or
               (mem_usage >= 0 and mem_usage <= 100))
) partition by range (time);

-- 创建时间分区管理配置。
-- p_premake = 4：提前创建未来的分区。
select partman.create_parent(
    p_parent_table := 'public.node_metrics',
    p_control := 'time',
    p_type := 'range',
    p_interval := '7 days',
    p_premake := 4
);

-- 2. index
create index if not exists node_metrics_node_time_idx
    on public.node_metrics (category, time desc);

-- 3. maintenance

-- 先手动执行一次，确认权限和配置正常。
call partman.run_maintenance_proc();

-- 每小时运行一次。
-- 如果任务已经存在，请先检查 cron.job，避免重复调度。
select cron.schedule(
    'node-metrics-partman-maintenance',
    '0 * * * *',
    $job$call partman.run_maintenance_proc();$job$
);


-- 4. metrics_3m 

create table if not exists public.node_metrics_3m (
    bucket_time timestamptz not null,
    category     text not null,
    cpu_avg     double precision,
    mem_avg     double precision,
    primary key (bucket_time, category)
) partition by range (bucket_time);

select partman.create_parent(
    p_parent_table := 'public.node_metrics_3m',
    p_control := 'bucket_time',
    p_type := 'range',
    p_interval := '7 days',
    p_premake := 4
);

create index if not exists node_metrics_3m_node_time_idx
    on public.node_metrics_3m (category, bucket_time desc); 

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
    -- 只处理已经结束的 3 分钟时间桶。
    v_until := date_bin(
        interval '3 minutes',
        v_now,
        timestamptz '2000-01-01 00:00:00+00'
    );

    -- 每次重算最近 24 小时，允许重复执行。
    v_from := v_until - interval '24 hours';

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
        AVG(cpu_usage),
        AVG(mem_usage)
    FROM public.node_metrics
    WHERE time >= v_from
      AND time < v_until
    GROUP BY 1, 2
    ON CONFLICT (bucket_time, category)
    DO UPDATE SET
        cpu_avg = EXCLUDED.cpu_avg,
        mem_avg = EXCLUDED.mem_avg;

    -- 删除过期的原始采样数据。
    DELETE FROM public.node_metrics
    WHERE time < v_now - interval '16 hours 48 minutes';

    -- 删除过期的汇总数据。
    DELETE FROM public.node_metrics_3m
    WHERE bucket_time < v_now - interval '18 days 12 hours';
END;
$$;

SELECT cron.schedule(
    'node-metrics-archive',
    '*/3 * * * *',
    'SELECT public.archive_node_metrics();'
);

-- 6. check
SELECT
    jobid,
    status,
    start_time,
    end_time,
    return_message
FROM cron.job_run_details
WHERE jobid = (
    SELECT jobid
    FROM cron.job
    WHERE jobname = 'node-metrics-archive'
)
ORDER BY start_time DESC
LIMIT 10;