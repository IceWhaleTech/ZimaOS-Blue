package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	harnesspkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillbundle"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	harnessDatasetBundleLocalPath string
	harnessDatasetBundleSource    string
	harnessDatasetBundlePath      string
	harnessDatasetBundleVersion   string

	harnessDatasetBundleHTTPClient = &http.Client{Timeout: 15 * time.Second}
)

var harnessDatasetCmd = &cobra.Command{
	Use:   "dataset",
	Short: "Import bundle-based Harness datasets",
}

var harnessDatasetImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import a local Harness dataset bundle",
	Args:  cobra.NoArgs,
	RunE:  runHarnessDatasetImportE,
}

var harnessDatasetPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull a GitHub-hosted Harness dataset bundle and import it",
	Args:  cobra.NoArgs,
	RunE:  runHarnessDatasetPullE,
}

type harnessDatasetBundleFile struct {
	APIVersion     string                                     `yaml:"api_version"`
	Kind           string                                     `yaml:"kind"`
	Name           string                                     `yaml:"name"`
	Description    string                                     `yaml:"description"`
	Subject        string                                     `yaml:"subject"`
	DefaultRunKind harnesspkg.RunKind                         `yaml:"default_run_kind"`
	DefaultProfile string                                     `yaml:"default_profile"`
	DefaultVersion string                                     `yaml:"default_version"`
	Metadata       map[string]interface{}                     `yaml:"metadata"`
	Versions       map[string]harnessDatasetBundleVersionYAML `yaml:"versions"`
}

type harnessDatasetBundleVersionYAML struct {
	EvalSpecs []string `yaml:"eval_specs"`
}

type harnessDatasetBundleEvalSpecYAML struct {
	Name          string                          `yaml:"name"`
	Subject       string                          `yaml:"subject"`
	RunKind       harnesspkg.RunKind              `yaml:"run_kind"`
	Profile       string                          `yaml:"profile"`
	Scheduler     harnesspkg.GroupSchedulerConfig `yaml:"scheduler"`
	Scoring       harnesspkg.GroupScoringConfig   `yaml:"scoring"`
	RuntimePolicy map[string]interface{}          `yaml:"runtime_policy"`
	Metadata      map[string]interface{}          `yaml:"metadata"`
}

type harnessDatasetGitHubSource struct {
	Owner      string
	Repo       string
	Ref        string
	BundlePath string
}

func init() {
	harnessDatasetCmd.PersistentFlags().StringVar(&harnessOwnerUserID, "owner", "", "owner user id for imported dataset assets (defaults to BLUE_USER_ID or local-cli)")
	harnessDatasetImportCmd.Flags().StringVar(&harnessDatasetBundleLocalPath, "path", "", "local bundle directory path")
	harnessDatasetImportCmd.Flags().StringVar(&harnessDatasetBundleVersion, "version", "", "bundle version to import (defaults to dataset.yaml default_version)")
	harnessDatasetPullCmd.Flags().StringVar(&harnessDatasetBundleSource, "source", "", "GitHub repo or tree URL for the bundle source")
	harnessDatasetPullCmd.Flags().StringVar(&harnessDatasetBundlePath, "bundle-path", "", "bundle path inside the GitHub repo when --source is a repo URL")
	harnessDatasetPullCmd.Flags().StringVar(&harnessDatasetBundleVersion, "version", "", "bundle version to import (defaults to dataset.yaml default_version)")
	harnessDatasetCmd.AddCommand(harnessDatasetImportCmd)
	harnessDatasetCmd.AddCommand(harnessDatasetPullCmd)
	harnessCmd.AddCommand(harnessDatasetCmd)
}

func runHarnessDatasetImportE(cmd *cobra.Command, args []string) error {
	req, err := loadHarnessDatasetBundleFromDir(harnessDatasetBundleLocalPath, harnessDatasetBundleVersion)
	if err != nil {
		return err
	}
	applyHarnessDatasetBundleOwner(req, resolveHarnessOwnerUserID(harnessOwnerUserID))
	result, err := importHarnessDatasetBundle(*req)
	if err != nil {
		return err
	}
	if jsonOutput {
		printJSON(result)
		return nil
	}
	printHarnessDatasetBundleSummary("Imported local dataset bundle", result)
	return nil
}

func runHarnessDatasetPullE(cmd *cobra.Command, args []string) error {
	req, err := loadHarnessDatasetBundleFromGitHubSource(harnessDatasetBundleSource, harnessDatasetBundlePath, harnessDatasetBundleVersion)
	if err != nil {
		return err
	}
	applyHarnessDatasetBundleOwner(req, resolveHarnessOwnerUserID(harnessOwnerUserID))
	result, err := importHarnessDatasetBundle(*req)
	if err != nil {
		return err
	}
	if jsonOutput {
		printJSON(result)
		return nil
	}
	printHarnessDatasetBundleSummary("Imported GitHub dataset bundle", result)
	return nil
}

func loadHarnessDatasetBundleFromDir(bundleDir string, requestedVersion string) (*harnesspkg.ImportDatasetBundleRequest, error) {
	bundleDir = strings.TrimSpace(bundleDir)
	if bundleDir == "" {
		return nil, fmt.Errorf("--path is required")
	}
	absBundleDir, err := filepath.Abs(bundleDir)
	if err != nil {
		return nil, err
	}
	meta, err := readHarnessDatasetBundleFile(filepath.Join(absBundleDir, "dataset.yaml"))
	if err != nil {
		return nil, err
	}
	selectedVersion := firstNonEmpty(strings.TrimSpace(requestedVersion), strings.TrimSpace(meta.DefaultVersion))
	if selectedVersion == "" {
		return nil, fmt.Errorf("bundle default_version is required")
	}
	manifest, err := readHarnessDatasetManifestFile(filepath.Join(absBundleDir, "versions", selectedVersion, "manifest.json"))
	if err != nil {
		return nil, err
	}
	evalSpecs, err := readHarnessDatasetEvalSpecsFromDir(filepath.Join(absBundleDir, "versions", selectedVersion, "eval-specs"), meta.Versions[selectedVersion].EvalSpecs)
	if err != nil {
		return nil, err
	}
	return buildHarnessDatasetBundleImportRequest(meta, selectedVersion, manifest, evalSpecs, "dataset_bundle_local", absBundleDir), nil
}

func loadHarnessDatasetBundleFromGitHubSource(source, bundlePath, requestedVersion string) (*harnesspkg.ImportDatasetBundleRequest, error) {
	spec, err := resolveHarnessDatasetGitHubSource(source, bundlePath)
	if err != nil {
		return nil, err
	}

	metaRaw, resolvedRef, err := fetchHarnessDatasetGitHubBundleWithRef(spec, path.Join(spec.BundlePath, "dataset.yaml"))
	if err != nil {
		return nil, err
	}
	spec.Ref = resolvedRef
	meta, err := parseHarnessDatasetBundleFile(metaRaw)
	if err != nil {
		return nil, err
	}
	selectedVersion := firstNonEmpty(strings.TrimSpace(requestedVersion), strings.TrimSpace(meta.DefaultVersion))
	if selectedVersion == "" {
		return nil, fmt.Errorf("bundle default_version is required")
	}
	manifestRaw, _, err := fetchHarnessDatasetGitHubBundleWithRef(spec, path.Join(spec.BundlePath, "versions", selectedVersion, "manifest.json"))
	if err != nil {
		return nil, err
	}
	manifest, err := parseHarnessDatasetManifest(manifestRaw)
	if err != nil {
		return nil, err
	}
	evalSpecNames := meta.Versions[selectedVersion].EvalSpecs
	evalSpecs := make([]harnessDatasetBundleEvalSpecYAML, 0, len(evalSpecNames))
	for _, filename := range evalSpecNames {
		filename = strings.TrimSpace(filename)
		if filename == "" {
			continue
		}
		blob, _, err := fetchHarnessDatasetGitHubBundleWithRef(spec, path.Join(spec.BundlePath, "versions", selectedVersion, "eval-specs", filename))
		if err != nil {
			return nil, err
		}
		evalSpec, err := parseHarnessDatasetEvalSpec(blob)
		if err != nil {
			return nil, err
		}
		evalSpecs = append(evalSpecs, evalSpec)
	}
	return buildHarnessDatasetBundleImportRequest(meta, selectedVersion, manifest, evalSpecs, "dataset_bundle_github", canonicalHarnessDatasetGitHubTreeURL(spec)), nil
}

func buildHarnessDatasetBundleImportRequest(meta harnessDatasetBundleFile, version string, manifest map[string]interface{}, evalSpecs []harnessDatasetBundleEvalSpecYAML, sourceType, sourceRef string) *harnesspkg.ImportDatasetBundleRequest {
	req := &harnesspkg.ImportDatasetBundleRequest{
		SourceType: sourceType,
		SourceRef:  sourceRef,
		Dataset: harnesspkg.DatasetSpec{
			Name:           strings.TrimSpace(meta.Name),
			Description:    strings.TrimSpace(meta.Description),
			Subject:        strings.TrimSpace(meta.Subject),
			DefaultRunKind: meta.DefaultRunKind,
			DefaultProfile: strings.TrimSpace(meta.DefaultProfile),
			Metadata:       cloneMetadataMap(meta.Metadata),
		},
		Version: harnesspkg.DatasetVersionSpec{
			Version:    strings.TrimSpace(version),
			SourceType: sourceType,
			SourceRef:  sourceRef,
			Manifest:   cloneMetadataMap(manifest),
		},
		EvalSpecs:  make([]harnesspkg.ImportDatasetBundleEvalSpec, 0, len(evalSpecs)),
		MakeActive: true,
	}
	for _, spec := range evalSpecs {
		req.EvalSpecs = append(req.EvalSpecs, harnesspkg.ImportDatasetBundleEvalSpec{
			Name:            strings.TrimSpace(spec.Name),
			Subject:         strings.TrimSpace(spec.Subject),
			RunKind:         spec.RunKind,
			Profile:         strings.TrimSpace(spec.Profile),
			SchedulerConfig: spec.Scheduler,
			ScoringConfig:   spec.Scoring,
			RuntimePolicy:   cloneMetadataMap(spec.RuntimePolicy),
			Metadata:        cloneMetadataMap(spec.Metadata),
		})
	}
	return req
}

func applyHarnessDatasetBundleOwner(req *harnesspkg.ImportDatasetBundleRequest, ownerUserID string) {
	if req == nil {
		return
	}
	ownerUserID = strings.TrimSpace(ownerUserID)
	if ownerUserID == "" {
		return
	}
	req.Dataset.OwnerUserID = ownerUserID
	req.Version.CreatedBy = ownerUserID
	for i := range req.EvalSpecs {
		req.EvalSpecs[i].OwnerUserID = ownerUserID
	}
}

func importHarnessDatasetBundle(req harnesspkg.ImportDatasetBundleRequest) (*harnesspkg.ImportDatasetBundleResult, error) {
	resp, err := doHarnessRequest(http.MethodPost, "/dataset-bundles/import", req)
	if err != nil {
		return nil, err
	}
	var result harnesspkg.ImportDatasetBundleResult
	if err := decodeInto(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func readHarnessDatasetBundleFile(path string) (harnessDatasetBundleFile, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return harnessDatasetBundleFile{}, err
	}
	return parseHarnessDatasetBundleFile(blob)
}

func parseHarnessDatasetBundleFile(blob []byte) (harnessDatasetBundleFile, error) {
	var meta harnessDatasetBundleFile
	if err := yaml.Unmarshal(blob, &meta); err != nil {
		return harnessDatasetBundleFile{}, err
	}
	if strings.TrimSpace(meta.APIVersion) != "harness.blue/v1alpha1" {
		return harnessDatasetBundleFile{}, fmt.Errorf("dataset bundle api_version must be harness.blue/v1alpha1")
	}
	if strings.TrimSpace(meta.Kind) != "dataset_bundle" {
		return harnessDatasetBundleFile{}, fmt.Errorf("dataset bundle kind must be dataset_bundle")
	}
	if strings.TrimSpace(meta.Name) == "" {
		return harnessDatasetBundleFile{}, fmt.Errorf("dataset bundle name is required")
	}
	return meta, nil
}

func readHarnessDatasetManifestFile(path string) (map[string]interface{}, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseHarnessDatasetManifest(blob)
}

func parseHarnessDatasetManifest(blob []byte) (map[string]interface{}, error) {
	var manifest map[string]interface{}
	if err := json.Unmarshal(blob, &manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func readHarnessDatasetEvalSpecsFromDir(dir string, fileNames []string) ([]harnessDatasetBundleEvalSpecYAML, error) {
	names := normalizeHarnessDatasetEvalSpecNames(fileNames)
	if len(names) == 0 {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := strings.TrimSpace(entry.Name())
			if strings.HasSuffix(strings.ToLower(name), ".yaml") || strings.HasSuffix(strings.ToLower(name), ".yml") {
				names = append(names, name)
			}
		}
		sort.Strings(names)
	}
	out := make([]harnessDatasetBundleEvalSpecYAML, 0, len(names))
	for _, name := range names {
		blob, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		spec, err := parseHarnessDatasetEvalSpec(blob)
		if err != nil {
			return nil, err
		}
		out = append(out, spec)
	}
	return out, nil
}

func parseHarnessDatasetEvalSpec(blob []byte) (harnessDatasetBundleEvalSpecYAML, error) {
	var spec harnessDatasetBundleEvalSpecYAML
	if err := yaml.Unmarshal(blob, &spec); err != nil {
		return harnessDatasetBundleEvalSpecYAML{}, err
	}
	if strings.TrimSpace(spec.Name) == "" {
		return harnessDatasetBundleEvalSpecYAML{}, fmt.Errorf("eval spec name is required")
	}
	return spec, nil
}

func normalizeHarnessDatasetEvalSpecNames(fileNames []string) []string {
	out := make([]string, 0, len(fileNames))
	for _, name := range fileNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	return out
}

func resolveHarnessDatasetGitHubSource(source, bundlePath string) (harnessDatasetGitHubSource, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return harnessDatasetGitHubSource{}, fmt.Errorf("--source is required")
	}
	if owner, repo, ref, treePath, ok := skillbundle.ParseGitHubTreeURL(source); ok {
		return harnessDatasetGitHubSource{
			Owner:      owner,
			Repo:       repo,
			Ref:        ref,
			BundlePath: strings.Trim(strings.TrimSpace(treePath), "/"),
		}, nil
	}
	if matches := skillbundle.GitHubRepoURLPattern.FindStringSubmatch(source); len(matches) == 3 {
		bundlePath = strings.Trim(strings.TrimSpace(bundlePath), "/")
		if bundlePath == "" {
			return harnessDatasetGitHubSource{}, fmt.Errorf("--bundle-path is required for GitHub repo URLs")
		}
		return harnessDatasetGitHubSource{
			Owner:      matches[1],
			Repo:       matches[2],
			BundlePath: bundlePath,
		}, nil
	}
	return harnessDatasetGitHubSource{}, fmt.Errorf("unsupported GitHub source %q", source)
}

func fetchHarnessDatasetGitHubBundleWithRef(spec harnessDatasetGitHubSource, bundleFilePath string) ([]byte, string, error) {
	refs := []string{strings.TrimSpace(spec.Ref)}
	if refs[0] == "" {
		refs = []string{"main", "master"}
	}
	var lastErr error
	for _, ref := range refs {
		blob, err := fetchHarnessDatasetGitHubBundleFile(spec.Owner, spec.Repo, ref, bundleFilePath)
		if err == nil {
			return blob, ref, nil
		}
		lastErr = err
	}
	return nil, "", lastErr
}

func fetchHarnessDatasetGitHubBundleFile(owner, repo, ref, bundleFilePath string) ([]byte, error) {
	var lastErr error
	for _, candidate := range skillbundle.GitHubRawURLCandidates(owner, repo, ref, bundleFilePath) {
		req, err := http.NewRequest(http.MethodGet, candidate, nil)
		if err != nil {
			return nil, err
		}
		resp, err := harnessDatasetBundleHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return body, nil
		}
		lastErr = fmt.Errorf("%s returned %d", candidate, resp.StatusCode)
	}
	return nil, fmt.Errorf("failed to fetch %s from GitHub source chain: %w", bundleFilePath, lastErr)
}

func canonicalHarnessDatasetGitHubTreeURL(spec harnessDatasetGitHubSource) string {
	return fmt.Sprintf("https://github.com/%s/%s/tree/%s/%s", spec.Owner, spec.Repo, spec.Ref, strings.Trim(strings.TrimSpace(spec.BundlePath), "/"))
}

func printHarnessDatasetBundleSummary(prefix string, result *harnesspkg.ImportDatasetBundleResult) {
	fmt.Println(prefix)
	if result == nil {
		return
	}
	if result.Dataset != nil {
		fmt.Printf("Dataset: %s (%s)\n", result.Dataset.Name, result.Dataset.ID)
	}
	if result.DatasetVersion != nil {
		fmt.Printf("Version: %s (%s)\n", result.DatasetVersion.Version, result.DatasetVersion.ID)
	}
	if len(result.EvalSpecs) > 0 {
		names := make([]string, 0, len(result.EvalSpecs))
		for _, spec := range result.EvalSpecs {
			names = append(names, spec.Name)
		}
		sort.Strings(names)
		fmt.Printf("Eval specs: %s\n", strings.Join(names, ", "))
	}
}

func cloneMetadataMap(input map[string]interface{}) map[string]interface{} {
	if input == nil {
		return nil
	}
	out := make(map[string]interface{}, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
