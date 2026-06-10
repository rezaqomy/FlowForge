package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultServer = "http://localhost:8080"

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
	case "help", "-h", "--help":
		usage(stdout)
		return nil
	default:
		usage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
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
	url := strings.TrimRight(*server, "/") + endpoint
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("apply %s/%s failed: %s: %s", header.Kind, name, resp.Status, strings.TrimSpace(string(responseBody)))
	}
	fmt.Fprintf(stdout, "%s/%s applied\n", header.Kind, name)
	return nil
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
	return body, endpoint, header, resourceName(body), nil
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

func resourceName(body []byte) string {
	var resource struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(body, &resource); err != nil || resource.Metadata.Name == "" {
		return "<unknown>"
	}
	return resource.Metadata.Name
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  flowforge apply -f <manifest.yaml> [--server http://localhost:8080]")
}
