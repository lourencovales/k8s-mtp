package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"strconv"
)

// Config is a struct that holds the values that configure the app.
type Config struct {
	ListenAddr string `json:"listen_addr"`
	DBURL      string `json:"db_url"`
	LogLevel   string `json:"log_level"`
	LogFormat string `json:"log_format"`
	DBMaxOpenConns int `json:"db_max_open_conns"`
	DBMaxIdleConns int `json:"db_max_idle_conns"`
	DBConnMaxLifetime string `json:"db_conn_max_lifetime"`
}

// Load is responsible for parsing the config for the app. It takes the
// information from either the config file, or the environment, and then it
// returns the Config object or an error.
func Load(configFile string) (*Config, error) {
	var cfg *Config
	var configSource string
	var err error

	// we check what type of config we have
	if configFile != "" {
		if _, err := os.Stat(configFile); err != nil {
			return nil, fmt.Errorf("problem loading config file: %w", err)
		}
		configSource = "file"
	} else {
		configFile = "config.json"
		if _, err := os.Stat(configFile); err == nil {
			configSource = "file"
		} else {
			if _, err := os.Stat(".env"); err == nil {
				configSource = "env"
			}
		}
	}

	switch configSource {
	case "file":
		cfg, err = fileParse(configFile)
	case "env":
		cfg, err = envParse()
	default:
		return nil, fmt.Errorf("no config file found (tried: config.json, .env)")
	}

	if err != nil {
		return nil, fmt.Errorf("problem with config: %w", err)
	}

	// overriding with env variables for prod deployments
	overrideCfg(cfg)

	// we make sure everything is ok
	if err = validateCfg(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// fileParse is a private function for extracting the configuration from a 
// config file.
func fileParse(file string) (*Config, error) {
	var cfg Config

	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

// envParse is a private function for extracting the configuration from an .env
// file.
func envParse() (*Config, error) {
	data, err := os.Open(".env")
	if err != nil {
		return nil, fmt.Errorf("reading .env file: %w", err)
	}

	var cfg Config
	scanner := bufio.NewScanner(data)
	for scanner.Scan() {
		line := scanner.Text()

		if len(line) == 0 {
			continue
		}

		if strings.HasPrefix(line, "#") {
			continue
		}

		result := strings.SplitN(line, "=", 2)
		if len(result) != 2 {
			return nil, fmt.Errorf("line %s is malformed", line)
		}

		key := strings.TrimSpace(result[0])
		value := strings.TrimSpace(result[1])

		switch key {
		case "K8S_MTP_LISTEN_ADDR":
			cfg.ListenAddr = value
		case "K8S_MTP_DB_URL":
			cfg.DBURL = value
		case "K8S_MTP_LOG_LEVEL":
			cfg.LogLevel = value
		case "K8S_MTP_DB_OPEN_CONNS":
			v, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("K8S_MTP_DB_OPEN_CONNS is malformed")
			}
			cfg.DBMaxOpenConns = v
		case "K8S_MTP_DB_IDLE_CONNS":
			v, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("K8S_MTP_DB_IDLE_CONNS is malformed")
			}
			cfg.DBMaxIdleConns = v
		case "K8S_MTP_DB_CONN_MAX_LIFETIME":
			cfg.DBConnMaxLifetime = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning .env: %w", err)
	}

	if err := data.Close(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// overrideCfg is a private function for overriding values coming from the env.
// This is useful for prod environments.
func overrideCfg(cfg *Config) {
	if listenAddr := os.Getenv("K8S_MTP_LISTEN_ADDR"); listenAddr != "" {
		cfg.ListenAddr = listenAddr
	}
	if dburl := os.Getenv("K8S_MTP_DB_URL"); dburl != "" {
		cfg.DBURL = dburl
	}
	if level := os.Getenv("K8S_MTP_LOG_LEVEL"); level != "" {
		cfg.LogLevel = level
	}
	if format := os.Getenv("K8S_MTP_LOG_FORMAT"); format != "" {
		cfg.LogFormat = format
	}
	if openConns := os.Getenv("K8S_MTP_DB_OPEN_CONNS"); openConns != "" {
		v, err := strconv.Atoi(openConns)
		if err != nil {
			panic("conversion of env variable K8S_MTP_DB_OPEN_CONNS")
		}
		cfg.DBMaxOpenConns = v
	}
	if idleConns := os.Getenv("K8S_MTP_DB_IDLE_CONNS"); idleConns != "" {
		v, err := strconv.Atoi(idleConns)
		if err != nil {
			panic("conversion of env variable K8S_MTP_DB_IDLE_CONNS")
		}
		cfg.DBMaxIdleConns = v
	}
	if maxLifetime := os.Getenv("K8S_MTP_DB_CONN_MAX_LIFETIME"); maxLifetime != "" {
		cfg.DBConnMaxLifetime = maxLifetime
	}
}

// validateCfg is a private function for checking if the necessary values for
// operation are present.
func validateCfg(cfg *Config) error {
	if cfg.ListenAddr == "" {
		return fmt.Errorf("ListenAddr is empty")
	}
	if cfg.DBURL == "" {
		return fmt.Errorf("database URL is empty")
	}
	if cfg.LogFormat == "" {
		cfg.LogFormat = "console"
	}
	if cfg.LogFormat != "json" && cfg.LogFormat != "console" {
		return fmt.Errorf("invalid log_format: %s", cfg.LogFormat)
	}

	validLevels := map[string]bool{
		"debug": true, 
		"info": true, 
		"warn": true, 
		"error": true,
	}
	if !validLevels[cfg.LogLevel] {
		return fmt.Errorf("invalid log level: %s", cfg.LogLevel)
	}

	if cfg.DBMaxOpenConns == 0 {
		cfg.DBMaxOpenConns = 10
	}
	if cfg.DBMaxIdleConns == 0 {
		cfg.DBMaxIdleConns = 5
	}
	if cfg.DBConnMaxLifetime == "" {
		cfg.DBConnMaxLifetime = "1h"
	}

	return nil
}

func (c *Config) slogLevel() slog.Level {
	switch strings.ToLower(c.LogLevel) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (c *Config) Handler() slog.Handler {
	var h slog.Handler
	if c.LogFormat == "json" {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level: c.slogLevel(),
		})
	} else {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level: c.slogLevel(),
		})
	}
	return h.WithGroup("app")
}
