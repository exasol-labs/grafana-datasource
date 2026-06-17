import React, { ChangeEvent } from 'react';
import { InlineField, Input, SecretInput, InlineSwitch } from '@grafana/ui';
import { DataSourcePluginOptionsEditorProps } from '@grafana/data';
import { ExasolDataSourceOptions, ExasolSecureJsonData } from '../types';

interface Props extends DataSourcePluginOptionsEditorProps<ExasolDataSourceOptions, ExasolSecureJsonData> {}

export function ConfigEditor(props: Props) {
  const { onOptionsChange, options } = props;
  const { jsonData, secureJsonFields, secureJsonData } = options;

  const update = (patch: Partial<ExasolDataSourceOptions>) =>
    onOptionsChange({ ...options, jsonData: { ...jsonData, ...patch } });

  const onDatabaseHostChange = (e: ChangeEvent<HTMLInputElement>) => update({ databaseHost: e.target.value });
  const onDatabasePortChange = (e: ChangeEvent<HTMLInputElement>) => update({ databasePort: e.target.value });
  const onDatabaseInsecureSkipVerifyChange = (e: ChangeEvent<HTMLInputElement>) =>
    update({ databaseInsecureSkipVerify: e.target.checked });
  const onDatabaseCertificateFingerprintChange = (e: ChangeEvent<HTMLInputElement>) =>
    update({ databaseCertificateFingerprint: e.target.value });
  const onUserChange = (e: ChangeEvent<HTMLInputElement>) => update({ user: e.target.value });
  const onSchemaChange = (e: ChangeEvent<HTMLInputElement>) => update({ schema: e.target.value });
  const onMaxOpenConnsChange = (e: ChangeEvent<HTMLInputElement>) => update({ maxOpenConns: e.target.value });
  const onMaxIdleConnsChange = (e: ChangeEvent<HTMLInputElement>) => update({ maxIdleConns: e.target.value });
  const onConnMaxLifetimeChange = (e: ChangeEvent<HTMLInputElement>) =>
    update({ connMaxLifetimeSecs: e.target.value });
  const onQueryTimeoutChange = (e: ChangeEvent<HTMLInputElement>) => update({ queryTimeoutSecs: e.target.value });

  const onPasswordChange = (e: ChangeEvent<HTMLInputElement>) =>
    onOptionsChange({ ...options, secureJsonData: { password: e.target.value } });

  const onResetPassword = () =>
    onOptionsChange({
      ...options,
      secureJsonFields: { ...options.secureJsonFields, password: false },
      secureJsonData: { ...options.secureJsonData, password: '' },
    });

  return (
    <>
      <InlineField label="Host" labelWidth={22} interactive tooltip="Exasol database connection host (e.g., localhost)">
        <Input
          id="config-editor-database-host"
          onChange={onDatabaseHostChange}
          value={jsonData.databaseHost || ''}
          placeholder="localhost"
          width={40}
          required
        />
      </InlineField>
      <InlineField label="Port" labelWidth={22} interactive tooltip="Exasol database connection port (default 8563)">
        <Input
          id="config-editor-database-port"
          onChange={onDatabasePortChange}
          value={jsonData.databasePort || ''}
          placeholder="8563"
          width={40}
        />
      </InlineField>
      <InlineField label="User" labelWidth={22} interactive tooltip="Exasol database username">
        <Input
          id="config-editor-user"
          onChange={onUserChange}
          value={jsonData.user || ''}
          placeholder="sys"
          width={40}
          required
        />
      </InlineField>
      <InlineField label="Schema" labelWidth={22} interactive tooltip="Optional default schema for this datasource session">
        <Input
          id="config-editor-schema"
          onChange={onSchemaChange}
          value={jsonData.schema || ''}
          placeholder="MY_SCHEMA"
          width={40}
        />
      </InlineField>
      <InlineField label="Password" labelWidth={22} interactive tooltip="Exasol database password (stored encrypted)">
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
      <InlineField
        label="Skip TLS verify"
        labelWidth={22}
        interactive
        tooltip="Skip TLS certificate verification (insecure — use only for testing)"
      >
        <InlineSwitch
          id="config-editor-insecure-skip-verify"
          onChange={onDatabaseInsecureSkipVerifyChange}
          value={jsonData.databaseInsecureSkipVerify || false}
        />
      </InlineField>
      <InlineField
        label="Cert fingerprint"
        labelWidth={22}
        interactive
        tooltip="Optional SHA-256 fingerprint of the Exasol server certificate. When set, the driver pins the cert to this fingerprint instead of validating against a CA — useful for self-signed Exasol clusters."
      >
        <Input
          id="config-editor-cert-fingerprint"
          onChange={onDatabaseCertificateFingerprintChange}
          value={jsonData.databaseCertificateFingerprint || ''}
          placeholder="e.g. ABCD1234..."
          width={60}
        />
      </InlineField>
      <InlineField
        label="Max open conns"
        labelWidth={22}
        interactive
        tooltip="Maximum number of open connections to Exasol (default 10)"
      >
        <Input
          id="config-editor-max-open-conns"
          onChange={onMaxOpenConnsChange}
          value={jsonData.maxOpenConns || ''}
          placeholder="10"
          width={20}
        />
      </InlineField>
      <InlineField
        label="Max idle conns"
        labelWidth={22}
        interactive
        tooltip="Maximum number of idle connections in the pool (default 5)"
      >
        <Input
          id="config-editor-max-idle-conns"
          onChange={onMaxIdleConnsChange}
          value={jsonData.maxIdleConns || ''}
          placeholder="5"
          width={20}
        />
      </InlineField>
      <InlineField
        label="Conn max lifetime (s)"
        labelWidth={22}
        interactive
        tooltip="Maximum time a connection may be reused, in seconds (default 14400)"
      >
        <Input
          id="config-editor-conn-max-lifetime"
          onChange={onConnMaxLifetimeChange}
          value={jsonData.connMaxLifetimeSecs || ''}
          placeholder="14400"
          width={20}
        />
      </InlineField>
      <InlineField
        label="Query timeout (s)"
        labelWidth={22}
        interactive
        tooltip="Per-query timeout in seconds (default 60). Long-running user queries are aborted past this."
      >
        <Input
          id="config-editor-query-timeout"
          onChange={onQueryTimeoutChange}
          value={jsonData.queryTimeoutSecs || ''}
          placeholder="60"
          width={20}
        />
      </InlineField>
    </>
  );
}
