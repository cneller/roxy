package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRoxyJSON_ParsesPortField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roxy.json")

	content := `{
	  "services": {
	    "api": {
	      "cmd": "npm run dev",
	      "port": 4000,
	      "tls": true,
	      "listen-port": 9000
	    }
	  }
	}`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write roxy.json: %v", err)
	}

	cfg, err := LoadRoxyJSON(dir)
	if err != nil {
		t.Fatalf("LoadRoxyJSON returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}

	svc, ok := cfg.Services["api"]
	if !ok {
		t.Fatal("expected service 'api' to exist")
	}

	if svc.Port != 4000 {
		t.Fatalf("Port = %d, want 4000", svc.Port)
	}
	if !svc.TLS {
		t.Fatal("expected tls=true")
	}
	if svc.ListenPort != 9000 {
		t.Fatalf("ListenPort = %d, want 9000", svc.ListenPort)
	}
}

func TestLoadRoxyJSON_RejectsPostField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roxy.json")

	content := `{
	  "services": {
	    "api": {
	      "cmd": "npm run dev",
	      "post": 4000
	    }
	  }
	}`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write roxy.json: %v", err)
	}

	_, err := LoadRoxyJSON(dir)
	if err == nil {
		t.Fatal("expected unknown field error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field \"post\"") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRoxyJSON_RejectsUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roxy.json")

	content := `{
	  "services": {
	    "api": {
	      "cmd": "npm run dev",
	      "unknown": true
	    }
	  }
	}`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write roxy.json: %v", err)
	}

	_, err := LoadRoxyJSON(dir)
	if err == nil {
		t.Fatal("expected unknown field error, got nil")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRoxyJSON_FallsBackToYAML(t *testing.T) {
	dir := t.TempDir()
	content := `services:
  api:
    cmd: npm run dev
    port: 4000
    tls: true
    listen-port: 9000
`
	if err := os.WriteFile(filepath.Join(dir, "roxy.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("write roxy.yaml: %v", err)
	}

	cfg, err := LoadRoxyJSON(dir)
	if err != nil {
		t.Fatalf("LoadRoxyJSON returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config from roxy.yaml, got nil")
	}

	svc, ok := cfg.Services["api"]
	if !ok {
		t.Fatal("expected service 'api' to exist")
	}
	if svc.Port != 4000 || !svc.TLS || svc.ListenPort != 9000 {
		t.Fatalf("yaml fields not parsed: %+v", svc)
	}
}

func TestLoadRoxyJSON_PrefersJSONOverYAML(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "roxy.json"),
		[]byte(`{"services":{"fromjson":{"cmd":"echo hi"}}}`), 0644); err != nil {
		t.Fatalf("write roxy.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "roxy.yaml"),
		[]byte("services:\n  fromyaml:\n    cmd: echo hi\n"), 0644); err != nil {
		t.Fatalf("write roxy.yaml: %v", err)
	}

	cfg, err := LoadRoxyJSON(dir)
	if err != nil {
		t.Fatalf("LoadRoxyJSON returned error: %v", err)
	}
	if _, ok := cfg.Services["fromjson"]; !ok {
		t.Fatalf("expected roxy.json to win, got services: %v", cfg.Services)
	}
	if _, ok := cfg.Services["fromyaml"]; ok {
		t.Fatal("roxy.yaml should have been ignored when roxy.json exists")
	}
}

func TestLoadRoxyJSON_RejectsUnknownYAMLFields(t *testing.T) {
	dir := t.TempDir()
	content := `services:
  api:
    cmd: npm run dev
    bogus: true
`
	if err := os.WriteFile(filepath.Join(dir, "roxy.yaml"), []byte(content), 0644); err != nil {
		t.Fatalf("write roxy.yaml: %v", err)
	}

	_, err := LoadRoxyJSON(dir)
	if err == nil {
		t.Fatal("expected unknown field error from yaml, got nil")
	}
}

func TestLoadRoxyJSON_ReturnsNilWhenNoManifest(t *testing.T) {
	cfg, err := LoadRoxyJSON(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config for empty dir, got %+v", cfg)
	}
}
