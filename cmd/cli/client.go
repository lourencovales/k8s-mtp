package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type Config struct {
	DexURL   string `json:"dex_url"`
	APIURL   string `json:"api_url"`
	ClientID string `json:"client_id"`
	Token    string `json:"token"`
}

func loadConfig(path string) (*Config, error) {
	file, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		config := &Config{ClientID: "k8s-mtp-cli"}
		if err := saveConfig(path, config); err != nil {
			return nil, err
		}
		return config, fmt.Errorf("config created at %s, edit the file and set api_url and dex_url", path)
	} else if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	if config.APIURL == "" || config.DexURL == "" {
		return nil, fmt.Errorf("issue with the imported configuration, check the fields")
	}

	return &config, nil
}

func saveConfig(path string, config *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	marshaled, err := json.MarshalIndent(config, "", "	")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, marshaled, 0o600); err != nil {
		return err
	}
	return nil
}

func apiGet(config *Config, path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", config.APIURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+config.Token)
	return http.DefaultClient.Do(req)
}

func apiPost(config *Config, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", config.APIURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+config.Token)
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

func apiPut(config *Config, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("PUT", config.APIURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+config.Token)
	req.Header.Set("Content-Type", "application/json")
	return http.DefaultClient.Do(req)
}

func apiDelete(config *Config, path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", config.APIURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+config.Token)
	return http.DefaultClient.Do(req)
}

func handleResponse(resp *http.Response) {
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		fmt.Fprintln(os.Stderr, "Token expired. Run: k8s-mtp login")
		os.Exit(1)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&e)
		if e.Error != "" {
			fmt.Fprintf(os.Stderr, "Error: %s\n", e.Error)
		} else {
			fmt.Fprintf(os.Stderr, "Request failed: %d\n", resp.StatusCode)
		}
		os.Exit(1)
	}
}
