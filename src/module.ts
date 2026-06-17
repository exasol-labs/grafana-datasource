import { DataSourcePlugin } from '@grafana/data';
import { DataSource } from './datasource';
import { ConfigEditor } from './components/ConfigEditor';
import { QueryEditor } from './components/QueryEditor';
import { ExasolQuery, ExasolDataSourceOptions } from './types';

export const plugin = new DataSourcePlugin<DataSource, ExasolQuery, ExasolDataSourceOptions>(DataSource)
  .setConfigEditor(ConfigEditor)
  .setQueryEditor(QueryEditor);
