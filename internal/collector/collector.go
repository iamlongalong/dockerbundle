package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/iamlongalong/dockerbundle/internal/model"
)

// ResourceCollector is responsible for collecting resources from a docker-compose configuration
type ResourceCollector struct {
	// dockerClient is the Docker API client
	dockerClient *client.Client
	// verbose enables detailed logging
	verbose bool
	// outputDir is the output directory for saving resources
	outputDir string
}

// NewResourceCollector creates a new ResourceCollector
func NewResourceCollector(verbose bool) (*ResourceCollector, error) {
	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &ResourceCollector{
		dockerClient: dockerClient,
		verbose:      verbose,
	}, nil
}

// Close closes the ResourceCollector
func (c *ResourceCollector) Close() error {
	if c.dockerClient != nil {
		return c.dockerClient.Close()
	}
	return nil
}

// CollectResources collects all resources needed for the docker-compose configuration
func (c *ResourceCollector) CollectResources(info *model.ComposeInfo, config model.BundleConfig) ([]model.Resource, error) {
	var resources []model.Resource

	log.Println("Starting resource collection...")

	// Collect images
	if config.IncludeImages {
		log.Println("Collecting Docker images...")
		imageResources, err := c.collectImages(info)
		if err != nil {
			return nil, err
		}
		resources = append(resources, imageResources...)
		log.Printf("Collected %d Docker images\n", len(imageResources))
	}

	// Collect config files
	log.Println("Collecting configuration files...")
	configResources, err := c.collectConfigFiles(info)
	if err != nil {
		return nil, err
	}
	resources = append(resources, configResources...)
	log.Printf("Collected %d configuration files\n", len(configResources))

	// Collect secrets
	log.Println("Collecting secrets...")
	for _, secret := range info.Secrets {
		// Check if the secret file exists
		if _, err := os.Stat(secret.Source); os.IsNotExist(err) {
			log.Printf("Warning: Secret file does not exist: %s\n", secret.Source)
			continue
		}

		resource := model.Resource{
			Type:   model.ResourceTypeConfig,
			Name:   secret.Name,
			Source: secret.Source,
			Target: secret.Target,
			Metadata: map[string]interface{}{
				"type": "secret",
				"mode": 0600, // Secrets should be readable only by owner
			},
		}
		resources = append(resources, resource)
		log.Printf("Added secret: %s\n", secret.Name)
	}

	// Include the docker-compose file itself
	log.Println("Adding docker-compose file...")
	composeFile := config.ComposeFilePath
	composeResource := model.Resource{
		Type:   model.ResourceTypeConfig,
		Name:   filepath.Base(composeFile),
		Source: composeFile,
		Target: filepath.Join("compose", filepath.Base(composeFile)),
		Metadata: map[string]interface{}{
			"size": getFileSize(composeFile),
		},
	}
	resources = append(resources, composeResource)

	// Include the restore script
	log.Println("Adding restore script...")
	// Try to find restore.sh in multiple locations
	scriptPaths := []string{
		filepath.Join(filepath.Dir(composeFile), "scripts", "restore.sh"),
		filepath.Join(filepath.Dir(filepath.Dir(config.ComposeFilePath)), "scripts", "restore.sh"),
		"scripts/restore.sh",
	}

	var scriptPath string
	for _, path := range scriptPaths {
		if _, err := os.Stat(path); err == nil {
			scriptPath = path
			break
		}
	}

	if scriptPath != "" {
		scriptResource := model.Resource{
			Type:   model.ResourceTypeConfig,
			Name:   "restore.sh",
			Source: scriptPath,
			Target: filepath.Join("scripts", "restore.sh"),
			Metadata: map[string]interface{}{
				"size": getFileSize(scriptPath),
				"mode": 0755, // Make the script executable
			},
		}
		resources = append(resources, scriptResource)
		log.Printf("Added restore script: %s\n", scriptPath)
	} else {
		log.Printf("Warning: restore.sh script not found in any of the search paths\n")
	}

	// Collect volumes if needed
	if config.IncludeVolumes {
		log.Println("Collecting volumes...")
		volumeResources, err := c.collectVolumes(info, config.ComposeFilePath)
		if err != nil {
			return nil, err
		}
		resources = append(resources, volumeResources...)
		log.Printf("Collected %d volumes\n", len(volumeResources))
	}

	log.Printf("Resource collection completed. Total resources: %d\n", len(resources))

	return resources, nil
}

// collectImages collects all Docker images
func (c *ResourceCollector) collectImages(info *model.ComposeInfo) ([]model.Resource, error) {
	var resources []model.Resource
	processedImages := make(map[string]bool)

	ctx := context.Background()
	totalImages := 0
	for _, service := range info.Services {
		if service.ImageName != "" {
			totalImages++
		}
	}

	log.Printf("Found %d unique images to process\n", totalImages)
	current := 0

	for _, service := range info.Services {
		imageName := service.ImageName
		if imageName == "" {
			continue
		}

		// Skip if we've already processed this image
		if processedImages[imageName] {
			continue
		}
		processedImages[imageName] = true
		current++

		log.Printf("Processing image %d/%d: %s\n", current, totalImages, imageName)

		// Get image details
		imageInspect, _, err := c.dockerClient.ImageInspectWithRaw(ctx, imageName)
		if err != nil {
			if client.IsErrNotFound(err) {
				log.Printf("Image not found locally, pulling: %s\n", imageName)
				reader, err := c.dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
				if err != nil {
					return nil, fmt.Errorf("failed to pull image %s: %w", imageName, err)
				}
				defer reader.Close()

				// Show pull progress
				decoder := json.NewDecoder(reader)
				for {
					var pullStatus struct {
						Status   string `json:"status"`
						Progress string `json:"progress,omitempty"`
					}
					if err := decoder.Decode(&pullStatus); err != nil {
						if err == io.EOF {
							break
						}
						return nil, err
					}
					if pullStatus.Progress != "" {
						log.Printf("\r%s: %s", pullStatus.Status, pullStatus.Progress)
					} else {
						log.Printf("\r%s", pullStatus.Status)
					}
				}
				log.Println() // New line after progress

				imageInspect, _, err = c.dockerClient.ImageInspectWithRaw(ctx, imageName)
				if err != nil {
					return nil, fmt.Errorf("failed to inspect image %s after pull: %w", imageName, err)
				}
			} else {
				return nil, fmt.Errorf("failed to inspect image %s: %w", imageName, err)
			}
		}

		// Create safe filename from image name
		safeImageName := strings.NewReplacer(
			"/", "_",
			":", "_",
			"@", "_",
		).Replace(imageName)

		// Prepare target path for the image
		targetPath := filepath.Join("images", fmt.Sprintf("%s.tar", safeImageName))

		// Create an image resource
		resource := model.Resource{
			Type:   model.ResourceTypeImage,
			Name:   imageName,
			Source: imageInspect.ID,
			Target: targetPath,
			Metadata: map[string]interface{}{
				"id":   imageInspect.ID,
				"size": imageInspect.Size,
				"tags": imageInspect.RepoTags,
			},
		}

		resources = append(resources, resource)
		log.Printf("Added image: %s (%.2f MB)\n", imageName, float64(imageInspect.Size)/1024/1024)
	}

	return resources, nil
}

// collectConfigFiles collects all configuration files used in the docker-compose configuration
func (c *ResourceCollector) collectConfigFiles(info *model.ComposeInfo) ([]model.Resource, error) {
	var resources []model.Resource
	processedFiles := make(map[string]bool)

	// Process all env files
	for _, envFile := range info.EnvFiles {
		if processedFiles[envFile] {
			continue
		}
		processedFiles[envFile] = true

		// Check if the file exists
		if _, err := os.Stat(envFile); os.IsNotExist(err) {
			if c.verbose {
				log.Printf("Warning: Env file does not exist: %s\n", envFile)
			}
			continue
		}

		// Prepare the target path
		relPath := filepath.Base(envFile)
		targetPath := filepath.Join("configs", relPath)

		// Create a config resource
		resource := model.Resource{
			Type:   model.ResourceTypeConfig,
			Name:   relPath,
			Source: envFile,
			Target: targetPath,
			Metadata: map[string]interface{}{
				"size": getFileSize(envFile),
			},
		}

		resources = append(resources, resource)

		if c.verbose {
			log.Printf("Added env resource: %s\n", envFile)
		}
	}

	return resources, nil
}

// getRelativePath returns a path relative to the compose file directory
func getRelativePath(path string, composeDir string) string {
	// If the path starts with "./", just remove it
	if strings.HasPrefix(path, "./") {
		return path[2:]
	}

	// If the path is already relative and doesn't start with "./", return as is
	if !filepath.IsAbs(path) {
		return path
	}

	// Try to make the absolute path relative to the compose directory
	relPath, err := filepath.Rel(composeDir, path)
	if err != nil {
		// If we can't make it relative, use the base name as fallback
		return filepath.Base(path)
	}

	// If the relative path starts with "../", it means the path is outside the compose directory
	// In this case, we'll use the full path without the leading "/"
	if strings.HasPrefix(relPath, "..") {
		return strings.TrimPrefix(filepath.Clean(path), "/")
	}

	return relPath
}

// collectVolumes collects all volumes
func (c *ResourceCollector) collectVolumes(info *model.ComposeInfo, composePath string) ([]model.Resource, error) {
	var resources []model.Resource
	log.Printf("Found %d volumes to process\n", len(info.Volumes))

	composeDir := filepath.Dir(composePath)

	// First collect named volumes with bind driver_opts
	for _, bindMount := range info.BindMounts {
		log.Printf("Processing bind mount volume: %s\n", bindMount.Name)

		// Get absolute source path for file operations
		sourcePath := bindMount.Source
		if !filepath.IsAbs(sourcePath) {
			sourcePath = filepath.Join(composeDir, sourcePath)
		}

		// Check if source exists
		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			log.Printf("Warning: Bind mount source does not exist: %s, skipping\n", sourcePath)
			continue
		}

		// Get relative source path for the target structure
		relSourcePath := getRelativePath(bindMount.Source, composeDir)

		// Create a volume resource
		resource := model.Resource{
			Type:   model.ResourceTypeVolume,
			Name:   bindMount.Name,
			Source: sourcePath, // Use absolute path for source (needed for file operations)
			Target: filepath.Join("volumes", "_named", bindMount.Name, relSourcePath),
			Metadata: map[string]interface{}{
				"type":     "bind",
				"target":   bindMount.Target,
				"readonly": bindMount.ReadOnly,
				"source":   bindMount.Source, // Keep original relative path
			},
		}

		resources = append(resources, resource)
		log.Printf("Added bind mount volume: %s\n", bindMount.Name)
	}

	// Then collect regular named volumes
	for _, volumeName := range info.Volumes {
		// Skip if it's already processed as a bind mount
		isBindMount := false
		for _, bindMount := range info.BindMounts {
			if bindMount.Name == volumeName {
				isBindMount = true
				break
			}
		}
		if isBindMount {
			continue
		}

		log.Printf("Processing named volume: %s\n", volumeName)

		// Named volumes require Docker volume inspection
		ctx := context.Background()
		volumeInspect, err := c.dockerClient.VolumeInspect(ctx, volumeName)
		if err != nil {
			if client.IsErrNotFound(err) {
				log.Printf("Warning: Volume %s does not exist, skipping\n", volumeName)
				continue
			}
			return nil, fmt.Errorf("failed to inspect volume %s: %w", volumeName, err)
		}

		// Create a volume resource
		resource := model.Resource{
			Type:   model.ResourceTypeVolume,
			Name:   volumeName,
			Source: volumeInspect.Mountpoint,
			Target: filepath.Join("volumes", "_named", volumeName),
			Metadata: map[string]interface{}{
				"driver": volumeInspect.Driver,
				"type":   "named",
			},
		}

		resources = append(resources, resource)
		log.Printf("Added named volume: %s (Driver: %s)\n", volumeName, volumeInspect.Driver)
	}

	// Then collect bind mounts from services
	for _, service := range info.Services {
		log.Printf("Processing volumes for service: %s\n", service.Name)

		// Create a map of named volumes for quick lookup
		namedVolumes := make(map[string]bool)
		for _, vol := range info.Volumes {
			namedVolumes[vol] = true
		}

		for _, volumeStr := range service.Volumes {
			// Parse volume string into source and target
			parts := strings.Split(volumeStr, ":")
			if len(parts) < 2 {
				continue // Skip invalid volume strings
			}
			source := parts[0]
			readOnly := len(parts) > 2 && parts[2] == "ro"

			// Skip if it's a named volume
			if namedVolumes[source] {
				continue
			}

			// Get absolute source path for file operations
			sourcePath := source
			if !filepath.IsAbs(sourcePath) {
				sourcePath = filepath.Join(composeDir, sourcePath)
			}

			// Get relative source path for the target structure
			relSourcePath := getRelativePath(source, composeDir)

			log.Printf("Processing bind mount: %s\n", sourcePath)

			// Check if source exists
			srcInfo, err := os.Stat(sourcePath)
			if os.IsNotExist(err) {
				log.Printf("Warning: Bind mount source does not exist: %s, skipping\n", sourcePath)
				continue
			}

			// Create a volume resource for the bind mount
			resource := model.Resource{
				Type:   model.ResourceTypeVolume,
				Name:   filepath.Base(sourcePath),
				Source: sourcePath, // Use absolute path for source (needed for file operations)
				Target: filepath.Join("volumes", "bind", service.Name, relSourcePath),
				Metadata: map[string]interface{}{
					"type":     "bind",
					"service":  service.Name,
					"mount":    volumeStr,
					"source":   source, // Keep original relative path
					"readonly": readOnly,
					"isdir":    srcInfo.IsDir(),
				},
			}

			resources = append(resources, resource)
			log.Printf("Added bind mount for service %s: %s\n", service.Name, sourcePath)
		}
	}

	return resources, nil
}

// getFileSize returns the size of a file in bytes
func getFileSize(filePath string) int64 {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0
	}
	return info.Size()
}

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	log.Printf("Copying directory: %s -> %s\n", src, dst)

	// Create the destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dst, err)
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string, mode ...os.FileMode) error {
	log.Printf("Copying file: %s -> %s\n", src, dst)

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", src, err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dst, err)
	}
	defer dstFile.Close()

	// Copy the contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file contents from %s to %s: %w", src, dst, err)
	}

	// Get source file mode if not specified
	var fileMode os.FileMode
	if len(mode) > 0 {
		fileMode = mode[0]
	} else {
		srcInfo, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("failed to get source file info %s: %w", src, err)
		}
		fileMode = srcInfo.Mode()
	}

	// Set the mode on destination
	return os.Chmod(dst, fileMode)
}

// SaveResources saves all collected resources to the output directory
func (c *ResourceCollector) SaveResources(resources []model.Resource, outputDir string) error {
	log.Printf("Saving resources to: %s\n", outputDir)

	// Set the output directory
	c.outputDir = outputDir

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Process each resource
	for _, resource := range resources {
		targetPath := filepath.Join(outputDir, resource.Target)
		targetDir := filepath.Dir(targetPath)

		// Create target directory
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
		}

		switch resource.Type {
		case model.ResourceTypeImage:
			log.Printf("Saving image: %s\n", resource.Name)
			if err := c.saveImage(resource.Source, targetPath); err != nil {
				return fmt.Errorf("failed to save image %s: %w", resource.Name, err)
			}

		case model.ResourceTypeVolume:
			log.Printf("Saving volume: %s\n", resource.Name)
			if err := copyDir(resource.Source, targetPath); err != nil {
				return fmt.Errorf("failed to save volume %s: %w", resource.Name, err)
			}

		case model.ResourceTypeConfig:
			log.Printf("Saving config: %s\n", resource.Name)
			if err := copyFile(resource.Source, targetPath); err != nil {
				return fmt.Errorf("failed to save config %s: %w", resource.Name, err)
			}

			// Set file mode if specified in metadata
			if mode, ok := resource.Metadata["mode"].(int); ok {
				if err := os.Chmod(targetPath, os.FileMode(mode)); err != nil {
					return fmt.Errorf("failed to set file mode for %s: %w", resource.Name, err)
				}
			}
		}
	}

	log.Println("All resources saved successfully")
	return nil
}

// saveImage saves a Docker image to a tar file
func (c *ResourceCollector) saveImage(imageID, target string) error {
	log.Printf("Saving image %s to %s\n", imageID, target)

	// Create target directory if it doesn't exist
	targetDir := filepath.Dir(target)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	// Create the target file
	targetFile, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("failed to create target file %s: %w", target, err)
	}
	defer targetFile.Close()

	// Get image reader from Docker API
	ctx := context.Background()
	reader, err := c.dockerClient.ImageSave(ctx, []string{imageID})
	if err != nil {
		return fmt.Errorf("failed to save image %s: %w", imageID, err)
	}
	defer reader.Close()

	// Copy the image data to the target file
	if _, err := io.Copy(targetFile, reader); err != nil {
		return fmt.Errorf("failed to write image data to %s: %w", target, err)
	}

	log.Printf("Successfully saved image %s\n", imageID)
	return nil
}

// saveResource saves a resource to the output directory
func (c *ResourceCollector) saveResource(resource model.Resource) error {
	targetPath := filepath.Join(c.outputDir, resource.Target)
	targetDir := filepath.Dir(targetPath)

	// Create target directory
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", targetDir, err)
	}

	switch resource.Type {
	case model.ResourceTypeImage:
		log.Printf("Saving image: %s\n", resource.Name)
		if err := c.saveImage(resource.Source, targetPath); err != nil {
			return fmt.Errorf("failed to save image %s: %w", resource.Name, err)
		}

	case model.ResourceTypeVolume:
		log.Printf("Saving volume: %s\n", resource.Name)
		if err := copyDir(resource.Source, targetPath); err != nil {
			return fmt.Errorf("failed to save volume %s: %w", resource.Name, err)
		}

	case model.ResourceTypeConfig:
		log.Printf("Saving config: %s\n", resource.Name)
		if err := copyFile(resource.Source, targetPath); err != nil {
			return fmt.Errorf("failed to save config %s: %w", resource.Name, err)
		}

		// Set file mode if specified in metadata
		if mode, ok := resource.Metadata["mode"].(int); ok {
			if err := os.Chmod(targetPath, os.FileMode(mode)); err != nil {
				return fmt.Errorf("failed to set file mode for %s: %w", resource.Name, err)
			}
		}
	}

	return nil
}
