// Package models holds the datasource configuration types persisted by Grafana
// and the unmarshaling logic that converts the JSON shape sent from the
// ConfigEditor into typed settings consumed by the backend.
package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

// Defaults applied when the corresponding UI field is left blank.
const (
	DefaultExasolPort          = 8563
	DefaultMaxOpenConns        = 10
	DefaultMaxIdleConns        = 5
	DefaultConnMaxLifetimeSecs = 14400
	DefaultQueryTimeoutSecs    = 60

	maxAllowedConns            = 100
	maxAllowedQueryTimeoutSecs = 3600
	maxAllowedConnLifetimeSecs = 86400
)

// PluginSettings is the parsed, validated form of the per-datasource
// configuration coming from Grafana.
type PluginSettings struct {
	DatabaseHost           string                `json:"databaseHost"`
	Port                   int                   `json:"-"`
	InsecureSkipVerify     bool                  `json:"databaseInsecureSkipVerify"`
	CertificateFingerprint string                `json:"databaseCertificateFingerprint"`
	Schema                 string                `json:"schema"`
	User                   string                `json:"user"`
	MaxOpenConns           int                   `json:"-"`
	MaxIdleConns           int                   `json:"-"`
	ConnMaxLifetimeSecs    int                   `json:"-"`
	QueryTimeoutSecs       int                   `json:"-"`
	Secrets                *SecretPluginSettings `json:"-"`
}

// SecretPluginSettings holds fields sourced from Grafana's encrypted
// secureJsonData. Never logged or returned to the frontend.
type SecretPluginSettings struct {
	Password string `json:"password"`
}

// jsonSettings is used for unmarshaling with port and pool fields as string (UI sends strings).
type jsonSettings struct {
	DatabaseHost               string `json:"databaseHost"`
	DatabasePort               string `json:"databasePort"`
	DatabaseInsecureSkipVerify bool   `json:"databaseInsecureSkipVerify"`
	DatabaseCertificateFP      string `json:"databaseCertificateFingerprint"`
	Schema                     string `json:"schema"`
	User                       string `json:"user"`
	MaxOpenConns               string `json:"maxOpenConns"`
	MaxIdleConns               string `json:"maxIdleConns"`
	ConnMaxLifetimeSecs        string `json:"connMaxLifetimeSecs"`
	QueryTimeoutSecs           string `json:"queryTimeoutSecs"`
}

// LoadPluginSettings parses Grafana's per-instance JSON + decrypted secrets
// into a validated PluginSettings. Returns an error if a required field is
// missing or an integer field is out of its allowed range.
func LoadPluginSettings(source backend.DataSourceInstanceSettings) (*PluginSettings, error) {
	var jsonData jsonSettings
	if err := json.Unmarshal(source.JSONData, &jsonData); err != nil {
		return nil, fmt.Errorf("could not unmarshal PluginSettings json: %w", err)
	}

	if strings.TrimSpace(jsonData.DatabaseHost) == "" {
		return nil, fmt.Errorf("database host is required")
	}
	if strings.TrimSpace(jsonData.User) == "" {
		return nil, fmt.Errorf("user is required")
	}

	port, err := parseIntField(jsonData.DatabasePort, DefaultExasolPort, 1, 65535, "port")
	if err != nil {
		return nil, err
	}
	maxOpen, err := parseIntField(jsonData.MaxOpenConns, DefaultMaxOpenConns, 1, maxAllowedConns, "maxOpenConns")
	if err != nil {
		return nil, err
	}
	maxIdle, err := parseIntField(jsonData.MaxIdleConns, DefaultMaxIdleConns, 0, maxAllowedConns, "maxIdleConns")
	if err != nil {
		return nil, err
	}
	connLifetime, err := parseIntField(jsonData.ConnMaxLifetimeSecs, DefaultConnMaxLifetimeSecs, 0, maxAllowedConnLifetimeSecs, "connMaxLifetimeSecs")
	if err != nil {
		return nil, err
	}
	queryTimeout, err := parseIntField(jsonData.QueryTimeoutSecs, DefaultQueryTimeoutSecs, 1, maxAllowedQueryTimeoutSecs, "queryTimeoutSecs")
	if err != nil {
		return nil, err
	}

	return &PluginSettings{
		DatabaseHost:           jsonData.DatabaseHost,
		Port:                   port,
		InsecureSkipVerify:     jsonData.DatabaseInsecureSkipVerify,
		CertificateFingerprint: strings.TrimSpace(jsonData.DatabaseCertificateFP),
		Schema:                 strings.TrimSpace(jsonData.Schema),
		User:                   jsonData.User,
		MaxOpenConns:           maxOpen,
		MaxIdleConns:           maxIdle,
		ConnMaxLifetimeSecs:    connLifetime,
		QueryTimeoutSecs:       queryTimeout,
		Secrets:                loadSecretPluginSettings(source.DecryptedSecureJSONData),
	}, nil
}

func parseIntField(raw string, def, lo, hi int, name string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	if v < lo || v > hi {
		return 0, fmt.Errorf("invalid %s: must be between %d and %d", name, lo, hi)
	}
	return v, nil
}

func loadSecretPluginSettings(source map[string]string) *SecretPluginSettings {
	return &SecretPluginSettings{
		Password: source["password"],
	}
}
