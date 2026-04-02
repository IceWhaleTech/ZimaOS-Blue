package server

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
)

type skillContractMetadata struct {
	Status string
	Source string
	Notes  []string
}

func metadataLines(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	lines := strings.Split(raw, "\n")
	values := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		values = append(values, line)
	}
	return values
}

func hasLegacyContractNote(notes []string) bool {
	for _, note := range notes {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(note)), "legacy manifest compatibility fallback applied:") {
			return true
		}
	}
	return false
}

func skillContractMetadataFromManifest(manifest *skill.Manifest) skillContractMetadata {
	if manifest == nil || manifest.Metadata == nil {
		return skillContractMetadata{}
	}

	status := strings.TrimSpace(manifest.Metadata[skillmanifest.ManifestMetadataContractStatus])
	source := strings.TrimSpace(manifest.Metadata[skillmanifest.ManifestMetadataContractSource])
	notes := metadataLines(manifest.Metadata[skillmanifest.ManifestMetadataContractNotes])
	if len(notes) == 0 {
		notes = metadataLines(manifest.Metadata["validation_notes"])
	}

	if status == "" {
		switch {
		case hasLegacyContractNote(notes):
			status = skillmanifest.ContractStatusLegacyFallback
		case len(notes) > 0:
			status = skillmanifest.ContractStatusGenerated
		}
	}
	if source == "" {
		switch status {
		case skillmanifest.ContractStatusStrict:
			source = skillmanifest.ContractSourceDeclaredFrontmatter
		case skillmanifest.ContractStatusLegacyFallback:
			source = skillmanifest.ContractSourceLegacyFrontmatter
		case skillmanifest.ContractStatusGenerated:
			source = skillmanifest.ContractSourceGeneratedSafeDefaults
		}
	}

	return skillContractMetadata{
		Status: status,
		Source: source,
		Notes:  append([]string(nil), notes...),
	}
}

func skillContractMetadataFromDocument(doc skillmanifest.Document) skillContractMetadata {
	return skillContractMetadataFromManifest(doc.Manifest)
}

func attachSkillContract(payload map[string]interface{}, meta skillContractMetadata) map[string]interface{} {
	if payload == nil {
		return nil
	}
	if meta.Status != "" {
		payload["contract_status"] = meta.Status
	}
	if meta.Source != "" {
		payload["contract_source"] = meta.Source
	}
	if len(meta.Notes) > 0 {
		payload["contract_notes"] = meta.Notes
	}
	return payload
}

func applySkillContractResponse(response *SkillResponse, meta skillContractMetadata) {
	if response == nil {
		return
	}
	response.ContractStatus = meta.Status
	response.ContractSource = meta.Source
	if len(meta.Notes) > 0 {
		response.ContractNotes = append([]string(nil), meta.Notes...)
	}
}
