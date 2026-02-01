import React, { ChangeEvent } from 'react';
import { InlineField, Select, TextArea } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { MyDataSourceOptions, MyQuery, QueryFormat } from '../types';

type Props = QueryEditorProps<DataSource, MyQuery, MyDataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
  const onFormatChange = (value: SelectableValue<QueryFormat>) => {
    onChange({ ...query, format: value.value ?? 'table' });
    onRunQuery();
  };

  const onQueryTextChange = (event: ChangeEvent<HTMLTextAreaElement>) => {
    onChange({ ...query, queryText: event.target.value });
  };

  const onQueryTextBlur = () => {
    // Execute query when user finishes editing
    if (query.queryText && query.queryText.trim().length > 0) {
      onRunQuery();
    }
  };

  const { queryText } = query;
  const currentFormat: QueryFormat = query.format ?? 'table';

  return (
    <>
      <InlineField label="Format" labelWidth={14} tooltip="Table returns raw rows. Time series returns Grafana wide series.">
        <Select
          inputId="query-editor-format"
          value={{ label: currentFormat === 'table' ? 'Table' : 'Time series', value: currentFormat }}
          options={[
            { label: 'Table', value: 'table' },
            { label: 'Time series', value: 'time_series' },
          ]}
          onChange={onFormatChange}
          width={24}
        />
      </InlineField>
      <InlineField label="SQL Query" labelWidth={14} grow tooltip="Enter your SQL query">
        <TextArea
          id="query-editor-sql"
          onChange={onQueryTextChange}
          onBlur={onQueryTextBlur}
          value={queryText || ''}
          required
          placeholder={`SELECT
  $__time(INTERVAL_START),
  USERS_AVG,
  USERS_MAX,
  CLUSTER_NAME
FROM EXA_USAGE_HOURLY
WHERE $__timeFilter(INTERVAL_START)
ORDER BY INTERVAL_START;`}
          rows={5}
        />
      </InlineField>
    </>
  );
}
