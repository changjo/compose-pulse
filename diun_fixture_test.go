package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractDiunFieldsFromFixtures(t *testing.T) {
	tests := []struct {
		file       string
		wantRepo   string
		wantTag    string
		wantDigest string
	}{
		{
			file:       "entry_image_name.json",
			wantRepo:   "ghcr.io/acme/service-a",
			wantTag:    "2.1.0",
			wantDigest: "sha256:aaaabbbb",
		},
		{
			file:     "entry_image_repository.json",
			wantRepo: "docker.io/library/nginx",
			wantTag:  "1.27.1",
		},
		{
			file:       "root_image_repository_with_port.json",
			wantRepo:   "ghcr.io:5000/team/backend",
			wantDigest: "sha256:ccccdddd",
		},
		{
			file:     "root_repository_with_digest.json",
			wantRepo: "registry.example.com/ops/worker",
			wantTag:  "ignored-tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			path := filepath.Join("testdata", "diun_payloads", tt.file)
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture %s: %v", tt.file, err)
			}

			var payload map[string]any
			if err := json.Unmarshal(raw, &payload); err != nil {
				t.Fatalf("unmarshal fixture %s: %v", tt.file, err)
			}

			gotRepo, gotTag, gotDigest := extractDiunFields(payload)
			if gotRepo != tt.wantRepo {
				t.Fatalf("repo = %q, want %q", gotRepo, tt.wantRepo)
			}
			if gotTag != tt.wantTag {
				t.Fatalf("tag = %q, want %q", gotTag, tt.wantTag)
			}
			if gotDigest != tt.wantDigest {
				t.Fatalf("digest = %q, want %q", gotDigest, tt.wantDigest)
			}
		})
	}
}
