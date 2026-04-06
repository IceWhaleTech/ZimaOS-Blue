package memory

import (
	"path/filepath"
	"testing"
)

func TestDefaultChatAttachmentDirUsesMediaSubdir(t *testing.T) {
	dataDir := filepath.Join("tmp", "blue-data")
	got := DefaultChatAttachmentDir(dataDir)
	want := filepath.Join(dataDir, "media", "message_attachments")
	if got != want {
		t.Fatalf("DefaultChatAttachmentDir() = %q, want %q", got, want)
	}
}

func TestDefaultChatStoreOptionsUsesMediaMessageAttachmentsDir(t *testing.T) {
	dbPath := filepath.Join("tmp", "blue-data", "blue.db")
	got := DefaultChatStoreOptions(dbPath).AttachmentDir
	want := filepath.Join("tmp", "blue-data", "media", "message_attachments")
	if got != want {
		t.Fatalf("DefaultChatStoreOptions().AttachmentDir = %q, want %q", got, want)
	}
}
