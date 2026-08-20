package main

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestServiceConfigParsing(t *testing.T) {
	// Test standard map format and list-item format
	yamlContent := `
domain: "test.example.com"
name: "Usermount Test"
services:
  mail:
    - name: Mailbox
    - goto: "https://mail.example.com"
  git:
    name: Forgejo
    goto: "https://git.example.com"
    icon: git-branch
`

	var cfg Config
	err := yaml.Unmarshal([]byte(yamlContent), &cfg)
	if err != nil {
		t.Fatalf("failed to unmarshal services yaml: %v", err)
	}

	if len(cfg.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(cfg.Services))
	}

	mailSvc, ok := cfg.Services["mail"]
	if !ok {
		t.Fatalf("expected 'mail' service to be present")
	}
	if mailSvc.Name != "Mailbox" || mailSvc.Goto != "https://mail.example.com" {
		t.Fatalf("unexpected mail service values: %+v", mailSvc)
	}

	gitSvc, ok := cfg.Services["git"]
	if !ok {
		t.Fatalf("expected 'git' service to be present")
	}
	if gitSvc.Name != "Forgejo" || gitSvc.Goto != "https://git.example.com" || gitSvc.Icon != "git-branch" {
		t.Fatalf("unexpected git service values: %+v", gitSvc)
	}
}

func TestConfigEnvOverrides(t *testing.T) {
	tmpConfig := "./test_config.yaml"
	content := []byte("domain: \"initial.com\"\nport: \"3000\"\n")
	if err := os.WriteFile(tmpConfig, content, 0644); err != nil {
		t.Fatalf("failed to write tmp config: %v", err)
	}
	defer os.Remove(tmpConfig)

	os.Setenv("USERMOUNT_DOMAIN", "override.com")
	os.Setenv("USERMOUNT_PORT", "8080")
	defer os.Unsetenv("USERMOUNT_DOMAIN")
	defer os.Unsetenv("USERMOUNT_PORT")

	loadConfig(tmpConfig)

	if AppConfig.Domain != "override.com" {
		t.Fatalf("expected domain override 'override.com', got '%s'", AppConfig.Domain)
	}
	if AppConfig.Port != "8080" {
		t.Fatalf("expected port override '8080', got '%s'", AppConfig.Port)
	}
}
