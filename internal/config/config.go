package config

import "fmt"

type Config struct {
	ListenAddr string `json:"listen_addr"`
	DBURL string `json:"db_url"`
	LogLevel string `json:"log_level"`
}

func (c *Config) Load(file string) error {
	return fmt.Errorf("not yet implemented")
}
