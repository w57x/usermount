package main

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Domain     string `yaml:"domain"`
	Name       string `yaml:"name"`
	Port       string `yaml:"port"`
	DBPath     string `yaml:"db_path"`
	SMTPHost   string `yaml:"smtp_host"`
	SMTPPort   string `yaml:"smtp_port"`
	FromEmail  string `yaml:"from_email"`
	ScriptPath string `yaml:"script_path"`
	JWTSecret  string `yaml:"jwt_secret"`
}

var AppConfig Config

func loadConfig(path string) {
	file, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to read config file at %s: %v", path, err)
	}

	err = yaml.Unmarshal(file, &AppConfig)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to parse config file: %v", err)
	}

	if AppConfig.Domain == "" {
		log.Fatalf("CRITICAL: 'domain' parameter is not specified in config. Application cannot start.")
	}

	// Set fallbacks for optional parameters
	if AppConfig.Name == "" {
		AppConfig.Name = "Usermount"
	}
	if AppConfig.Port == "" {
		AppConfig.Port = "3000"
	}
	if AppConfig.DBPath == "" {
		AppConfig.DBPath = "./usermount.db"
	}
	if AppConfig.SMTPHost == "" {
		AppConfig.SMTPHost = "127.0.0.1"
	}
	if AppConfig.SMTPPort == "" {
		AppConfig.SMTPPort = "25"
	}
	if AppConfig.ScriptPath == "" {
		AppConfig.ScriptPath = "./create_user.sh"
	}
	if AppConfig.JWTSecret == "" {
		AppConfig.JWTSecret = "super-secret-key-change-me"
	}
}
