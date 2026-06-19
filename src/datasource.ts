import { DataSourceInstanceSettings, CoreApp, ScopedVars } from '@grafana/data';
import { DataSourceWithBackend, getTemplateSrv } from '@grafana/runtime';

import { ExasolQuery, ExasolDataSourceOptions, DEFAULT_QUERY } from './types';

export class DataSource extends DataSourceWithBackend<ExasolQuery, ExasolDataSourceOptions> {
  annotations = {};

  constructor(instanceSettings: DataSourceInstanceSettings<ExasolDataSourceOptions>) {
    super(instanceSettings);
  }

  getDefaultQuery(_: CoreApp): Partial<ExasolQuery> {
    return DEFAULT_QUERY;
  }

  applyTemplateVariables(query: ExasolQuery, scopedVars: ScopedVars) {
    return {
      ...query,
      queryText: getTemplateSrv().replace(query.queryText, scopedVars),
    };
  }

  filterQuery(query: ExasolQuery): boolean {
    return !!query.queryText && query.queryText.trim().length > 0;
  }
}
