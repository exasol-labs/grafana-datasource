import { test, expect } from '@grafana/plugin-e2e';

const runQueryE2E = process.env.RUN_QUERY_E2E === 'true';

test('smoke: should render config editor', async ({ createDataSourceConfigPage, readProvisionedDataSource, page }) => {
  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  await createDataSourceConfigPage({ type: ds.type });
  await expect(page.getByLabel('Host')).toBeVisible();
  await expect(page.getByLabel('Port')).toBeVisible();
  await expect(page.getByLabel('User')).toBeVisible();
  await expect(page.getByLabel('Schema')).toBeVisible();
  await expect(page.getByLabel('Password')).toBeVisible();
});

test('provisioned datasource health check succeeds', async ({ readProvisionedDataSource, page }) => {
  test.skip(!runQueryE2E, 'Requires RUN_QUERY_E2E=true and a reachable Exasol test database.');

  const ds = await readProvisionedDataSource({ fileName: 'datasources.yml' });
  const response = await page.request.get(`/api/datasources/uid/${ds.uid}/health`);
  expect(response.ok()).toBeTruthy();

  const body = await response.json();
  expect(body.status).toBe('OK');
});
