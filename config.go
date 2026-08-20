package main

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type ServiceConfig struct {
	Name string `yaml:"name"`
	Goto string `yaml:"goto"`
	Icon string `yaml:"icon"`
}

func (s *ServiceConfig) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.MappingNode {
		type plain ServiceConfig
		return value.Decode((*plain)(s))
	}
	if value.Kind == yaml.SequenceNode {
		for _, item := range value.Content {
			if item.Kind == yaml.MappingNode {
				for i := 0; i < len(item.Content); i += 2 {
					key := item.Content[i].Value
					val := item.Content[i+1].Value
					switch key {
					case "name":
						s.Name = val
					case "goto":
						s.Goto = val
					case "icon":
						s.Icon = val
					}
				}
			}
		}
		return nil
	}
	return nil
}

type Config struct {
	Domain                   string                   `yaml:"domain"`
	Name                     string                   `yaml:"name"`
	Port                     string                   `yaml:"port"`
	DBPath                   string                   `yaml:"db_path"`
	SMTPHost                 string                   `yaml:"smtp_host"`
	SMTPPort                 string                   `yaml:"smtp_port"`
	SMTPUser                 string                   `yaml:"smtp_user"`
	SMTPPassword             string                   `yaml:"smtp_password"`
	SMTPSkipVerify           bool                     `yaml:"smtp_skip_verify"`
	FromEmail                string                   `yaml:"from_email"`
	ScriptPath               string                   `yaml:"script_path"`
	DeleteScriptPath         string                   `yaml:"delete_script_path"`
	UpdatePasswordScriptPath string                   `yaml:"update_password_script_path"`
	JWTSecret                string                   `yaml:"jwt_secret"`
	CookieSecure             *bool                    `yaml:"cookie_secure"`
	Services                 map[string]ServiceConfig `yaml:"services"`
}

var AppConfig Config

func (c *Config) IsCookieSecure() bool {
	if c.CookieSecure != nil {
		return *c.CookieSecure
	}
	return true
}

func loadConfig(path string) {
	file, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to read config file at %s: %v", path, err)
	}

	err = yaml.Unmarshal(file, &AppConfig)
	if err != nil {
		log.Fatalf("CRITICAL: Failed to parse config file: %v", err)
	}

	if env := os.Getenv("USERMOUNT_DOMAIN"); env != "" {
		AppConfig.Domain = env
	}
	if env := os.Getenv("USERMOUNT_PORT"); env != "" {
		AppConfig.Port = env
	}
	if env := os.Getenv("USERMOUNT_NAME"); env != "" {
		AppConfig.Name = env
	}
	if env := os.Getenv("USERMOUNT_DB_PATH"); env != "" {
		AppConfig.DBPath = env
	}
	if env := os.Getenv("USERMOUNT_SMTP_HOST"); env != "" {
		AppConfig.SMTPHost = env
	}
	if env := os.Getenv("USERMOUNT_SMTP_PORT"); env != "" {
		AppConfig.SMTPPort = env
	}
	if env := os.Getenv("USERMOUNT_SMTP_USER"); env != "" {
		AppConfig.SMTPUser = env
	}
	if env := os.Getenv("USERMOUNT_SMTP_PASSWORD"); env != "" {
		AppConfig.SMTPPassword = env
	}
	if env := os.Getenv("USERMOUNT_FROM_EMAIL"); env != "" {
		AppConfig.FromEmail = env
	}
	if env := os.Getenv("USERMOUNT_SCRIPT_PATH"); env != "" {
		AppConfig.ScriptPath = env
	}
	if env := os.Getenv("USERMOUNT_DELETE_SCRIPT_PATH"); env != "" {
		AppConfig.DeleteScriptPath = env
	}
	if env := os.Getenv("USERMOUNT_UPDATE_PASSWORD_SCRIPT_PATH"); env != "" {
		AppConfig.UpdatePasswordScriptPath = env
	}
	if env := os.Getenv("USERMOUNT_JWT_SECRET"); env != "" {
		AppConfig.JWTSecret = env
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
	if AppConfig.DeleteScriptPath == "" {
		AppConfig.DeleteScriptPath = "./delete_user.sh"
	}
	if AppConfig.UpdatePasswordScriptPath == "" {
		AppConfig.UpdatePasswordScriptPath = "./update_user_pass.sh"
	}
	if AppConfig.JWTSecret == "" {
		AppConfig.JWTSecret = "super-secret-key-change-me"
	}
}
