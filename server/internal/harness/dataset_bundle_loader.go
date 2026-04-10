package harness

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

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillbundle"
	"gopkg.in/yaml.v3"
)

var datasetBundleLoaderHTTPClient = &http.Client{Timeout: 15 * time.Second}

type datasetBundleFile struct {
	APIVersion     string                              `yaml:"api_version"`
	Kind           string                              `yaml:"kind"`
	Name           string                              `yaml:"name"`
	Description    string                              `yaml:"description"`
	Subject        string                              `yaml:"subject"`
	DefaultRunKind RunKind                             `yaml:"default_run_kind"`
	DefaultProfile string                              `yaml:"default_profile"`
	DefaultVersion string                              `yaml:"default_version"`
	Metadata       map[string]interface{}              `yaml:"metadata"`
	Versions       map[string]datasetBundleVersionYAML `yaml:"versions"`
}

type datasetBundleVersionYAML struct {
	EvalSpecs []string `yaml:"eval_specs"`
}

type datasetBundleEvalSpecYAML struct {
	Name          string                 `yaml:"name"`
	Subject       string                 `yaml:"subject"`
	RunKind       RunKind                `yaml:"run_kind"`
	Profile       string                 `yaml:"profile"`
	Scheduler     GroupSchedulerConfig   `yaml:"scheduler"`
	Scoring       GroupScoringConfig     `yaml:"scoring"`
	RuntimePolicy map[string]interface{} `yaml:"runtime_policy"`
	Metadata      map[string]interface{} `yaml:"metadata"`
}

type datasetBundleGitHubSource struct {
	Owner      string
	Repo       string
	Ref        string
	BundlePath string
}

func loadImportDatasetBundleRequestFromDir(bundleDir string, requestedVersion string) (*ImportDatasetBundleRequest, error) {
	bundleDir = strings.TrimSpace(bundleDir)
	if bundleDir == "" {
		return nil, fmt.Errorf("bundle path is required")
	}
	absBundleDir, err := filepath.Abs(bundleDir)
	if err != nil {
		return nil, err
	}
	meta, err := readDatasetBundleFile(filepath.Join(absBundleDir, "dataset.yaml"))
	if err != nil {
		return nil, err
	}
	selectedVersion := firstNonEmpty(strings.TrimSpace(requestedVersion), strings.TrimSpace(meta.DefaultVersion))
	if selectedVersion == "" {
		return nil, fmt.Errorf("bundle default_version is required")
	}
	manifest, err := readDatasetBundleManifestFile(filepath.Join(absBundleDir, "versions", selectedVersion, "manifest.json"))
	if err != nil {
		return nil, err
	}
	evalSpecs, err := readDatasetBundleEvalSpecsFromDir(
		filepath.Join(absBundleDir, "versions", selectedVersion, "eval-specs"),
		meta.Versions[selectedVersion].EvalSpecs,
	)
	if err != nil {
		return nil, err
	}
	return buildDatasetBundleImportRequest(meta, selectedVersion, manifest, evalSpecs, "dataset_bundle_local", absBundleDir), nil
}

func loadImportDatasetBundleRequestFromGitHubSource(source, bundlePath, requestedVersion string) (*ImportDatasetBundleRequest, error) {
	spec, err := resolveDatasetBundleGitHubSource(source, bundlePath)
	if err != nil {
		return nil, err
	}

	metaRaw, resolvedRef, err := fetchDatasetBundleGitHubFileWithRef(spec, path.Join(spec.BundlePath, "dataset.yaml"))
	if err != nil {
		return nil, err
	}
	spec.Ref = resolvedRef
	meta, err := parseDatasetBundleFile(metaRaw)
	if err != nil {
		return nil, err
	}
	selectedVersion := firstNonEmpty(strings.TrimSpace(requestedVersion), strings.TrimSpace(meta.DefaultVersion))
	if selectedVersion == "" {
		return nil, fmt.Errorf("bundle default_version is required")
	}
	manifestRaw, _, err := fetchDatasetBundleGitHubFileWithRef(spec, path.Join(spec.BundlePath, "versions", selectedVersion, "manifest.json"))
	if err != nil {
		return nil, err
	}
	manifest, err := parseDatasetBundleManifest(manifestRaw)
	if err != nil {
		return nil, err
	}
	evalSpecNames := meta.Versions[selectedVersion].EvalSpecs
	evalSpecs := make([]datasetBundleEvalSpecYAML, 0, len(evalSpecNames))
	for _, filename := range evalSpecNames {
		filename = strings.TrimSpace(filename)
		if filename == "" {
			continue
		}
		blob, _, err := fetchDatasetBundleGitHubFileWithRef(spec, path.Join(spec.BundlePath, "versions", selectedVersion, "eval-specs", filename))
		if err != nil {
			return nil, err
		}
		evalSpec, err := parseDatasetBundleEvalSpec(blob)
		if err != nil {
			return nil, err
		}
		evalSpecs = append(evalSpecs, evalSpec)
	}
	return buildDatasetBundleImportRequest(
		meta,
		selectedVersion,
		manifest,
		evalSpecs,
		"dataset_bundle_github",
		canonicalDatasetBundleGitHubTreeURL(spec),
	), nil
}

func buildDatasetBundleImportRequest(
	meta datasetBundleFile,
	version string,
	manifest map[string]interface{},
	evalSpecs []datasetBundleEvalSpecYAML,
	sourceType,
	sourceRef string,
) *ImportDatasetBundleRequest {
	req := &ImportDatasetBundleRequest{
		SourceType: sourceType,
		SourceRef:  sourceRef,
		Dataset: DatasetSpec{
			Name:           strings.TrimSpace(meta.Name),
			Description:    strings.TrimSpace(meta.Description),
			Subject:        strings.TrimSpace(meta.Subject),
			DefaultRunKind: meta.DefaultRunKind,
			DefaultProfile: strings.TrimSpace(meta.DefaultProfile),
			Metadata:       cloneMetadataMap(meta.Metadata),
		},
		Version: DatasetVersionSpec{
			Version:    strings.TrimSpace(version),
			SourceType: sourceType,
			SourceRef:  sourceRef,
			Manifest:   cloneMetadataMap(manifest),
		},
		EvalSpecs:  make([]ImportDatasetBundleEvalSpec, 0, len(evalSpecs)),
		MakeActive: true,
	}
	for _, spec := range evalSpecs {
		req.EvalSpecs = append(req.EvalSpecs, ImportDatasetBundleEvalSpec{
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

func readDatasetBundleFile(path string) (datasetBundleFile, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return datasetBundleFile{}, err
	}
	return parseDatasetBundleFile(blob)
}

func parseDatasetBundleFile(blob []byte) (datasetBundleFile, error) {
	var meta datasetBundleFile
	if err := yaml.Unmarshal(blob, &meta); err != nil {
		return datasetBundleFile{}, err
	}
	if strings.TrimSpace(meta.APIVersion) != "harness.blue/v1alpha1" {
		return datasetBundleFile{}, fmt.Errorf("dataset bundle api_version must be harness.blue/v1alpha1")
	}
	if strings.TrimSpace(meta.Kind) != "dataset_bundle" {
		return datasetBundleFile{}, fmt.Errorf("dataset bundle kind must be dataset_bundle")
	}
	if strings.TrimSpace(meta.Name) == "" {
		return datasetBundleFile{}, fmt.Errorf("dataset bundle name is required")
	}
	return meta, nil
}

func readDatasetBundleManifestFile(path string) (map[string]interface{}, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseDatasetBundleManifest(blob)
}

func parseDatasetBundleManifest(blob []byte) (map[string]interface{}, error) {
	var manifest map[string]interface{}
	if err := json.Unmarshal(blob, &manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func readDatasetBundleEvalSpecsFromDir(dir string, fileNames []string) ([]datasetBundleEvalSpecYAML, error) {
	names := normalizeDatasetBundleEvalSpecNames(fileNames)
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
	out := make([]datasetBundleEvalSpecYAML, 0, len(names))
	for _, name := range names {
		blob, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		spec, err := parseDatasetBundleEvalSpec(blob)
		if err != nil {
			return nil, err
		}
		out = append(out, spec)
	}
	return out, nil
}

func parseDatasetBundleEvalSpec(blob []byte) (datasetBundleEvalSpecYAML, error) {
	var spec datasetBundleEvalSpecYAML
	if err := yaml.Unmarshal(blob, &spec); err != nil {
		return datasetBundleEvalSpecYAML{}, err
	}
	if strings.TrimSpace(spec.Name) == "" {
		return datasetBundleEvalSpecYAML{}, fmt.Errorf("eval spec name is required")
	}
	return spec, nil
}

func normalizeDatasetBundleEvalSpecNames(fileNames []string) []string {
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

func resolveDatasetBundleGitHubSource(source, bundlePath string) (datasetBundleGitHubSource, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return datasetBundleGitHubSource{}, fmt.Errorf("GitHub source is required")
	}
	if owner, repo, ref, treePath, ok := skillbundle.ParseGitHubTreeURL(source); ok {
		return datasetBundleGitHubSource{
			Owner:      owner,
			Repo:       repo,
			Ref:        ref,
			BundlePath: strings.Trim(strings.TrimSpace(treePath), "/"),
		}, nil
	}
	if matches := skillbundle.GitHubRepoURLPattern.FindStringSubmatch(source); len(matches) == 3 {
		bundlePath = strings.Trim(strings.TrimSpace(bundlePath), "/")
		if bundlePath == "" {
			bundlePath = DefaultDatasetBundleGitHubPathForRepo(matches[1], matches[2])
		}
		if bundlePath == "" {
			return datasetBundleGitHubSource{}, fmt.Errorf("bundle_path is required for GitHub repo URLs")
		}
		return datasetBundleGitHubSource{
			Owner:      matches[1],
			Repo:       matches[2],
			BundlePath: bundlePath,
		}, nil
	}
	return datasetBundleGitHubSource{}, fmt.Errorf("unsupported GitHub source %q", source)
}

func fetchDatasetBundleGitHubFileWithRef(spec datasetBundleGitHubSource, bundleFilePath string) ([]byte, string, error) {
	refs := []string{strings.TrimSpace(spec.Ref)}
	if refs[0] == "" {
		refs = []string{"main", "master"}
	}
	var lastErr error
	for _, ref := range refs {
		blob, err := fetchDatasetBundleGitHubFile(spec.Owner, spec.Repo, ref, bundleFilePath)
		if err == nil {
			return blob, ref, nil
		}
		lastErr = err
	}
	return nil, "", lastErr
}

func fetchDatasetBundleGitHubFile(owner, repo, ref, bundleFilePath string) ([]byte, error) {
	client := datasetBundleLoaderHTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	var lastErr error
	for _, candidate := range skillbundle.GitHubRawURLCandidates(owner, repo, ref, bundleFilePath) {
		req, err := http.NewRequest(http.MethodGet, candidate, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
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

func canonicalDatasetBundleGitHubTreeURL(spec datasetBundleGitHubSource) string {
	return fmt.Sprintf(
		"https://github.com/%s/%s/tree/%s/%s",
		spec.Owner,
		spec.Repo,
		spec.Ref,
		strings.Trim(strings.TrimSpace(spec.BundlePath), "/"),
	)
}
