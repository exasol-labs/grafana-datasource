import React, { ChangeEvent, useRef } from 'react';
import { InlineField, Select, TextArea } from '@grafana/ui';
import { QueryEditorProps, SelectableValue } from '@grafana/data';
import { DataSource } from '../datasource';
import { ExasolDataSourceOptions, ExasolQuery, QueryFormat } from '../types';

type Props = QueryEditorProps<DataSource, ExasolQuery, ExasolDataSourceOptions>;

export function QueryEditor({ query, onChange, onRunQuery }: Props) {
  const lastRunText = useRef<string | undefined>(query.queryText);

  const onFormatChange = (value: SelectableValue<QueryFormat>) => {
    onChange({ ...query, format: value.value ?? 'table' });
    // Defer so React flushes the state update before Grafana re-runs.
    setTimeout(onRunQuery, 0);
  };

  const onQueryTextChange = (event: ChangeEvent<HTMLTextAreaElement>) => {
    onChange({ ...query, queryText: event.target.value });
  };

  const onQueryTextBlur = () => {
    const text = query.queryText?.trim() ?? '';
    if (text.length === 0 || text === lastRunText.current?.trim()) {
      return;
    }
    lastRunText.current = query.queryText;
    onRunQuery();
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
