package skillmanifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillbundle"
)

type InstallBundle struct {
	Root     string
	EntryDoc *skillbundle.EntryDocument
	Document Document
	Raw      []byte
}

func ValidateInstalledDir(skillDir string, expectedID string, opts Options) (*InstallBundle, error) {
	entryDoc, err := skillbundle.FindEntryDocumentInDir(skillDir)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(entryDoc.Path)
	if err != nil {
		return nil, err
	}
	doc, err := ParseEntry(filepath.Base(skillDir), entryDoc.Path, raw, opts)
	if err != nil {
		return nil, err
	}
	if err := validateExpectedID(expectedID, doc.ID); err != nil {
		return nil, err
	}
	return &InstallBundle{
		Root:     skillDir,
		EntryDoc: entryDoc,
		Document: doc,
		Raw:      raw,
	}, nil
}

func ValidateArchiveInstallRoot(extractDir, expectedID string, opts Options) (*InstallBundle, error) {
	entryDoc, err := skillbundle.FindEntryDocument(extractDir, expectedID)
	if err != nil {
		return nil, err
	}
	if _, err := skillbundle.EnsureCompatibilitySkillDoc(entryDoc.Dir, entryDoc.Path); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(entryDoc.Path)
	if err != nil {
		return nil, err
	}
	doc, err := ParseEntry(filepath.Base(entryDoc.Dir), entryDoc.Path, raw, opts)
	if err != nil {
		return nil, err
	}
	if err := validateExpectedID(expectedID, doc.ID); err != nil {
		return nil, err
	}
	return &InstallBundle{
		Root:     entryDoc.Dir,
		EntryDoc: entryDoc,
		Document: doc,
		Raw:      raw,
	}, nil
}

func validateExpectedID(expectedID, actualID string) error {
	expectedID = normalizeSkillID(strings.TrimSpace(expectedID))
	actualID = normalizeSkillID(strings.TrimSpace(actualID))
	if expectedID == "" || expectedID == "skill" {
		return nil
	}
	if expectedID != actualID {
		return fmt.Errorf("skill id mismatch: expected %s, found %s", expectedID, actualID)
	}
	return nil
}
