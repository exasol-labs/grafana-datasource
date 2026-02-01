package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type PluginSettings struct {
	DatabaseHost       string                `json:"databaseHost"`
	Port               int                   `json:"-"`
	InsecureSkipVerify bool                  `json:"databaseInsecureSkipVerify"`
	Schema             string                `json:"schema"`
	User               string                `json:"user"`
	Secrets            *SecretPluginSettings `json:"-"`
}

type SecretPluginSettings struct {
	Password string `json:"password"`
}

// jsonSettings is used for unmarshaling with port as string
type jsonSettings struct {
	DatabaseHost               string `json:"databaseHost"`
	DatabasePort               string `json:"databasePort"`
	DatabaseInsecureSkipVerify bool   `json:"databaseInsecureSkipVerify"`
	Schema                     string `json:"schema"`
	User                       string `json:"user"`
}

func LoadPluginSettings(source backend.DataSourceInstanceSettings) (*PluginSettings, error) {
	var jsonData jsonSettings
	err := json.Unmarshal(source.JSONData, &jsonData)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal PluginSettings json: %w", err)
	}

	if strings.TrimSpace(jsonData.DatabaseHost) == "" {
		return nil, fmt.Errorf("database host is required")
	}
	if strings.TrimSpace(jsonData.User) == "" {
		return nil, fmt.Errorf("user is required")
	}

	// Parse port from string to int
	port := 8563 // Default port
	if jsonData.DatabasePort != "" {
		parsedPort, err := strconv.Atoi(jsonData.DatabasePort)
		if err != nil {
			return nil, fmt.Errorf("invalid port number: %w", err)
		}
		if parsedPort < 1 || parsedPort > 65535 {
			return nil, fmt.Errorf("invalid port number: must be between 1 and 65535")
		}
		port = parsedPort
	}

	settings := &PluginSettings{
		DatabaseHost:       jsonData.DatabaseHost,
		Port:               port,
		InsecureSkipVerify: jsonData.DatabaseInsecureSkipVerify,
		Schema:             strings.TrimSpace(jsonData.Schema),
		User:               jsonData.User,
		Secrets:            loadSecretPluginSettings(source.DecryptedSecureJSONData),
	}

	return settings, nil
}

func loadSecretPluginSettings(source map[string]string) *SecretPluginSettings {
	return &SecretPluginSettings{
		Password: source["password"],
	}
}
