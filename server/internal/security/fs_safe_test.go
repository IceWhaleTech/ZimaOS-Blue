package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenFileWithinRoot(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir, err := os.MkdirTemp("", "fs-safe-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	subFile := filepath.Join(subDir, "nested.txt")
	if err := os.WriteFile(subFile, []byte("nested content"), 0644); err != nil {
		t.Fatalf("failed to create nested file: %v", err)
	}

	tests := []struct {
		name          string
		rootDir       string
		relativePath  string
		allowSymlinks bool
		wantErr       bool
		errCode       SafeOpenErrorCode
	}{
		{
			name:          "valid file in root",
			rootDir:       tmpDir,
			relativePath:  "test.txt",
			allowSymlinks: false,
			wantErr:       false,
		},
		{
			name:          "valid nested file",
			rootDir:       tmpDir,
			relativePath:  "subdir/nested.txt",
			allowSymlinks: false,
			wantErr:       false,
		},
		{
			name:          "path traversal attempt",
			rootDir:       tmpDir,
			relativePath:  "../etc/passwd",
			allowSymlinks: false,
			wantErr:       true,
			errCode:       ErrCodeInvalidPath,
		},
		{
			name:          "absolute path",
			rootDir:       tmpDir,
			relativePath:  "/etc/passwd",
			allowSymlinks: false,
			wantErr:       true,
			errCode:       "", // Error code may vary by platform
		},
		{
			name:          "non-existent file",
			rootDir:       tmpDir,
			relativePath:  "nonexistent.txt",
			allowSymlinks: false,
			wantErr:       true,
			errCode:       ErrCodeNotFound,
		},
		{
			name:          "directory instead of file",
			rootDir:       tmpDir,
			relativePath:  "subdir",
			allowSymlinks: false,
			wantErr:       true,
			errCode:       ErrCodeInvalidPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := OpenFileWithinRoot(tt.rootDir, tt.relativePath, tt.allowSymlinks)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
					if result != nil {
						result.Close()
					}
					return
				}

				if tt.errCode != "" {
					code, ok := GetSafeOpenErrorCode(err)
					if !ok {
						t.Errorf("expected SafeOpenError but got: %v", err)
						return
					}
					if code != tt.errCode {
						t.Errorf("expected error code %s but got %s", tt.errCode, code)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Errorf("expected result but got nil")
				return
			}

			defer result.Close()

			// Verify the file can be read
			buf := make([]byte, 100)
			n, err := result.File.Read(buf)
			if err != nil {
				t.Errorf("failed to read file: %v", err)
			}
			if n == 0 {
				t.Errorf("expected to read some bytes")
			}
		})
	}
}

func TestValidateMediaID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "valid alphanumeric",
			id:      "abc123",
			wantErr: false,
		},
		{
			name:    "valid with dots",
			id:      "file.txt",
			wantErr: false,
		},
		{
			name:    "valid with hyphens",
			id:      "my-file-123",
			wantErr: false,
		},
		{
			name:    "valid with underscores",
			id:      "my_file_123",
			wantErr: false,
		},
		{
			name:    "valid UUID",
			id:      "550e8400-e29b-41d4-a716-446655440000",
			wantErr: false,
		},
		{
			name:    "empty string",
			id:      "",
			wantErr: true,
		},
		{
			name:    "dot only",
			id:      ".",
			wantErr: true,
		},
		{
			name:    "double dot",
			id:      "..",
			wantErr: true,
		},
		{
			name:    "path traversal",
			id:      "../etc/passwd",
			wantErr: true,
		},
		{
			name:    "contains space",
			id:      "my file",
			wantErr: true,
		},
		{
			name:    "contains slash",
			id:      "path/to/file",
			wantErr: true,
		},
		{
			name:    "too long",
			id:      string(make([]byte, 201)),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMediaID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMediaID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestSecureFileModes(t *testing.T) {
	if SecureFileMode() != 0600 {
		t.Errorf("SecureFileMode() = %o, want 0600", SecureFileMode())
	}

	if SecureDirMode() != 0700 {
		t.Errorf("SecureDirMode() = %o, want 0700", SecureDirMode())
	}
}

func TestCreateSecureDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "secure-dir-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testDir := filepath.Join(tmpDir, "secure", "nested")
	if err := CreateSecureDir(testDir); err != nil {
		t.Fatalf("CreateSecureDir() error = %v", err)
	}

	info, err := os.Stat(testDir)
	if err != nil {
		t.Fatalf("failed to stat created dir: %v", err)
	}

	if !info.IsDir() {
		t.Errorf("expected directory but got file")
	}
}

func TestWriteSecureFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "secure-file-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "secure.txt")
	content := []byte("secure content")

	if err := WriteSecureFile(testFile, content); err != nil {
		t.Fatalf("WriteSecureFile() error = %v", err)
	}

	// Verify content
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", data, content)
	}
}
