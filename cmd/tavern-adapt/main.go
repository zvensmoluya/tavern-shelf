package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/zvensmoluya/tavern-shelf/internal/adaptation"
	"github.com/zvensmoluya/tavern-shelf/internal/app"
)

func main() {
	dataDir := flag.String("data-dir", "", "Shelf data directory")
	characterID := flag.String("character", "", "character id/source hash to adapt")
	baseURL := flag.String("base-url", os.Getenv("TAVERN_TEST_BASE_URL"), "Responses API base URL")
	apiKey := flag.String("api-key", os.Getenv("TAVERN_TEST_API_KEY"), "Responses API key (prefer environment variable)")
	model := flag.String("model", os.Getenv("TAVERN_TEST_MODEL"), "model id")
	maxOutputTokens := flag.Int("max-output-tokens", 16_384, "maximum model output tokens")
	programViewOnly := flag.Bool("program-view-only", false, "build the sanitized Program View without calling a model")
	flag.Parse()

	if strings.TrimSpace(*dataDir) == "" || strings.TrimSpace(*characterID) == "" {
		log.Fatal("-data-dir and -character are required")
	}
	shelf, err := app.Open(app.Options{DataDir: *dataDir})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := shelf.Close(); err != nil {
			log.Printf("close Shelf: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	view, record, err := shelf.BuildProgramView(ctx, *characterID)
	if err != nil {
		log.Fatal(err)
	}
	if *programViewOnly {
		writeSummary(map[string]any{
			"characterId": record.CharacterID, "sourceSha256": record.SourceHash,
			"status": record.Status, "programBlocks": len(view.ProgramBlocks),
			"worldBookHandles": len(view.WorldBookHandles), "programViewHash": record.ProgramViewHash,
		})
		return
	}
	compiler, err := adaptation.NewCompiler(adaptation.CompilerConfig{
		BaseURL: *baseURL, APIKey: *apiKey, Model: *model, MaxOutputTokens: *maxOutputTokens,
	})
	if err != nil {
		log.Fatal(err)
	}
	result, err := compiler.Compile(ctx, view)
	if err != nil {
		log.Fatal(err)
	}
	record, err = shelf.InstallAdaptation(ctx, *characterID, strings.NewReader(string(result.Raw)))
	if err != nil {
		log.Fatal(err)
	}
	writeSummary(map[string]any{
		"characterId": record.CharacterID, "sourceSha256": record.SourceHash,
		"status": record.Status, "artifactHash": record.ArtifactHash,
		"artifactSize": record.ArtifactSize, "model": record.CompilerModel,
		"attempts": result.Attempts, "inputTokens": result.InputTokens, "outputTokens": result.OutputTokens,
	})
}

func writeSummary(value any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		fmt.Fprintln(os.Stderr, "encode result:", err)
		os.Exit(1)
	}
}
