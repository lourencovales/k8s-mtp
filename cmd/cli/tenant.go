package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func tenantCmd(configPath string, args []string) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: k8s-mtp tenant <list|create|get|update|delete>\n")
		os.Exit(1)
	}

	switch args[0] {
	case "list":
		listTenants(cfg)
	case "create":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: k8s-mtp tenant <list|create|get|update|delete>\n")
			os.Exit(1)
		}
		createTenant(cfg, args[1])
	case "get":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: k8s-mtp tenant <list|create|get|update|delete>\n")
			os.Exit(1)
		}
		getTenant(cfg, args[1])
	case "update":
		if len(args) < 3 {
			fmt.Fprintf(os.Stderr, "Usage: k8s-mtp tenant <list|create|get|update|delete>\n")
			os.Exit(1)
		}
		updateTenant(cfg, args[1], args[2])
	case "delete":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "Usage: k8s-mtp tenant <list|create|get|update|delete>\n")
			os.Exit(1)
		}
		deleteTenant(cfg, args[1])
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func listTenants(cfg *Config) {
	resp, err := apiGet(cfg, "/api/v1/tenants")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	handleResponse(resp)
	var tenants []map[string]any
	if err = json.NewDecoder(resp.Body).Decode(&tenants); err != nil {
		fmt.Fprintf(os.Stderr, "Decode failed: %v\n", err)
		os.Exit(1)
	}
	pretty, err := json.MarshalIndent(tenants, "", "	")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Marshaling failed: %v", err)
		os.Exit(1)
	}
	fmt.Println(string(pretty))
}

func createTenant(cfg *Config, arg string) {
	body := strings.NewReader(arg)
	resp, err := apiPost(cfg, "/api/v1/tenants", body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	handleResponse(resp)
	var tenant map[string]any
	if err = json.NewDecoder(resp.Body).Decode(&tenant); err != nil {
		fmt.Fprintf(os.Stderr, "Decode failed: %v\n", err)
		os.Exit(1)
	}
	pretty, err := json.MarshalIndent(tenant, "", "	")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Marshaling failed: %v", err)
		os.Exit(1)
	}
	fmt.Println(string(pretty))
}

func getTenant(cfg *Config, arg string) {
	resp, err := apiGet(cfg, "/api/v1/tenants/"+arg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	handleResponse(resp)
	var tenant map[string]any
	if err = json.NewDecoder(resp.Body).Decode(&tenant); err != nil {
		fmt.Fprintf(os.Stderr, "Decode failed: %v\n", err)
		os.Exit(1)
	}
	pretty, err := json.MarshalIndent(tenant, "", "	")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Marshaling failed: %v", err)
		os.Exit(1)
	}
	fmt.Println(string(pretty))
}

func updateTenant(cfg *Config, arg1, arg2 string) {
	body := strings.NewReader(arg2)
	resp, err := apiPut(cfg, "/api/v1/tenants/"+arg1, body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	handleResponse(resp)
	var tenant map[string]any
	if err = json.NewDecoder(resp.Body).Decode(&tenant); err != nil {
		fmt.Fprintf(os.Stderr, "Decode failed: %v\n", err)
		os.Exit(1)
	}
	pretty, err := json.MarshalIndent(tenant, "", "	")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Marshaling failed: %v", err)
		os.Exit(1)
	}
	fmt.Println(string(pretty))
}

func deleteTenant(cfg *Config, arg string) {
	resp, err := apiDelete(cfg, "/api/v1/tenants/"+arg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	handleResponse(resp)
	fmt.Println("Deleted")
}
