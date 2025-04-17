package parser

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/iamlongalong/dockerbundle/internal/model"
)

func TestIsComposeFile(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "standard docker-compose.yml",
			filePath: "/path/to/docker-compose.yml",
			want:     true,
		},
		{
			name:     "standard docker-compose.yaml",
			filePath: "/path/to/docker-compose.yaml",
			want:     true,
		},
		{
			name:     "compose.yml",
			filePath: "/path/to/compose.yml",
			want:     true,
		},
		{
			name:     "compose.yaml",
			filePath: "/path/to/compose.yaml",
			want:     true,
		},
		{
			name:     "docker-compose.prod.yml",
			filePath: "/path/to/docker-compose.prod.yml",
			want:     true,
		},
		{
			name:     "not a compose file",
			filePath: "/path/to/something-else.yml",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsComposeFile(tt.filePath); got != tt.want {
				t.Errorf("IsComposeFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectComposeFile(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "dockerbundle-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test with no compose file
	_, err = DetectComposeFile(tempDir)
	if err == nil {
		t.Errorf("DetectComposeFile() should have failed with no compose file")
	}

	// Create a docker-compose.yml file
	composeFile := filepath.Join(tempDir, "docker-compose.yml")
	err = os.WriteFile(composeFile, []byte("version: '3'"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test compose file: %v", err)
	}

	// Test with compose file
	detected, err := DetectComposeFile(tempDir)
	if err != nil {
		t.Errorf("DetectComposeFile() error = %v, wantErr nil", err)
	}

	if detected != composeFile {
		t.Errorf("DetectComposeFile() = %v, want %v", detected, composeFile)
	}
}

func TestParse(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "dockerbundle-test-")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a simple docker-compose.yml file
	composeContent := `
version: '3'
services:
  web:
    image: nginx:latest
    ports:
      - "80:80"
    volumes:
      - ./html:/usr/share/nginx/html
    depends_on:
      - db
  db:
    image: postgres:13
    volumes:
      - postgres_data:/var/lib/postgresql/data
    environment:
      - POSTGRES_PASSWORD=example
    env_file:
      - ./.env
volumes:
  postgres_data:
networks:
  default:
`

	composeFile := filepath.Join(tempDir, "docker-compose.yml")
	err = os.WriteFile(composeFile, []byte(composeContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test compose file: %v", err)
	}

	// Create an .env file
	envContent := `
POSTGRES_PASSWORD=example
POSTGRES_USER=postgres
`
	envFile := filepath.Join(tempDir, ".env")
	err = os.WriteFile(envFile, []byte(envContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test env file: %v", err)
	}

	// Create an HTML directory and file
	htmlDir := filepath.Join(tempDir, "html")
	err = os.MkdirAll(htmlDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create html directory: %v", err)
	}

	htmlFile := filepath.Join(htmlDir, "index.html")
	err = os.WriteFile(htmlFile, []byte("<html><body>Hello</body></html>"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test html file: %v", err)
	}

	// Parse the compose file
	parser := NewComposeParser(false)
	info, err := parser.Parse(composeFile)
	if err != nil {
		t.Fatalf("Parse() error = %v, wantErr nil", err)
	}

	// Validate the parsed information
	if info.Version != "3" {
		t.Errorf("Parse() version = %v, want %v", info.Version, "3")
	}

	if len(info.Services) != 2 {
		t.Errorf("Parse() services = %v, want %v", len(info.Services), 2)
	}

	if len(info.Networks) != 1 {
		t.Errorf("Parse() networks = %v, want %v", len(info.Networks), 1)
	}

	if len(info.Volumes) != 1 {
		t.Errorf("Parse() volumes = %v, want %v", len(info.Volumes), 1)
	}

	// Check service details
	var webService *model.ServiceInfo
	for i, svc := range info.Services {
		if svc.Name == "web" {
			webService = &info.Services[i]
			break
		}
	}

	if webService == nil {
		t.Fatalf("Parse() web service not found")
	}

	if webService.ImageName != "nginx:latest" {
		t.Errorf("Parse() web image = %v, want %v", webService.ImageName, "nginx:latest")
	}

	if len(webService.Ports) != 1 {
		t.Errorf("Parse() web ports = %v, want %v", len(webService.Ports), 1)
	}

	if len(webService.DependsOn) != 1 {
		t.Errorf("Parse() web depends_on = %v, want %v", len(webService.DependsOn), 1)
	}

	// Check database service
	var dbService *model.ServiceInfo
	for i, svc := range info.Services {
		if svc.Name == "db" {
			dbService = &info.Services[i]
			break
		}
	}

	if dbService == nil {
		t.Fatalf("Parse() db service not found")
	}

	if dbService.ImageName != "postgres:13" {
		t.Errorf("Parse() db image = %v, want %v", dbService.ImageName, "postgres:13")
	}

	if len(dbService.EnvFiles) != 1 {
		t.Errorf("Parse() db env_file = %v, want %v", len(dbService.EnvFiles), 1)
	}
}
