/* eslint-disable no-console */
/**
 * Captures catalog screenshots from the locally running Grafana
 * (docker compose) and writes them to src/img/.
 *
 * Run with:  npx ts-node scripts/screenshots.ts
 * Requires:  Grafana at http://localhost:3000 with anonymous Admin enabled
 *            and at least one Exasol datasource configured.
 */
import { chromium, Browser, Page } from '@playwright/test';
import * as fs from 'fs';
import * as path from 'path';

const GRAFANA = process.env.GRAFANA_URL ?? 'http://localhost:3000';
const OUT_DIR = path.resolve(__dirname, '../src/img');
const VIEWPORT = { width: 1440, height: 900 };

interface DataSource {
  uid: string;
  name: string;
  type: string;
}

async function findExasolDatasource(): Promise<DataSource> {
  const res = await fetch(`${GRAFANA}/api/datasources`);
  if (!res.ok) {
    throw new Error(`failed to list datasources: ${res.status}`);
  }
  const datasources = (await res.json()) as DataSource[];
  const candidates = datasources.filter((ds) => ds.type === 'exasol-exasol-datasource');
  if (candidates.length === 0) {
    throw new Error('no exasol-exasol-datasource instance found — configure one in Grafana first');
  }
  // Prefer a non-e2e instance (user-configured), fall back to the first one.
  const preferred = candidates.find((ds) => ds.name !== 'exasol') ?? candidates[0];
  console.log(`using datasource: ${preferred.name} (${preferred.uid})`);
  return preferred;
}

async function snap(page: Page, file: string): Promise<void> {
  const outPath = path.join(OUT_DIR, file);
  await page.screenshot({ path: outPath, fullPage: false });
  const size = fs.statSync(outPath).size;
  console.log(`✓ ${file} (${Math.round(size / 1024)} KiB)`);
}

async function captureConfigEditor(browser: Browser, ds: DataSource): Promise<void> {
  const page = await browser.newPage({ viewport: VIEWPORT });
  await page.goto(`${GRAFANA}/connections/datasources/edit/${ds.uid}`, { waitUntil: 'networkidle' });
  await page.waitForSelector('#config-editor-database-host', { timeout: 10_000 });
  // Give settings panel a moment to render fully.
  await page.waitForTimeout(800);
  await snap(page, 'config-editor.png');
  await page.close();
}

async function exploreUrl(ds: DataSource, queryText: string, format: 'table' | 'time_series'): Promise<string> {
  return `${GRAFANA}/explore?left=${encodeURIComponent(
    JSON.stringify({
      datasource: ds.uid,
      queries: [{ refId: 'A', queryText, format }],
      range: { from: 'now-6h', to: 'now' },
    })
  )}`;
}

async function captureQueryEditor(browser: Browser, ds: DataSource): Promise<void> {
  const page = await browser.newPage({ viewport: VIEWPORT });
  // Use Exasol system tables that exist on every cluster.
  const query =
    'SELECT\n  USER_NAME,\n  CREATED,\n  USER_CONSUMER_GROUP\nFROM EXA_ALL_USERS\nORDER BY CREATED DESC\nLIMIT 50';
  await page.goto(await exploreUrl(ds, query, 'table'), { waitUntil: 'networkidle' });
  await page.waitForSelector('#query-editor-sql', { timeout: 10_000 });
  // Wait for results to render.
  await page.waitForTimeout(2500);
  await snap(page, 'query-editor.png');
  await page.close();
}

async function captureTimeSeries(browser: Browser, ds: DataSource): Promise<void> {
  const page = await browser.newPage({ viewport: VIEWPORT });
  // EXA_USAGE_LAST_DAY is a standard usage view available on every Exasol cluster.
  const query =
    "SELECT\n  $__timeGroupAlias(MEASURE_TIME, '5m'),\n  AVG(USERS) AS users,\n  AVG(QUERIES) AS queries,\n  CLUSTER_NAME\nFROM EXA_USAGE_LAST_DAY\nWHERE $__timeFilter(MEASURE_TIME)\nGROUP BY 1, CLUSTER_NAME\nORDER BY 1";
  await page.goto(await exploreUrl(ds, query, 'time_series'), { waitUntil: 'networkidle' });
  await page.waitForSelector('#query-editor-sql', { timeout: 10_000 });
  await page.waitForTimeout(3500);
  await snap(page, 'time-series.png');
  await page.close();
}

async function main(): Promise<void> {
  fs.mkdirSync(OUT_DIR, { recursive: true });
  const ds = await findExasolDatasource();

  const browser = await chromium.launch({ headless: true });
  try {
    await captureConfigEditor(browser, ds);
    await captureQueryEditor(browser, ds);
    await captureTimeSeries(browser, ds);
  } finally {
    await browser.close();
  }
}

main().catch((err: unknown) => {
  console.error(err);
  process.exit(1);
});
