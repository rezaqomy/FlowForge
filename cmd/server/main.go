package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"flowforge/internal/api"
	"flowforge/internal/app"
	"flowforge/internal/catalog"
	"flowforge/internal/kernel"
	"flowforge/internal/plugins/ai"
	"flowforge/internal/plugins/storage"
	"flowforge/internal/plugins/telegram"
	"flowforge/internal/runner"
	"flowforge/internal/secrets"
	"flowforge/internal/store"
)

type workflowFiles []string

func (f *workflowFiles) String() string {
	return strings.Join(*f, ",")
}

func (f *workflowFiles) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func main() {
	var files workflowFiles
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	dataDir := flag.String("data-dir", ".flowforge", "directory for local encrypted state")
	telegramPoll := flag.Bool("telegram-poll", true, "poll Telegram getUpdates and trigger telegram.message workflows")
	flag.Var(&files, "workflow", "workflow YAML file to load at startup; may be repeated")
	flag.Parse()

	if err := run(*addr, *dataDir, *telegramPoll, files); err != nil {
		log.Fatal(err)
	}
}

func run(addr, dataDir string, telegramPoll bool, workflowPaths []string) error {
	cipher, err := localCipher(dataDir)
	if err != nil {
		return err
	}
	secretStore := secrets.NewEncryptedFileStore(filepath.Join(dataDir, "secrets"), cipher)
	workflowStore := store.NewFileWorkflowStore(filepath.Join(dataDir, "workflows"))

	reg := kernel.NewRegistry()
	cat := catalog.New()
	ai.Register(reg, cat)
	telegram.RegisterWithOptions(reg, cat, telegram.SendOptions{
		SecretResolver: secretStore,
		BotTokenRef:    &secrets.SecretRef{Name: "telegram-bot", Key: "api-key"},
		ProxyURLRef:    &secrets.SecretRef{Name: "telegram-proxy", Key: "url"},
	})
	storage.Register(reg, cat)

	for _, path := range workflowPaths {
		workflow, err := loadWorkflow(path)
		if err != nil {
			return err
		}
		if err := workflowStore.Create(workflow); err != nil {
			if errors.Is(err, store.ErrWorkflowExists) {
				log.Printf("workflow %q already exists in persistent state; startup file %s skipped", workflow.Metadata.Name, path)
				continue
			}
			return err
		}
		log.Printf("loaded workflow %q from %s", workflow.Metadata.Name, path)
	}

	engine := kernel.NewEngine(reg, kernel.NewEvaluator(), kernel.NewResolver())
	runService := app.NewRunService(runner.New(engine))
	server := api.NewServer(secretStore, workflowStore, runService)

	if telegramPoll {
		poller := telegram.NewPoller(telegram.PollerOptions{
			SecretResolver: secretStore,
			BotTokenRef:    &secrets.SecretRef{Name: "telegram-bot", Key: "api-key"},
			ProxyURLRef:    &secrets.SecretRef{Name: "telegram-proxy", Key: "url"},
			Logger:         log.Default(),
			HandleEvent: func(ctx context.Context, event kernel.Event) error {
				return dispatchEvent(ctx, workflowStore, runService, event)
			},
		})
		go poller.Run(context.Background())
		log.Printf("telegram polling enabled")
	}

	log.Printf("flowforge server listening on %s", addr)
	log.Printf("state directory: %s", dataDir)
	return http.ListenAndServe(addr, server.Handler())
}

func dispatchEvent(ctx context.Context, workflows store.WorkflowStore, runs *app.RunService, event kernel.Event) error {
	list, err := workflows.List()
	if err != nil {
		return err
	}
	for _, workflow := range list {
		if workflow.Spec.Trigger.Type != event.Type {
			continue
		}
		if _, err := runs.Run(ctx, kernel.RunRequest{
			Workflow: workflow,
			Event:    event,
			Inputs:   map[string]any{},
			Mode:     kernel.RunModeLive,
		}); err != nil {
			return fmt.Errorf("workflow %s: %w", workflow.Metadata.Name, err)
		}
	}
	return nil
}

func loadWorkflow(path string) (kernel.WorkflowResource, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return kernel.WorkflowResource{}, err
	}
	var workflow kernel.WorkflowResource
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return kernel.WorkflowResource{}, err
	}
	if workflow.Metadata.Name == "" {
		return kernel.WorkflowResource{}, fmt.Errorf("workflow %s metadata.name is required", path)
	}
	return workflow, nil
}

func localCipher(dataDir string) (*secrets.EnvelopeCipher, error) {
	key, err := loadOrCreateMasterKey(filepath.Join(dataDir, "master.key"))
	if err != nil {
		return nil, err
	}
	return secrets.NewEnvelopeCipher(secrets.StaticKeyProvider{Key: key}), nil
}

func loadOrCreateMasterKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err == nil {
		return key, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	key, err = secrets.GenerateMasterKey()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, key, 0600); err != nil {
		return nil, err
	}
	return key, nil
}
