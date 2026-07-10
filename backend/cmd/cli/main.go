package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

var (
	baseURL  string
	apiToken string
)

func main() {
	// Load config from env or flags
	baseURL = os.Getenv("SHELLY_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	apiToken = os.Getenv("SHELLY_TOKEN")

	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	switch args[0] {
	case "asset":
		handleAsset(args[1:])
	case "exec":
		handleExec(args[1:])
	case "upload":
		handleUpload(args[1:])
	case "download":
		handleDownload(args[1:])
	case "list":
		handleAsset([]string{"list"})
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", args[0])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Shelly CLI - SSH Terminal Manager

Usage:
  shelly-cli <command> [options]

Commands:
  asset list              List all assets
  asset get <id>          Get asset details
  asset create            Create a new asset (reads JSON from stdin)
  asset delete <id>       Delete an asset

  exec <asset_id> <cmd>   Execute command on asset
  exec --batch <cmd>      Execute on multiple assets (reads IDs from stdin)

  upload <asset_id> <local_path> <remote_path>   Upload file to asset
  download <asset_id> <remote_path> <local_path>  Download file from asset

Environment:
  SHELLY_URL    Server URL (default: http://localhost:8080)
  SHELLY_TOKEN  API Token for authentication

Output format: JSON`)
}

func handleAsset(args []string) {
	if len(args) == 0 {
		args = []string{"list"}
	}

	switch args[0] {
	case "list":
		data := apiGet("/api/assets")
		fmt.Println(string(data))

	case "get":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: asset get <id>")
			os.Exit(1)
		}
		data := apiGet("/api/assets/" + args[1])
		fmt.Println(string(data))

	case "create":
		// Read JSON from stdin
		var body bytes.Buffer
		io.Copy(&body, os.Stdin)
		data := apiPost("/api/assets", body.Bytes())
		fmt.Println(string(data))

	case "delete":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "Usage: asset delete <id>")
			os.Exit(1)
		}
		data := apiDelete("/api/assets/" + args[1])
		fmt.Println(string(data))

	default:
		fmt.Fprintf(os.Stderr, "Unknown asset command: %s\n", args[0])
		os.Exit(1)
	}
}

func handleExec(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: exec <asset_id> <command>")
		os.Exit(1)
	}

	if args[0] == "--batch" {
		// Batch execution
		cmd := strings.Join(args[1:], " ")
		// Read asset IDs from stdin
		fmt.Fprintln(os.Stderr, "Enter asset IDs (one per line, EOF to end):")
		var ids []int
		var id int
		for {
			_, err := fmt.Scanf("%d", &id)
			if err != nil {
				break
			}
			ids = append(ids, id)
		}

		body := map[string]interface{}{
			"asset_ids": ids,
			"command":   cmd,
		}
		jsonBody, _ := json.Marshal(body)
		data := apiPost("/api/batch/exec", jsonBody)
		fmt.Println(string(data))
	} else {
		assetID := args[0]
		cmd := strings.Join(args[1:], " ")

		body := map[string]interface{}{
			"asset_ids": []string{assetID},
			"command":   cmd,
		}
		jsonBody, _ := json.Marshal(body)
		data := apiPost("/api/batch/exec", jsonBody)
		fmt.Println(string(data))
	}
}

func handleUpload(args []string) {
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: upload <asset_id> <local_path> <remote_path>")
		os.Exit(1)
	}

	assetID := args[0]
	localPath := args[1]
	remotePath := args[2]

	// Multipart upload
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	file, err := os.Open(localPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating form: %v\n", err)
		os.Exit(1)
	}
	io.Copy(part, file)
	writer.WriteField("path", remotePath)
	writer.Close()

	req, _ := http.NewRequest("POST", baseURL+"/api/sftp/"+assetID+"/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-API-Token", apiToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	fmt.Println(string(data))
}

func handleDownload(args []string) {
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: download <asset_id> <remote_path> <local_path>")
		os.Exit(1)
	}

	assetID := args[0]
	remotePath := args[1]
	localPath := args[2]

	data := apiGet("/api/sftp/" + assetID + "/download?path=" + remotePath)
	if err := os.WriteFile(localPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Downloaded to %s\n", localPath)
}

// HTTP helpers
func apiGet(path string) []byte {
	req, _ := http.NewRequest("GET", baseURL+path, nil)
	req.Header.Set("X-API-Token", apiToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data
}

func apiPost(path string, body []byte) []byte {
	req, _ := http.NewRequest("POST", baseURL+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Token", apiToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data
}

func apiDelete(path string) []byte {
	req, _ := http.NewRequest("DELETE", baseURL+path, nil)
	req.Header.Set("X-API-Token", apiToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return data
}
