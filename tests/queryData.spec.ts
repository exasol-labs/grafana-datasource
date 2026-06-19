import type { Page } from '@playwright/test';
import { expect, test } from '@grafana/plugin-e2e';

const runQueryE2E = process.env.RUN_QUERY_E2E === 'true';

type QueryFormat = 'table' | 'time_series';

async function runQuery(page: Page, queryText: string, format: QueryFormat, expectedHttpStatus = 200) {
  const response = await page.request.post('/api/ds/query?ds_type=exasol-datasource&requestId=e2e', {
    data: {
      queries: [
        {
          refId: 'A',
          queryText,
          format,
          datasource: {
            type: 'exasol-datasource',
            uid: 'exasol-e2e',
          },
          intervalMs: 60000,
          maxDataPoints: 100,
        },
      ],
      from: '1745420000000',
      to: '1773670962836',
    },
  });

  expect(response.status()).toBe(expectedHttpStatus);
  return response.json();
}

test.describe('query data', () => {
  test.skip(!runQueryE2E, 'Requires RUN_QUERY_E2E=true and a reachable Exasol test database.');

  test('deterministic literal query returns expected values and types', async ({ page }) => {
    const body = await runQuery(
      page,
      `SELECT
    CAST('2025-01-01 00:00:00' AS TIMESTAMP) AS FIXED_TIME,
    CAST(42 AS DECIMAL(18,0)) AS FIXED_INT,
    CAST(12.5 AS DOUBLE) AS FIXED_FLOAT,
    'ok' AS FIXED_TEXT,
    TRUE AS FIXED_BOOL
  FROM DUAL`,
      'table'
    );

    const result = body.results.A;
    expect(result.status).toBe(200);

    const fields = result.frames[0].schema.fields;
    expect(fields.map((field: any) => field.name)).toEqual([
      'FIXED_TIME',
      'FIXED_INT',
      'FIXED_FLOAT',
      'FIXED_TEXT',
      'FIXED_BOOL',
    ]);
    expect(fields.map((field: any) => field.type)).toEqual(['time', 'number', 'number', 'string', 'boolean']);

    const values = result.frames[0].data.values;
    expect(values[0][0]).toBe(1735689600000);
    expect(values[1][0]).toBe(42);
    expect(values[2][0]).toBe(12.5);
    expect(values[3][0]).toBe('ok');
    expect(values[4][0]).toBe(true);
  });

  test('table query preserves varchar values that look like numbers or booleans', async ({ page }) => {
    const body = await runQuery(
      page,
      `SELECT
    '00123' AS CODE_TEXT,
    'false' AS BOOL_TEXT,
    '1.9' AS FLOAT_TEXT
  FROM DUAL`,
      'table'
    );

    const result = body.results.A;
    expect(result.status).toBe(200);

    const fields = result.frames[0].schema.fields;
    expect(fields.map((field: any) => field.name)).toEqual(['CODE_TEXT', 'BOOL_TEXT', 'FLOAT_TEXT']);
    expect(fields.map((field: any) => field.type)).toEqual(['string', 'string', 'string']);

    const values = result.frames[0].data.values;
    expect(values[0][0]).toBe('00123');
    expect(values[1][0]).toBe('false');
    expect(values[2][0]).toBe('1.9');
  });

  test('table query preserves high precision decimals as strings', async ({ page }) => {
    const body = await runQuery(
      page,
      `SELECT
    CAST(12345678901234567890.1234567890 AS DECIMAL(30,10)) AS BIG_DECIMAL
  FROM DUAL`,
      'table'
    );

    const result = body.results.A;
    expect(result.status).toBe(200);
    expect(result.frames[0].schema.fields.map((field: any) => field.name)).toEqual(['BIG_DECIMAL']);
    expect(result.frames[0].schema.fields.map((field: any) => field.type)).toEqual(['string']);
    const value = result.frames[0].data.values[0][0];
    expect(typeof value).toBe('string');
    expect(value).toMatch(/^12345678901234567890\.1234567890?$/);
  });

  test('time series query keeps high precision decimals numeric', async ({ page }) => {
    const body = await runQuery(
      page,
      `SELECT
    CAST('2025-01-01 00:00:00' AS TIMESTAMP) AS FIXED_TIME,
    CAST(12345678901234567890.1234567890 AS DECIMAL(30,10)) AS BIG_DECIMAL
  FROM DUAL`,
      'time_series'
    );

    const result = body.results.A;
    expect(result.status).toBe(200);

    const fields = result.frames[0].schema.fields;
    expect(fields.map((field: any) => field.type)).toEqual(['time', 'number']);
    expect(typeof result.frames[0].data.values[1][0]).toBe('number');
  });

  test('table query returns typed fields from EXA_SYSTEM_EVENTS', async ({ page }) => {
    const body = await runQuery(
      page,
      `SELECT
    CLUSTER_NAME,
    $__time(MEASURE_TIME),
    EVENT_TYPE,
    NODES,
    DB_RAM_SIZE,
    VCPU
  FROM EXA_SYSTEM_EVENTS
  WHERE $__timeFilter(MEASURE_TIME)
  ORDER BY MEASURE_TIME DESC`,
      'table'
    );

    const result = body.results.A;
    expect(result.status).toBe(200);
    expect(result.frames[0].schema.fields.map((field: any) => field.name)).toEqual([
      'CLUSTER_NAME',
      'time',
      'EVENT_TYPE',
      'NODES',
      'DB_RAM_SIZE',
      'VCPU',
    ]);
    expect(result.frames[0].schema.fields.map((field: any) => field.type)).toEqual([
      'string',
      'time',
      'string',
      'number',
      'number',
      'number',
    ]);
  });

  test('time series query with grouping returns a time field and numeric series', async ({ page }) => {
    const body = await runQuery(
      page,
      `SELECT
    $__timeGroupAlias(MEASURE_TIME, '1h'),
    MAX(NODES) AS nodes,
    MAX(DB_RAM_SIZE) AS db_ram_size,
    MAX(VCPU) AS vcpu,
    CLUSTER_NAME
  FROM EXA_SYSTEM_EVENTS
  WHERE $__timeFilter(MEASURE_TIME)
  GROUP BY 1, 5
  ORDER BY 1`,
      'time_series'
    );

    const result = body.results.A;
    expect(result.status).toBe(200);

    const fieldTypes = result.frames[0].schema.fields.map((field: any) => field.type);
    expect(fieldTypes[0]).toBe('time');
    expect(fieldTypes.slice(1).some((type: string) => type === 'number')).toBeTruthy();
  });

  test('time series query without numeric fields returns a clear validation error', async ({ page }) => {
    const body = await runQuery(
      page,
      `SELECT
    $__time(MEASURE_TIME),
    CLUSTER_NAME,
    EVENT_TYPE
  FROM EXA_SYSTEM_EVENTS
  WHERE $__timeFilter(MEASURE_TIME)
  ORDER BY MEASURE_TIME`,
      'time_series',
      400
    );

    const result = body.results.A;
    expect(result.status).toBe(400);
    expect(result.error).toContain('time series format requires at least one time column and one numeric column');
  });
});
