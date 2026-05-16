package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/term"
)

func loginCmd(configPath string) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config")
		os.Exit(1)
	}
	var username string
	_, _ = fmt.Print("Username: ")
	_, err = fmt.Scanln(&username)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading the username")
		os.Exit(1)
	}
	_, _ = fmt.Print("Password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error reading password:", err)
		os.Exit(1)
	}
	fmt.Println()

	data := url.Values{
		"grant_type": {"password"},
		"username":   {username},
		"password":   {string(password)},
		"scope":      {"openid"},
	}

	req, err := http.NewRequest("POST", cfg.DexURL+"/token", strings.NewReader(data.Encode()))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error requesting the token:", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(cfg.ClientID+":")))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error requesting auth:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Expected status 200, instead got %d", resp.StatusCode)
		os.Exit(1)
	}

	type token struct {
		IDToken string `json:"id_token"`
	}
	var tokenResp token
	if err = json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		fmt.Fprintln(os.Stderr, "Error decoding the json response")
		os.Exit(1)
	}

	cfg.Token = tokenResp.IDToken
	if err = saveConfig(configPath, cfg); err != nil {
		fmt.Fprintln(os.Stderr, "Error saving config")
		os.Exit(1)
	} else {
		fmt.Fprintf(os.Stdout, "Configuration saved to %s", configPath)
	}
}
