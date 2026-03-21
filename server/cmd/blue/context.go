package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/contextpack"
	contextpackembed "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/contextpack/embedded"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"
	"github.com/spf13/cobra"
)

var (
	contextLang    string
	contextVersion string
	contextFile    string
	contextFull    bool
	contextTenant  string
	contextUser    string
	contextClear   bool
	contextList    bool
)

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Local context pack management",
	Long: `Search, inspect, annotate, import, and validate local context packs.

Context packs live under {workspace}/.blue/context and are used for dynamic,
auditable prompt-time context injection.`,
}

var contextSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search local context packs",
	Args:  cobra.MinimumNArgs(1),
	Run:   runContextSearch,
}

var contextGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get a context pack document",
	Args:  cobra.ExactArgs(1),
	Run:   runContextGet,
}

var contextAnnotateCmd = &cobra.Command{
	Use:   "annotate <id> [note]",
	Short: "Create, list, or clear local context annotations",
	Args:  cobra.RangeArgs(1, 2),
	Run:   runContextAnnotate,
}

var contextImportCmd = &cobra.Command{
	Use:   "import <dir>",
	Short: "Import local context packs into the workspace registry",
	Args:  cobra.ExactArgs(1),
	Run:   runContextImport,
}

var contextValidateCmd = &cobra.Command{
	Use:   "validate [dir]",
	Short: "Validate the workspace registry or a candidate import directory",
	Args:  cobra.MaximumNArgs(1),
	Run:   runContextValidate,
}

func init() {
	contextGetCmd.Flags().StringVar(&contextLang, "lang", "", "preferred language variant")
	contextGetCmd.Flags().StringVar(&contextVersion, "version", "", "preferred version variant")
	contextGetCmd.Flags().StringVar(&contextFile, "file", "", "specific file within the pack")
	contextGetCmd.Flags().BoolVar(&contextFull, "full", false, "include references as well as the primary file")

	contextAnnotateCmd.Flags().StringVar(&contextLang, "lang", "", "annotation language scope")
	contextAnnotateCmd.Flags().StringVar(&contextVersion, "version", "", "annotation version scope")
	contextAnnotateCmd.Flags().StringVar(&contextFile, "file", "", "annotation file scope")
	contextAnnotateCmd.Flags().StringVar(&contextTenant, "tenant", "", "tenant scope (defaults to local)")
	contextAnnotateCmd.Flags().StringVar(&contextUser, "user", "", "user scope (defaults to local)")
	contextAnnotateCmd.Flags().BoolVar(&contextClear, "clear", false, "delete the matching annotation")
	contextAnnotateCmd.Flags().BoolVar(&contextList, "list", false, "list matching annotations instead of writing")

	contextCmd.AddCommand(contextSearchCmd)
	contextCmd.AddCommand(contextGetCmd)
	contextCmd.AddCommand(contextAnnotateCmd)
	contextCmd.AddCommand(contextImportCmd)
	contextCmd.AddCommand(contextValidateCmd)

	rootCmd.AddCommand(contextCmd)
}

type localContextRuntime struct {
	workspace *workspace.Manager
	registry  *contextpack.Registry
	store     *contextpack.AnnotationStore
	db        *sql.DB
}

func openLocalContextRuntime() (*localContextRuntime, error) {
	dataDir := getDataDir()
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, err
	}
	workspaceMgr := workspace.NewManager(filepath.Join(dataDir, "workspace"))
	if err := workspaceMgr.EnsureWorkspace(); err != nil {
		return nil, err
	}
	if err := workspaceMgr.ReleaseContextPacks(contextpackembed.PacksFS); err != nil {
		return nil, err
	}
	cfg, err := config.Load("")
	if err != nil {
		return nil, err
	}
	db, err := openPrimaryDatabaseWithStartupRecovery(dataDir, cfg.Performance.Database)
	if err != nil {
		return nil, err
	}
	if _, err := contextpack.MigrateLegacyAnnotations(context.Background(), db, dataDir); err != nil {
		_ = db.Close()
		return nil, err
	}
	store, err := contextpack.NewAnnotationStoreWithDB(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	registry := contextpack.NewRegistry(workspaceMgr.ContextDir())
	registry.SetRefreshTTL(0)
	if err := registry.Refresh(context.Background()); err != nil {
		_ = store.Close()
		_ = db.Close()
		return nil, err
	}
	return &localContextRuntime{workspace: workspaceMgr, registry: registry, store: store, db: db}, nil
}

func closeLocalContextRuntime(rt *localContextRuntime) {
	if rt == nil {
		return
	}
	if rt.store != nil {
		_ = rt.store.Close()
	}
	if rt.db != nil {
		_ = rt.db.Close()
	}
}

func runContextSearch(cmd *cobra.Command, args []string) {
	rt, err := openLocalContextRuntime()
	if err != nil {
		printContextError("Failed to initialize context registry", err)
		return
	}
	defer closeLocalContextRuntime(rt)

	query := strings.TrimSpace(strings.Join(args, " "))
	results, err := rt.registry.Search(context.Background(), contextpack.SearchOptions{Query: query, Language: contextLang, Version: contextVersion, Limit: 10})
	if err != nil {
		printContextError("Failed to search context packs", err)
		return
	}
	if jsonOutput {
		printJSON(map[string]interface{}{"query": query, "results": results})
		return
	}
	if len(results) == 0 {
		fmt.Println("No context packs found")
		return
	}
	fmt.Printf("Context packs for %q:\n", query)
	for _, result := range results {
		fmt.Printf("- %s [%s] score=%.2f\n", result.Entry.ID, result.Entry.Type, result.Score)
		if result.Entry.Description != "" {
			fmt.Printf("    %s\n", result.Entry.Description)
		}
		if len(result.Entry.Tags) > 0 {
			fmt.Printf("    tags: %s\n", strings.Join(result.Entry.Tags, ", "))
		}
	}
}

func runContextGet(cmd *cobra.Command, args []string) {
	rt, err := openLocalContextRuntime()
	if err != nil {
		printContextError("Failed to initialize context registry", err)
		return
	}
	defer closeLocalContextRuntime(rt)

	result, err := rt.registry.Get(context.Background(), args[0], contextpack.GetOptions{Language: contextLang, Version: contextVersion, File: contextFile, Full: contextFull})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			printContextError("Context pack not found", err)
			return
		}
		printContextError("Failed to load context pack", err)
		return
	}
	anns, _ := rt.store.List(context.Background(), contextpack.AnnotationFilter{TenantID: contextTenant, UserID: contextUser, EntryID: result.Entry.ID, Language: contextLang, Version: contextVersion})
	if jsonOutput {
		printJSON(map[string]interface{}{"entry": result.Entry, "variant": result.Variant, "files": result.Files, "annotations": anns})
		return
	}
	fmt.Printf("Context Pack: %s\n", result.Entry.ID)
	fmt.Printf("Type: %s\n", result.Entry.Type)
	if result.Variant.Language != "" || result.Variant.Version != "" {
		fmt.Printf("Variant: lang=%s version=%s\n", emptyAsDash(result.Variant.Language), emptyAsDash(result.Variant.Version))
	}
	for _, file := range result.Files {
		fmt.Printf("\n--- %s ---\n%s\n", file.Path, file.Content)
	}
	if len(anns) > 0 {
		fmt.Println("\nAnnotations:")
		for _, ann := range anns {
			fmt.Printf("- [%s] %s\n", ann.UpdatedAt.Format(time.RFC3339), ann.Note)
		}
	}
}

func runContextAnnotate(cmd *cobra.Command, args []string) {
	rt, err := openLocalContextRuntime()
	if err != nil {
		printContextError("Failed to initialize annotation store", err)
		return
	}
	defer closeLocalContextRuntime(rt)

	id := strings.TrimSpace(args[0])
	filter := contextpack.AnnotationFilter{TenantID: contextTenant, UserID: contextUser, EntryID: id, Language: contextLang, Version: contextVersion, File: contextFile}
	if contextList {
		anns, err := rt.store.List(context.Background(), filter)
		if err != nil {
			printContextError("Failed to list annotations", err)
			return
		}
		sort.Slice(anns, func(i, j int) bool { return anns[i].UpdatedAt.After(anns[j].UpdatedAt) })
		if jsonOutput {
			printJSON(map[string]interface{}{"annotations": anns})
			return
		}
		if len(anns) == 0 {
			fmt.Println("No annotations found")
			return
		}
		for _, ann := range anns {
			fmt.Printf("- file=%s lang=%s version=%s updated=%s\n  %s\n", emptyAsDash(ann.File), emptyAsDash(ann.Language), emptyAsDash(ann.Version), ann.UpdatedAt.Format(time.RFC3339), ann.Note)
		}
		return
	}
	if contextClear {
		if err := rt.store.Delete(context.Background(), filter); err != nil {
			printContextError("Failed to clear annotation", err)
			return
		}
		if jsonOutput {
			printJSON(map[string]interface{}{"cleared": true, "entry_id": id})
		} else {
			fmt.Printf("Cleared annotation for %s\n", id)
		}
		return
	}
	if len(args) < 2 || strings.TrimSpace(args[1]) == "" {
		printContextError("Missing note", fmt.Errorf("note is required unless --list or --clear is used"))
		return
	}
	ann := contextpack.Annotation{TenantID: contextTenant, UserID: contextUser, EntryID: id, Language: contextLang, Version: contextVersion, File: contextFile, Note: args[1]}
	if err := rt.store.Upsert(context.Background(), ann); err != nil {
		printContextError("Failed to write annotation", err)
		return
	}
	if jsonOutput {
		printJSON(map[string]interface{}{"saved": true, "entry_id": id})
	} else {
		fmt.Printf("Saved annotation for %s\n", id)
	}
}

func runContextImport(cmd *cobra.Command, args []string) {
	rt, err := openLocalContextRuntime()
	if err != nil {
		printContextError("Failed to initialize context registry", err)
		return
	}
	defer closeLocalContextRuntime(rt)

	if err := contextpack.ImportDir(args[0], rt.workspace.ContextDir()); err != nil {
		printContextError("Failed to import context packs", err)
		return
	}
	if err := rt.registry.Refresh(context.Background()); err != nil {
		printContextError("Imported, but failed to refresh registry", err)
		return
	}
	issues, _ := rt.registry.Issues(context.Background())
	if jsonOutput {
		printJSON(map[string]interface{}{"imported": true, "path": args[0], "issues": issues})
	} else {
		fmt.Printf("Imported context packs from %s\n", args[0])
		if len(issues) > 0 {
			fmt.Printf("Validation issues: %d\n", len(issues))
		}
	}
}

func runContextValidate(cmd *cobra.Command, args []string) {
	root := ""
	if len(args) == 0 {
		rt, err := openLocalContextRuntime()
		if err != nil {
			printContextError("Failed to initialize context registry", err)
			return
		}
		defer closeLocalContextRuntime(rt)
		root = rt.workspace.ContextDir()
	} else {
		root = args[0]
	}
	registry := contextpack.NewRegistry(root)
	registry.SetRefreshTTL(0)
	if err := registry.Refresh(context.Background()); err != nil {
		printContextError("Validation failed", err)
		return
	}
	issues, err := registry.Issues(context.Background())
	if err != nil {
		printContextError("Validation failed", err)
		return
	}
	if jsonOutput {
		printJSON(map[string]interface{}{"root": root, "issues": issues})
		return
	}
	if len(issues) == 0 {
		fmt.Printf("Context registry OK: %s\n", root)
		return
	}
	fmt.Printf("Validation issues for %s:\n", root)
	for _, issue := range issues {
		fmt.Printf("- %s: %s\n", emptyAsDash(issue.Path), issue.Message)
	}
}

func printContextError(msg string, err error) {
	if jsonOutput {
		printJSON(map[string]interface{}{"error": msg, "detail": err.Error()})
	} else {
		fmt.Printf("%s: %v\n", msg, err)
	}
	os.Exit(1)
}

func emptyAsDash(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "-"
	}
	return v
}
