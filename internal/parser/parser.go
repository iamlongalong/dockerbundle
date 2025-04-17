package parser

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
	"github.com/iamlongalong/dockerbundle/internal/model"
)

// ComposeParser is responsible for parsing docker-compose files
type ComposeParser struct {
	// verbose enables detailed logging
	verbose bool
}

// NewComposeParser creates a new ComposeParser
func NewComposeParser(verbose bool) *ComposeParser {
	return &ComposeParser{
		verbose: verbose,
	}
}

// Parse parses a docker-compose file and returns ComposeInfo
func (p *ComposeParser) Parse(composePath string) (*model.ComposeInfo, error) {
	// Check if the file exists
	if _, err := os.Stat(composePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("compose file does not exist: %s", composePath)
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(composePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Load the compose file content to detect version
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read compose file: %w", err)
	}

	// Simple version extraction - this is a basic approach
	version := "unknown"
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "version:") {
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				version = strings.Trim(parts[1], " '\"\n")
				break
			}
		}
	}

	// Load the compose file
	workingDir := filepath.Dir(absPath)
	composeConfig, err := loader.Load(types.ConfigDetails{
		WorkingDir: workingDir,
		ConfigFiles: []types.ConfigFile{
			{
				Filename: absPath,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load compose file: %w", err)
	}

	if p.verbose {
		log.Printf("Loaded compose file: %s (version: %s)\n", absPath, version)
	}

	// Extract information from the compose config
	info := &model.ComposeInfo{
		Version:         version,
		Services:        []model.ServiceInfo{},
		Networks:        []string{},
		Volumes:         []string{},
		EnvFiles:        []string{},
		ComposeFilePath: absPath,
	}

	// Extract networks
	for networkName := range composeConfig.Networks {
		info.Networks = append(info.Networks, networkName)
	}

	// Extract volumes
	for volumeName, volumeConfig := range composeConfig.Volumes {
		info.Volumes = append(info.Volumes, volumeName)
		// If volume has driver_opts with type=bind, treat it as a bind mount
		if volumeConfig.Driver == "local" && volumeConfig.DriverOpts != nil {
			if volumeConfig.DriverOpts["type"] == "none" && volumeConfig.DriverOpts["o"] == "bind" {
				device := volumeConfig.DriverOpts["device"]
				// Find the target path from services that use this volume
				target := "/data" // default fallback
				for _, service := range composeConfig.Services {
					for _, volume := range service.Volumes {
						if volume.Type == "volume" && volume.Source == volumeName {
							target = volume.Target
							break
						}
					}
				}
				// Store the original relative path
				info.BindMounts = append(info.BindMounts, model.BindMount{
					Name:   volumeName,
					Source: device, // Keep the original path
					Target: target, // Use actual target path from service
				})
			}
		}
	}

	// Extract secrets
	for secretName, secretConfig := range composeConfig.Secrets {
		if secretConfig.File != "" {
			absPath := secretConfig.File
			if !filepath.IsAbs(absPath) {
				absPath = filepath.Join(workingDir, absPath)
			}
			info.Secrets = append(info.Secrets, model.Secret{
				Name:   secretName,
				Type:   "file",
				Source: absPath,
				Target: filepath.Join("secrets", secretName),
			})
		}
	}

	// Process each service
	for _, serviceConfig := range composeConfig.Services {
		serviceInfo := model.ServiceInfo{
			Name:        serviceConfig.Name,
			ImageName:   serviceConfig.Image,
			Ports:       make([]string, 0),
			Volumes:     make([]string, 0),
			EnvFiles:    make([]string, 0),
			ConfigFiles: make([]string, 0),
			Secrets:     make([]string, 0),
			DependsOn:   make([]string, 0),
		}

		// Extract ports
		for _, port := range serviceConfig.Ports {
			if port.Published != "" && port.Target != 0 {
				portMapping := fmt.Sprintf("%s:%d", port.Published, port.Target)
				serviceInfo.Ports = append(serviceInfo.Ports, portMapping)
			}
		}

		// Extract volumes
		for _, volume := range serviceConfig.Volumes {
			if volume.Type == "bind" || volume.Type == "volume" {
				volumeMapping := fmt.Sprintf("%s:%s", volume.Source, volume.Target)
				if volume.ReadOnly {
					volumeMapping += ":ro"
				}
				serviceInfo.Volumes = append(serviceInfo.Volumes, volumeMapping)

				// Only add to config files if it's explicitly marked as a config
				if volume.Type == "bind" {
					absPath := volume.Source
					if !filepath.IsAbs(absPath) {
						absPath = filepath.Join(workingDir, absPath)
					}
					// Don't automatically add files to config files
					// They will be handled as bind mounts
				}
			}
		}

		// Extract env files
		for _, envFile := range serviceConfig.EnvFile {
			serviceInfo.EnvFiles = append(serviceInfo.EnvFiles, envFile)
			// Add to env files list with absolute path
			absPath := envFile
			if !filepath.IsAbs(absPath) {
				absPath = filepath.Join(workingDir, absPath)
			}
			info.EnvFiles = append(info.EnvFiles, absPath)
		}

		// Extract secrets
		for _, secret := range serviceConfig.Secrets {
			serviceInfo.Secrets = append(serviceInfo.Secrets, secret.Source)
		}

		// Extract depends_on
		for dependName := range serviceConfig.DependsOn {
			serviceInfo.DependsOn = append(serviceInfo.DependsOn, dependName)
		}

		// Add the service info to the list
		info.Services = append(info.Services, serviceInfo)
	}

	// Process config files
	configFiles := map[string]bool{}

	for _, envFile := range info.EnvFiles {
		configFiles[envFile] = true
	}

	if p.verbose {
		p.printComposeInfo(info)
	}

	return info, nil
}

// printComposeInfo prints the compose info for debugging
func (p *ComposeParser) printComposeInfo(info *model.ComposeInfo) {
	log.Println("Docker Compose Info:")
	log.Printf("  Version: %s\n", info.Version)

	log.Println("  Services:")
	for _, service := range info.Services {
		log.Printf("    - %s (Image: %s)\n", service.Name, service.ImageName)

		if len(service.Ports) > 0 {
			log.Println("      Ports:")
			for _, port := range service.Ports {
				log.Printf("        - %s\n", port)
			}
		}

		if len(service.Volumes) > 0 {
			log.Println("      Volumes:")
			for _, volume := range service.Volumes {
				log.Printf("        - %s\n", volume)
			}
		}

		if len(service.EnvFiles) > 0 {
			log.Println("      Env Files:")
			for _, envFile := range service.EnvFiles {
				log.Printf("        - %s\n", envFile)
			}
		}

		if len(service.DependsOn) > 0 {
			log.Println("      Depends On:")
			for _, dep := range service.DependsOn {
				log.Printf("        - %s\n", dep)
			}
		}
	}

	if len(info.Networks) > 0 {
		log.Println("  Networks:")
		for _, network := range info.Networks {
			log.Printf("    - %s\n", network)
		}
	}

	if len(info.Volumes) > 0 {
		log.Println("  Volumes:")
		for _, volume := range info.Volumes {
			log.Printf("    - %s\n", volume)
		}
	}

	if len(info.EnvFiles) > 0 {
		log.Println("  Env Files:")
		for _, env := range info.EnvFiles {
			log.Printf("    - %s\n", env)
		}
	}
}

// IsComposeFile checks if the given file is a docker-compose file
func IsComposeFile(filePath string) bool {
	fileName := filepath.Base(filePath)
	return strings.HasPrefix(fileName, "docker-compose") ||
		strings.HasPrefix(fileName, "compose") ||
		strings.HasSuffix(fileName, "compose.yml") ||
		strings.HasSuffix(fileName, "compose.yaml")
}

// DetectComposeFile tries to detect a docker-compose file in the given directory
func DetectComposeFile(directory string) (string, error) {
	possibleNames := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	for _, name := range possibleNames {
		path := filepath.Join(directory, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no docker-compose file found in directory: %s", directory)
}
