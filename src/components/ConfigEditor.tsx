import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, InlineSwitch } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { MyDataSourceOptions, MySecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<MyDataSourceOptions, MySecureJsonData> {}

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const onDatabaseHostChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        databaseHost: event.target.value,
      },
    });
  };

  const onDatabasePortChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        databasePort: event.target.value,
      },
    });
  };
  
  const onDatabaseInsecureSkipVerifyChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        databaseInsecureSkipVerify: event.target.checked,
      },
    });
  };


  const onUserChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        user: event.target.value,
      },
    });
  };

  const onSchemaChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      jsonData: {
        ...jsonData,
        schema: event.target.value,
      },
    });
  };

  // Secure field (only sent to the backend)
  const onPasswordChange = (event: ChangeEvent<HTMLInputElement>) => {
    onOptionsChange({
      ...options,
      secureJsonData: {
        password: event.target.value,
      },
    });
  };

  const onResetPassword = () => {
    onOptionsChange({
      ...options,
      secureJsonFields: {
        ...options.secureJsonFields,
        password: false,
      },
      secureJsonData: {
        ...options.secureJsonData,
        password: '',
      },
    });
  };

  return (
    <>
      <InlineField label="Host" labelWidth={14} interactive tooltip={'Exasol database connection host (e.g., localhost)'}>
        <Input
          id="config-editor-database-host"
          onChange={onDatabaseHostChange}
          value={jsonData.databaseHost || ''}
          placeholder="localhost"
          width={40}
          required
        />
      </InlineField>
      <InlineField label="Port" labelWidth={14} interactive tooltip={'Exasol database connection port (e.g., 8563)'}>
        <Input
          id="config-editor-database-port"
          onChange={onDatabasePortChange}
          value={jsonData.databasePort || ''}
          placeholder="8563"
          width={40}
          required
        />
      </InlineField>
      <InlineField label="User" labelWidth={14} interactive tooltip={'Exasol database username'}>
        <Input
          id="config-editor-user"
          onChange={onUserChange}
          value={jsonData.user || ''}
          placeholder="sys"
          width={40}
          required
        />
      </InlineField>
      <InlineField label="Schema" labelWidth={14} interactive tooltip={'Optional default schema to open for this datasource session'}>
        <Input
          id="config-editor-schema"
          onChange={onSchemaChange}
          value={jsonData.schema || ''}
          placeholder="MY_SCHEMA"
          width={40}
        />
      </InlineField>
      <InlineField label="Password" labelWidth={14} interactive tooltip={'Exasol database password (securely stored)'}>
        <SecretInput
          required
          id="config-editor-password"
          isConfigured={secureJsonFields.password}
          value={secureJsonData?.password || ''}
          placeholder="Enter your password"
          width={40}
          onReset={onResetPassword}
          onChange={onPasswordChange}
        />
      </InlineField>
      <InlineField label="Skip TLS" labelWidth={14} interactive tooltip={'Skip TLS certificate verification (insecure, use only for testing)'}>
        <InlineSwitch
          id="config-editor-insecure-skip-verify"
          onChange={onDatabaseInsecureSkipVerifyChange}
          value={jsonData.databaseInsecureSkipVerify || false}
        />
      </InlineField>
    </>
  );
}
