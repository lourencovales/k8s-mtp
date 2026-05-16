package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func memberCmd(configPath string, args []string) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: k8s-mtp member <add|list|remove>\n")
		os.Exit(1)
	}

	switch args[0] {
	case "add":
		if len(args) != 4 {
			fmt.Fprintf(os.Stderr, "Usage: k8s-mtp member <add|list|remove>\n")
			os.Exit(1)
		}
		addMember(cfg, args[1], args[2], args[3])
	case "list":
		if len(args) != 2 {
			fmt.Fprintf(os.Stderr, "Usage: k8s-mtp member <add|list|remove>\n")
			os.Exit(1)
		}
		listMembers(cfg, args[1])
	case "remove":
		if len(args) != 3 {
			fmt.Fprintf(os.Stderr, "Usage: k8s-mtp member <add|list|remove>\n")
			os.Exit(1)
		}
		removeMember(cfg, args[1], args[2])
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", args[0])
		os.Exit(1)
	}
}

func addMember(cfg *Config, tid, uid, role string) {
	bodyJSON := fmt.Sprintf(`{"user_id":"%s","role":"%s"}`, uid, role)
	body := strings.NewReader(bodyJSON)
	resp, err := apiPost(cfg, "/api/v1/tenants/"+tid+"/members", body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	handleResponse(resp)
	var member map[string]any
	if err = json.NewDecoder(resp.Body).Decode(&member); err != nil {
		fmt.Fprintf(os.Stderr, "Decoding failed: %v\n", err)
		os.Exit(1)
	}
	pretty, err := json.MarshalIndent(member, "", "	")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Marshaling failed: %v", err)
		os.Exit(1)
	}
	fmt.Println(string(pretty))
}

func listMembers(cfg *Config, tid string) {
	resp, err := apiGet(cfg, "/api/v1/tenants/"+tid+"/members")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	handleResponse(resp)
	var members []map[string]any
	if err = json.NewDecoder(resp.Body).Decode(&members); err != nil {
		fmt.Fprintf(os.Stderr, "Decoding failed: %v\n", err)
		os.Exit(1)
	}
	pretty, err := json.MarshalIndent(members, "", "	")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Marshaling failed: %v", err)
		os.Exit(1)
	}
	fmt.Println(string(pretty))
}

func removeMember(cfg *Config, tid, uid string) {
	resp, err := apiDelete(cfg, "/api/v1/tenants/"+tid+"/members/"+uid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	handleResponse(resp)
	fmt.Println("Removed")
}
