package bundler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"
	"github.com/iamlongalong/dockerbundle/internal/model"
)

// Bundler is responsible for bundling resources into a target directory
type Bundler struct {
	// dockerClient is the Docker API client
	dockerClient *client.Client
	// verbose enables detailed logging
	verbose bool
}

// NewBundler creates a new Bundler
func NewBundler(verbose bool) (*Bundler, error) {
	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Bundler{
		dockerClient: dockerClient,
		verbose:      verbose,
	}, nil
}

// Close closes the Bundler
func (b *Bundler) Close() error {
	if b.dockerClient != nil {
		return b.dockerClient.Close()
	}
	return nil
}

// Bundle bundles the resources into the target directory
func (b *Bundler) Bundle(resources []model.Resource, outputDir string) (*model.Bundle, error) {
	// Create the bundle output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create subdirectories
	if err := b.createSubdirectories(outputDir); err != nil {
		return nil, err
	}

	bundleStats := model.BundleStats{}
	manifestData := model.BundleManifest{
		CreatedAt: time.Now().Format(time.RFC3339),
		Images:    []model.ImageInfo{},
		Configs:   []model.ConfigInfo{},
		Volumes:   []model.VolumeInfo{},
		Networks:  []string{},
	}

	// Process each resource
	for i, resource := range resources {
		if b.verbose {
			log.Printf("Processing resource %d/%d: %s\n", i+1, len(resources), resource.Name)
		}

		switch resource.Type {
		case model.ResourceTypeImage:
			imageStats, err := b.bundleImage(resource, outputDir)
			if err != nil {
				return nil, fmt.Errorf("failed to bundle image %s: %w", resource.Name, err)
			}
			bundleStats.ImageSize += imageStats.Size
			bundleStats.ResourceCount++

			// Add to manifest
			manifestData.Images = append(manifestData.Images, model.ImageInfo{
				Name:     resource.Name,
				ID:       resource.Metadata["id"].(string),
				Size:     imageStats.Size,
				Path:     resource.Target,
				Services: []string{}, // Will be populated later
			})

		case model.ResourceTypeConfig:
			configStats, err := b.bundleConfig(resource, outputDir)
			if err != nil {
				return nil, fmt.Errorf("failed to bundle config %s: %w", resource.Name, err)
			}
			bundleStats.ConfigSize += configStats.Size
			bundleStats.ResourceCount++

			// Add to manifest
			if resource.Source != resource.Name { // Only add real configs, not the compose file
				manifestData.Configs = append(manifestData.Configs, model.ConfigInfo{
					Name:     resource.Name,
					Path:     resource.Target,
					Size:     configStats.Size,
					Services: []string{}, // Will be populated later
				})
			}

		case model.ResourceTypeVolume:
			volumeStats, err := b.bundleVolume(resource, outputDir)
			if err != nil {
				return nil, fmt.Errorf("failed to bundle volume %s: %w", resource.Name, err)
			}
			bundleStats.VolumeSize += volumeStats.Size
			bundleStats.ResourceCount++

			// Add to manifest
			manifestData.Volumes = append(manifestData.Volumes, model.VolumeInfo{
				Name:     resource.Name,
				Path:     resource.Target,
				Size:     volumeStats.Size,
				Services: []string{}, // Will be populated later
			})

		case model.ResourceTypeNetwork:
			manifestData.Networks = append(manifestData.Networks, resource.Name)
			bundleStats.ResourceCount++
		}
	}

	// Calculate total size
	bundleStats.TotalSize = bundleStats.ImageSize + bundleStats.ConfigSize + bundleStats.VolumeSize

	// Write manifest file
	manifestPath := filepath.Join(outputDir, "manifest.json")
	if err := b.writeManifest(manifestData, manifestPath); err != nil {
		return nil, fmt.Errorf("failed to write manifest file: %w", err)
	}

	// Create and return the bundle
	bundle := &model.Bundle{
		Resources:    resources,
		ManifestPath: manifestPath,
		Stats:        bundleStats,
	}

	if b.verbose {
		b.printBundleStats(bundle)
	}

	return bundle, nil
}

// createSubdirectories creates the necessary subdirectories in the output directory
func (b *Bundler) createSubdirectories(outputDir string) error {
	subdirs := []string{"images", "configs", "volumes", "compose"}
	for _, subdir := range subdirs {
		dirPath := filepath.Join(outputDir, subdir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create subdirectory %s: %w", subdir, err)
		}
	}
	return nil
}

// bundleImage bundles a Docker image resource
func (b *Bundler) bundleImage(resource model.Resource, outputDir string) (struct{ Size int64 }, error) {
	stats := struct{ Size int64 }{0}

	ctx := context.Background()
	targetPath := filepath.Join(outputDir, resource.Target)

	if b.verbose {
		log.Printf("Saving image: %s to %s\n", resource.Name, targetPath)
	}

	// Create the target directory if it doesn't exist
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return stats, fmt.Errorf("failed to create target directory: %w", err)
	}

	// Save the image to a tar file
	imageReader, err := b.dockerClient.ImageSave(ctx, []string{resource.Source})
	if err != nil {
		return stats, fmt.Errorf("failed to save image: %w", err)
	}
	defer imageReader.Close()

	// Create the target file
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return stats, fmt.Errorf("failed to create target file: %w", err)
	}
	defer targetFile.Close()

	// Copy the image data
	written, err := io.Copy(targetFile, imageReader)
	if err != nil {
		return stats, fmt.Errorf("failed to copy image data: %w", err)
	}

	stats.Size = written

	if b.verbose {
		log.Printf("Saved image %s to %s (%.2f MB)\n", resource.Name, targetPath, float64(written)/1024/1024)
	}

	return stats, nil
}

// bundleConfig bundles a configuration file resource
func (b *Bundler) bundleConfig(resource model.Resource, outputDir string) (struct{ Size int64 }, error) {
	stats := struct{ Size int64 }{0}

	targetPath := filepath.Join(outputDir, resource.Target)

	if b.verbose {
		log.Printf("Copying config: %s to %s\n", resource.Source, targetPath)
	}

	// Create the target directory if it doesn't exist
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return stats, fmt.Errorf("failed to create target directory: %w", err)
	}

	// Copy the file
	sourceFile, err := os.Open(resource.Source)
	if err != nil {
		return stats, fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create the target file
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return stats, fmt.Errorf("failed to create target file: %w", err)
	}
	defer targetFile.Close()

	// Copy the file data
	written, err := io.Copy(targetFile, sourceFile)
	if err != nil {
		return stats, fmt.Errorf("failed to copy file data: %w", err)
	}

	stats.Size = written

	if b.verbose {
		log.Printf("Copied config %s to %s (%.2f KB)\n", resource.Source, targetPath, float64(written)/1024)
	}

	return stats, nil
}

// bundleVolume bundles a volume resource
func (b *Bundler) bundleVolume(resource model.Resource, outputDir string) (struct{ Size int64 }, error) {
	stats := struct{ Size int64 }{0}

	sourcePath := resource.Source
	targetPath := filepath.Join(outputDir, resource.Target)

	if b.verbose {
		log.Printf("Copying volume: %s to %s\n", sourcePath, targetPath)
	}

	// Check if the source path exists
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return stats, fmt.Errorf("failed to access volume source: %w", err)
	}

	// Create the target directory
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return stats, fmt.Errorf("failed to create target directory: %w", err)
	}

	// If the source is a directory, copy its contents
	if sourceInfo.IsDir() {
		size, err := b.copyDirectory(sourcePath, targetPath)
		if err != nil {
			return stats, fmt.Errorf("failed to copy volume directory: %w", err)
		}
		stats.Size = size
	} else {
		// If it's a file, copy it directly
		targetFilePath := filepath.Join(targetPath, filepath.Base(sourcePath))
		size, err := b.copyFile(sourcePath, targetFilePath)
		if err != nil {
			return stats, fmt.Errorf("failed to copy volume file: %w", err)
		}
		stats.Size = size
	}

	if b.verbose {
		log.Printf("Copied volume %s to %s (%.2f MB)\n", resource.Name, targetPath, float64(stats.Size)/1024/1024)
	}

	return stats, nil
}

// copyFile copies a file from source to target and returns the number of bytes copied
func (b *Bundler) copyFile(source, target string) (int64, error) {
	sourceFile, err := os.Open(source)
	if err != nil {
		return 0, fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create target file
	targetFile, err := os.Create(target)
	if err != nil {
		return 0, fmt.Errorf("failed to create target file: %w", err)
	}
	defer targetFile.Close()

	// Copy the content
	return io.Copy(targetFile, sourceFile)
}

// copyDirectory recursively copies a directory from source to target and returns the total size
func (b *Bundler) copyDirectory(source, target string) (int64, error) {
	var size int64

	// Create the target directory
	if err := os.MkdirAll(target, 0755); err != nil {
		return 0, fmt.Errorf("failed to create target directory: %w", err)
	}

	// Walk through the source directory
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate the target path
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}
		targetPath := filepath.Join(target, rel)

		// If it's a directory, create it
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// If it's a file, copy it
		copiedSize, err := b.copyFile(path, targetPath)
		if err != nil {
			return fmt.Errorf("failed to copy file %s: %w", path, err)
		}
		size += copiedSize

		return nil
	})

	return size, err
}

// writeManifest writes the manifest data to a file
func (b *Bundler) writeManifest(manifest model.BundleManifest, path string) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest file: %w", err)
	}

	if b.verbose {
		log.Printf("Wrote manifest to %s\n", path)
	}

	return nil
}

// printBundleStats prints statistics about the bundle
func (b *Bundler) printBundleStats(bundle *model.Bundle) {
	log.Println("Bundle statistics:")
	log.Printf("  Total resources: %d\n", bundle.Stats.ResourceCount)
	log.Printf("  Total size: %.2f MB\n", float64(bundle.Stats.TotalSize)/1024/1024)
	log.Printf("  Image size: %.2f MB\n", float64(bundle.Stats.ImageSize)/1024/1024)
	log.Printf("  Config size: %.2f KB\n", float64(bundle.Stats.ConfigSize)/1024)
	log.Printf("  Volume size: %.2f MB\n", float64(bundle.Stats.VolumeSize)/1024/1024)
	log.Printf("  Manifest: %s\n", bundle.ManifestPath)
}
