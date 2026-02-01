-- Exasol Datasource Plugin - Example Queries

-- ==============================================
-- Basic Queries
-- ==============================================

-- Simple test query
SELECT 1 as test_value FROM DUAL;

-- Current timestamp
SELECT CURRENT_TIMESTAMP as now FROM DUAL;

-- ==============================================
-- Time Series Queries
-- ==============================================

-- Metrics over the last hour (requires a time column)
SELECT 
    timestamp as time,
    metric_value,
    metric_name
FROM system_metrics
WHERE timestamp > NOW() - INTERVAL '1' HOUR
ORDER BY timestamp;

-- Aggregated metrics by minute
SELECT 
    DATE_TRUNC('minute', timestamp) as time,
    AVG(cpu_usage) as avg_cpu,
    MAX(memory_usage) as max_memory,
    MIN(disk_free) as min_disk_free
FROM server_metrics
WHERE timestamp BETWEEN $__timeFrom() AND $__timeTo()
GROUP BY DATE_TRUNC('minute', timestamp)
ORDER BY time;

-- Multiple series
SELECT 
    timestamp as time,
    server_name as metric,
    AVG(response_time) as value
FROM request_metrics
WHERE timestamp > NOW() - INTERVAL '24' HOUR
GROUP BY timestamp, server_name
ORDER BY timestamp;

-- ==============================================
-- Table Queries
-- ==============================================

-- Top 10 customers by revenue
SELECT 
    customer_id,
    customer_name,
    SUM(order_amount) as total_revenue,
    COUNT(order_id) as order_count,
    MAX(order_date) as last_order
FROM orders o
JOIN customers c ON o.customer_id = c.id
WHERE order_date >= CURRENT_DATE - 30
GROUP BY customer_id, customer_name
ORDER BY total_revenue DESC
LIMIT 10;

-- Active database sessions
SELECT 
    session_id,
    user_name,
    client,
    LOGIN_TIME,
    DURATION as session_duration,
    TEMP_DB_RAM as temp_ram_mb
FROM EXA_DBA_SESSIONS
WHERE status = 'EXECUTE'
ORDER BY login_time DESC;

-- ==============================================
-- Statistical Queries
-- ==============================================

-- Daily aggregations
SELECT 
    DATE_TRUNC('day', order_date) as date,
    COUNT(*) as order_count,
    SUM(amount) as total_amount,
    AVG(amount) as avg_amount,
    MIN(amount) as min_amount,
    MAX(amount) as max_amount
FROM orders
WHERE order_date >= CURRENT_DATE - 90
GROUP BY DATE_TRUNC('day', order_date)
ORDER BY date;

-- Percentile calculations
SELECT 
    DATE_TRUNC('hour', timestamp) as time,
    MEDIAN(response_time) as median_response,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY response_time) as p95_response,
    PERCENTILE_CONT(0.99) WITHIN GROUP (ORDER BY response_time) as p99_response
FROM api_logs
WHERE timestamp > NOW() - INTERVAL '1' DAY
GROUP BY DATE_TRUNC('hour', timestamp)
ORDER BY time;

-- ==============================================
-- Monitoring Queries
-- ==============================================

-- Database size
SELECT 
    schema_name,
    RAW_OBJECT_SIZE / 1024 / 1024 / 1024 as size_gb,
    MEM_OBJECT_SIZE / 1024 / 1024 / 1024 as mem_size_gb
FROM EXA_DBA_OBJECT_SIZES
WHERE schema_name NOT IN ('SYS', 'EXA_STATISTICS')
ORDER BY size_gb DESC
LIMIT 20;

-- Table sizes
SELECT 
    table_name,
    table_schema,
    RAW_OBJECT_SIZE / 1024 / 1024 as size_mb,
    table_rows
FROM EXA_DBA_TABLES
WHERE table_schema = 'MY_SCHEMA'
ORDER BY size_mb DESC
LIMIT 20;

-- Query performance
SELECT 
    TO_CHAR(start_time, 'YYYY-MM-DD HH24:MI:SS') as time,
    session_id,
    command_name,
    DURATION / 1000 as duration_seconds,
    CPU / 1000 as cpu_seconds,
    TEMP_DB_RAM_PEAK / 1024 / 1024 as peak_ram_mb,
    SUBSTR(sql_text, 1, 100) as query_preview
FROM EXA_DBA_AUDIT_SQL
WHERE start_time > CURRENT_TIMESTAMP - INTERVAL '1' HOUR
  AND success = TRUE
  AND command_name IN ('SELECT', 'INSERT', 'UPDATE', 'DELETE')
ORDER BY duration_seconds DESC
LIMIT 50;

-- ==============================================
-- Business Analytics
-- ==============================================

-- Sales funnel
SELECT 
    funnel_stage,
    COUNT(*) as count,
    COUNT(*) * 100.0 / SUM(COUNT(*)) OVER () as percentage
FROM (
    SELECT 
        CASE 
            WHEN event_type = 'page_view' THEN '1. Viewed'
            WHEN event_type = 'add_to_cart' THEN '2. Added to Cart'
            WHEN event_type = 'checkout_start' THEN '3. Started Checkout'
            WHEN event_type = 'purchase' THEN '4. Purchased'
        END as funnel_stage
    FROM user_events
    WHERE event_date >= CURRENT_DATE - 7
)
WHERE funnel_stage IS NOT NULL
GROUP BY funnel_stage
ORDER BY funnel_stage;

-- Retention cohort
SELECT 
    DATE_TRUNC('month', first_purchase_date) as cohort_month,
    MONTHS_BETWEEN(purchase_date, first_purchase_date) as months_since_first,
    COUNT(DISTINCT customer_id) as customers
FROM (
    SELECT 
        customer_id,
        purchase_date,
        FIRST_VALUE(purchase_date) OVER (PARTITION BY customer_id ORDER BY purchase_date) as first_purchase_date
    FROM orders
    WHERE purchase_date >= ADD_MONTHS(CURRENT_DATE, -12)
)
GROUP BY cohort_month, months_since_first
ORDER BY cohort_month, months_since_first;

-- ==============================================
-- Notes
-- ==============================================
-- 
-- Time Macros (use in Grafana query editor):
-- - $__timeFrom() - Start of dashboard time range
-- - $__timeTo() - End of dashboard time range
-- - $__timeFilter(columnName) - Generates time filter
--
-- Variable Substitution:
-- - Use Grafana variables like: WHERE customer_id = ${customer_id}
--
-- Best Practices:
-- - Always include a time column for time series
-- - Use proper indexing on timestamp columns
-- - Limit result sets with LIMIT clause
-- - Use GROUP BY for aggregations
-- - Add WHERE clauses to filter data
