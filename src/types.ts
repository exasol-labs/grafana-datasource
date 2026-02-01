import { DataSourceJsonData } from '@grafana/data';
import { DataQuery } from '@grafana/schema';

export type QueryFormat = 'table' | 'time_series';

export interface MyQuery extends DataQuery {
  queryText: string;
  format?: QueryFormat;
}

export const DEFAULT_QUERY: Partial<MyQuery> = {
  queryText: `SELECT
  $__time(INTERVAL_START),
  USERS_AVG,
  USERS_MAX,
  CLUSTER_NAME
FROM EXA_USAGE_HOURLY
WHERE $__timeFilter(INTERVAL_START)
ORDER BY INTERVAL_START;`,
  format: 'table',
};

export interface DataPoint {
  Time: number;
  Value: number;
}

export interface DataSourceResponse {
  datapoints: DataPoint[];
}

/**
 * These are options configured for each DataSource instance
 */
export interface MyDataSourceOptions extends DataSourceJsonData {
  databaseHost?: string;
  databasePort?: string;
  databaseInsecureSkipVerify?: boolean;
  schema?: string;
  user?: string;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface MySecureJsonData {
  password?: string;
}
