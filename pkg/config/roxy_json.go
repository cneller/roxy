package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	roxyConfigFileJSON = "roxy.json"
	roxyConfigFileYAML = "roxy.yaml"
	maxPortNumber      = 65535
)

// RoxyConfig represents a roxy service manifest (roxy.json or roxy.yaml) with
// multiple service definitions.
type RoxyConfig struct {
	Schema   string                   `json:"$schema,omitempty" yaml:"$schema,omitempty"`
	Services map[string]ServiceConfig `json:"services" yaml:"services"`
}

// ServiceConfig defines a single service in the manifest.
type ServiceConfig struct {
	Cmd        string `json:"cmd" yaml:"cmd"`
	Name       string `json:"name,omitempty" yaml:"name,omitempty"`
	Port       int    `json:"port,omitempty" yaml:"port,omitempty"`
	TLS        bool   `json:"tls,omitempty" yaml:"tls,omitempty"`
	ListenPort int    `json:"listen-port,omitempty" yaml:"listen-port,omitempty"`
	Public     bool   `json:"public,omitempty" yaml:"public,omitempty"`
}

// LoadRoxyJSON reads the service manifest from the given directory. It prefers
// roxy.json and falls back to roxy.yaml, so existing YAML manifests keep
// working. Returns nil, nil if neither file exists.
func LoadRoxyJSON(dir string) (*RoxyConfig, error) {
	if data, err := os.ReadFile(filepath.Join(dir, roxyConfigFileJSON)); err == nil {
		return parseRoxyJSON(data)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	if data, err := os.ReadFile(filepath.Join(dir, roxyConfigFileYAML)); err == nil {
		return parseRoxyYAML(data)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	return nil, nil
}

func parseRoxyJSON(data []byte) (*RoxyConfig, error) {
	var cfg RoxyConfig
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", roxyConfigFileJSON, err)
	}

	if err := validateRoxyConfig(cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func parseRoxyYAML(data []byte) (*RoxyConfig, error) {
	var cfg RoxyConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", roxyConfigFileYAML, err)
	}

	if err := validateRoxyConfig(cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func validateRoxyConfig(cfg RoxyConfig) error {
	for name, svc := range cfg.Services {
		if strings.TrimSpace(svc.Cmd) == "" {
			return fmt.Errorf("service %q: cmd is required", name)
		}

		if svc.Port != 0 {
			if err := validatePort(name, "port", svc.Port); err != nil {
				return err
			}
		}

		if svc.ListenPort != 0 {
			if err := validatePort(name, "listen-port", svc.ListenPort); err != nil {
				return err
			}
		}
	}

	return nil
}

func validatePort(serviceName, field string, value int) error {
	if value < 1 || value > maxPortNumber {
		return fmt.Errorf("service %q: %s must be between 1 and %d", serviceName, field, maxPortNumber)
	}
	return nil
}
