package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultServer = "http://127.0.0.1:8080"

var resourceHTTPClient = http.DefaultClient

type resourceHeader struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind" yaml:"kind"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		usage(stderr)
		return fmt.Errorf("command is required")
	}
	switch args[0] {
	case "apply":
		return runApply(args[1:], stdout)
	case "get":
		return runGet(args[1:], stdout)
	case "delete":
		return runDelete(args[1:], stdout)
	case "completion":
		return runCompletion(args[1:], stdout)
	case "help", "-h", "--help":
		usage(stdout)
		return nil
	default:
		usage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runGet(args []string, stdout io.Writer) error {
	kind, name, server, output, err := parseGetArgs(args)
	if err != nil {
		return err
	}
	endpoint, err := endpointForKind(kind)
	if err != nil {
		return err
	}
	requestURL := strings.TrimRight(server, "/") + endpoint
	if name != "" {
		requestURL += "/" + url.PathEscape(name)
	}
	status, responseBody, err := sendResourceRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		resource := kind
		if name != "" {
			resource += "/" + name
		}
		return fmt.Errorf("get %s failed: %s: %s", resource, http.StatusText(status), strings.TrimSpace(string(responseBody)))
	}
	formatted, err := formatResourceOutput(responseBody, output)
	if err != nil {
		return fmt.Errorf("format get response: %w", err)
	}
	_, err = stdout.Write(formatted)
	return err
}

func parseGetArgs(args []string) (string, string, string, string, error) {
	server := defaultServer
	output := "json"
	positionals := make([]string, 0, 2)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--server":
			if i+1 >= len(args) {
				return "", "", "", "", fmt.Errorf("get --server requires a URL")
			}
			i++
			server = args[i]
		case strings.HasPrefix(args[i], "--server="):
			server = strings.TrimPrefix(args[i], "--server=")
		case args[i] == "-o" || args[i] == "--output":
			if i+1 >= len(args) {
				return "", "", "", "", fmt.Errorf("get %s requires a format", args[i])
			}
			i++
			output = strings.ToLower(args[i])
		case strings.HasPrefix(args[i], "--output="):
			output = strings.ToLower(strings.TrimPrefix(args[i], "--output="))
		default:
			positionals = append(positionals, args[i])
		}
	}
	if len(positionals) < 1 || len(positionals) > 2 {
		return "", "", "", "", fmt.Errorf("get requires a kind and optional name")
	}
	kind, err := normalizeKind(positionals[0])
	if err != nil {
		return "", "", "", "", err
	}
	name := ""
	if len(positionals) == 2 {
		name = positionals[1]
	}
	if output != "json" && output != "yaml" {
		return "", "", "", "", fmt.Errorf("unsupported output %q: expected json or yaml", output)
	}
	return kind, name, server, output, nil
}

func formatResourceOutput(body []byte, output string) ([]byte, error) {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return nil, err
	}
	switch output {
	case "json":
		formatted, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(formatted, '\n'), nil
	case "yaml":
		return yaml.Marshal(value)
	default:
		return nil, fmt.Errorf("unsupported output %q", output)
	}
}

func runApply(args []string, stdout io.Writer) error {
	flags := flag.NewFlagSet("apply", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	file := flags.String("f", "", "resource manifest file")
	server := flags.String("server", defaultServer, "FlowForge server base URL")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *file == "" {
		return fmt.Errorf("apply requires -f")
	}

	body, endpoint, header, name, err := buildApplyRequest(*file)
	if err != nil {
		return err
	}
	status, responseBody, err := sendResourceRequest(http.MethodPost, strings.TrimRight(*server, "/")+endpoint, body)
	if err != nil {
		return err
	}
	if status == http.StatusConflict {
		status, responseBody, err = sendResourceRequest(
			http.MethodPut,
			strings.TrimRight(*server, "/")+endpoint+"/"+name,
			body,
		)
		if err != nil {
			return err
		}
	}
	if header.Kind == "Secret" &&
		status == http.StatusBadRequest &&
		strings.Contains(string(responseBody), "secret is immutable") {
		status, responseBody, err = sendResourceRequest(
			http.MethodPut,
			strings.TrimRight(*server, "/")+endpoint+"/"+name+"?replace=true",
			body,
		)
		if err != nil {
			return err
		}
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		message := strings.TrimSpace(string(responseBody))
		return fmt.Errorf("apply %s/%s failed: %s: %s", header.Kind, name, http.StatusText(status), message)
	}
	fmt.Fprintf(stdout, "%s/%s applied\n", header.Kind, name)
	return nil
}

func runDelete(args []string, stdout io.Writer) error {
	kind, name, server, err := parseDeleteArgs(args)
	if err != nil {
		return err
	}
	endpoint, err := endpointForKind(kind)
	if err != nil {
		return err
	}
	url := strings.TrimRight(server, "/") + endpoint + "/" + name
	status, responseBody, err := sendResourceRequest(http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	if status < http.StatusOK || status >= http.StatusMultipleChoices {
		return fmt.Errorf("delete %s/%s failed: %s: %s", kind, name, http.StatusText(status), strings.TrimSpace(string(responseBody)))
	}
	fmt.Fprintf(stdout, "%s/%s deleted\n", kind, name)
	return nil
}

func parseDeleteArgs(args []string) (string, string, string, error) {
	server := defaultServer
	file := ""
	positionals := make([]string, 0, 2)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-f":
			if i+1 >= len(args) {
				return "", "", "", fmt.Errorf("delete -f requires a manifest file")
			}
			i++
			file = args[i]
		case args[i] == "--server":
			if i+1 >= len(args) {
				return "", "", "", fmt.Errorf("delete --server requires a URL")
			}
			i++
			server = args[i]
		case strings.HasPrefix(args[i], "--server="):
			server = strings.TrimPrefix(args[i], "--server=")
		default:
			positionals = append(positionals, args[i])
		}
	}
	if file != "" {
		if len(positionals) != 0 {
			return "", "", "", fmt.Errorf("delete accepts either -f or kind and name")
		}
		_, _, header, name, err := buildApplyRequest(file)
		if err != nil {
			return "", "", "", err
		}
		return header.Kind, name, server, nil
	}
	if len(positionals) != 2 {
		return "", "", "", fmt.Errorf("delete requires kind and name, or -f")
	}
	kind, err := normalizeKind(positionals[0])
	if err != nil {
		return "", "", "", err
	}
	if isManifestPath(positionals[1]) {
		_, _, header, name, err := buildApplyRequest(positionals[1])
		if err != nil {
			return "", "", "", err
		}
		if header.Kind != kind {
			return "", "", "", fmt.Errorf("manifest kind %s does not match requested kind %s", header.Kind, kind)
		}
		return kind, name, server, nil
	}
	if positionals[1] == "" {
		return "", "", "", fmt.Errorf("resource name is required")
	}
	return kind, positionals[1], server, nil
}

func isManifestPath(value string) bool {
	extension := strings.ToLower(filepath.Ext(value))
	return extension == ".yaml" || extension == ".yml" || extension == ".json"
}

func runCompletion(args []string, stdout io.Writer) error {
	if len(args) == 2 && args[0] == "install" {
		return installCompletion(args[1], stdout)
	}
	if len(args) != 1 {
		return fmt.Errorf("completion requires a shell, or: completion install fish")
	}
	switch args[0] {
	case "fish":
		_, err := io.WriteString(stdout, fishCompletion)
		return err
	case "bash":
		_, err := io.WriteString(stdout, bashCompletion)
		return err
	case "zsh":
		_, err := io.WriteString(stdout, zshCompletion)
		return err
	default:
		return fmt.Errorf("unsupported shell %q: expected fish, bash, or zsh", args[0])
	}
}

func installCompletion(shell string, stdout io.Writer) error {
	if shell != "fish" {
		return fmt.Errorf("automatic completion installation currently supports fish")
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(configDir, "fish", "completions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "flowforge.fish")
	if err := os.WriteFile(path, []byte(fishCompletion), 0644); err != nil {
		return err
	}
	confDir := filepath.Join(configDir, "fish", "conf.d")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		return err
	}
	loaderPath := filepath.Join(confDir, "flowforge-completion.fish")
	loader := fmt.Sprintf("source %s\n", fishQuote(path))
	if err := os.WriteFile(loaderPath, []byte(loader), 0644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "fish completion installed at %s\n", path)
	return nil
}

func fishQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "\\'") + "'"
}

func sendResourceRequest(method, url string, body []byte) (int, []byte, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := resourceHTTPClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, responseBody, nil
}

func buildApplyRequest(path string) ([]byte, string, resourceHeader, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", resourceHeader{}, "", err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, "", resourceHeader{}, "", fmt.Errorf("decode manifest: %w", err)
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return nil, "", resourceHeader{}, "", err
	}

	var header resourceHeader
	if err := json.Unmarshal(body, &header); err != nil {
		return nil, "", resourceHeader{}, "", err
	}
	endpoint, err := endpointForKind(header.Kind)
	if err != nil {
		return nil, "", resourceHeader{}, "", err
	}
	name := resourceName(body)
	if name == "" {
		return nil, "", resourceHeader{}, "", fmt.Errorf("%s metadata.name is required", header.Kind)
	}
	return body, endpoint, header, name, nil
}

func endpointForKind(kind string) (string, error) {
	switch kind {
	case "Secret":
		return "/v1/secrets", nil
	case "Workflow":
		return "/v1/workflows", nil
	default:
		return "", fmt.Errorf("unsupported kind %q", kind)
	}
}

func normalizeKind(kind string) (string, error) {
	switch strings.ToLower(kind) {
	case "secret", "secrets":
		return "Secret", nil
	case "workflow", "workflows":
		return "Workflow", nil
	default:
		return "", fmt.Errorf("unsupported kind %q", kind)
	}
}

func resourceName(body []byte) string {
	var resource struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(body, &resource); err != nil {
		return ""
	}
	return resource.Metadata.Name
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  flowforge apply -f <manifest.yaml> [--server http://127.0.0.1:8080]")
	fmt.Fprintln(w, "  flowforge get <kind> [name] [-o json|yaml] [--server http://127.0.0.1:8080]")
	fmt.Fprintln(w, "  flowforge delete <kind> <name> [--server http://127.0.0.1:8080]")
	fmt.Fprintln(w, "  flowforge delete <kind> <manifest.yaml> [--server http://127.0.0.1:8080]")
	fmt.Fprintln(w, "  flowforge delete -f <manifest.yaml> [--server http://127.0.0.1:8080]")
	fmt.Fprintln(w, "  flowforge completion <fish|bash|zsh>")
	fmt.Fprintln(w, "  flowforge completion install fish")
}
