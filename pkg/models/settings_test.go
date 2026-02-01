package models

import (
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

func TestLoadPluginSettings_Valid(t *testing.T) {
	source := backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"databaseHost":"exa.example.com","databasePort":"8563","user":"sys","schema":"analytics","databaseInsecureSkipVerify":false}`),
		DecryptedSecureJSONData: map[string]string{
			"password": "secret",
		},
	}

	settings, err := LoadPluginSettings(source)
	if err != nil {
		t.Fatalf("expected valid settings, got error: %v", err)
	}

	if settings.DatabaseHost != "exa.example.com" {
		t.Fatalf("unexpected host: %s", settings.DatabaseHost)
	}
	if settings.Port != 8563 {
		t.Fatalf("unexpected port: %d", settings.Port)
	}
	if settings.User != "sys" {
		t.Fatalf("unexpected user: %s", settings.User)
	}
	if settings.Schema != "analytics" {
		t.Fatalf("unexpected schema: %s", settings.Schema)
	}
	if settings.Secrets.Password != "secret" {
		t.Fatalf("unexpected password: %s", settings.Secrets.Password)
	}
}

func TestLoadPluginSettings_TrimmedSchema(t *testing.T) {
	source := backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"databaseHost":"exa.example.com","databasePort":"8563","user":"sys","schema":"  analytics  "}`),
	}

	settings, err := LoadPluginSettings(source)
	if err != nil {
		t.Fatalf("expected valid settings, got error: %v", err)
	}

	if settings.Schema != "analytics" {
		t.Fatalf("unexpected schema: %s", settings.Schema)
	}
}

func TestLoadPluginSettings_InvalidPortRange(t *testing.T) {
	source := backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"databaseHost":"exa.example.com","databasePort":"70000","user":"sys"}`),
	}

	_, err := LoadPluginSettings(source)
	if err == nil {
		t.Fatal("expected error for out-of-range port")
	}
}

func TestLoadPluginSettings_MissingHost(t *testing.T) {
	source := backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"databasePort":"8563","user":"sys"}`),
	}

	_, err := LoadPluginSettings(source)
	if err == nil {
		t.Fatal("expected error when host is missing")
	}
}

func TestLoadPluginSettings_MissingUser(t *testing.T) {
	source := backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"databaseHost":"exa.example.com","databasePort":"8563"}`),
	}

	_, err := LoadPluginSettings(source)
	if err == nil {
		t.Fatal("expected error when user is missing")
	}
}
