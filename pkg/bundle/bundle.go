package bundle

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/iamlongalong/dockerbundle/internal/bundler"
	"github.com/iamlongalong/dockerbundle/internal/collector"
	"github.com/iamlongalong/dockerbundle/internal/model"
	"github.com/iamlongalong/dockerbundle/internal/parser"
)

// Options represents the options for bundling a docker-compose configuration
type Options struct {
	// ComposeFilePath is the path to the docker-compose.yml file
	ComposeFilePath string
	// OutputDir is the directory where the bundle will be created
	OutputDir string
	// IncludeImages determines whether to include Docker images in the bundle
	IncludeImages bool
	// IncludeVolumes determines whether to include volume data in the bundle
	IncludeVolumes bool
	// Verbose enables detailed logging
	Verbose bool
}

// Result represents the result of the bundling operation
type Result struct {
	// Resources is the list of resources included in the bundle
	Resources []model.Resource
	// ManifestPath is the path to the manifest file within the bundle
	ManifestPath string
	// TotalSize is the total size of the bundle in bytes
	TotalSize int64
	// ResourceCount is the total number of resources in the bundle
	ResourceCount int
}

// Bundle bundles a docker-compose configuration into a directory
func Bundle(options Options) (*Result, error) {
	// Create the bundle configuration
	bundleConfig := model.BundleConfig{
		ComposeFilePath: options.ComposeFilePath,
		OutputDir:       options.OutputDir,
		IncludeImages:   options.IncludeImages,
		IncludeVolumes:  options.IncludeVolumes,
		Verbose:         options.Verbose,
	}

	// Create a compose parser
	composeParser := parser.NewComposeParser(options.Verbose)

	// Parse the compose file
	if options.Verbose {
		log.Printf("Parsing compose file: %s\n", options.ComposeFilePath)
	}

	composeInfo, err := composeParser.Parse(options.ComposeFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse compose file: %w", err)
	}

	// Create a resource collector
	resourceCollector, err := collector.NewResourceCollector(options.Verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource collector: %w", err)
	}
	defer resourceCollector.Close()

	// Collect resources
	if options.Verbose {
		log.Println("Collecting resources...")
	}

	resources, err := resourceCollector.CollectResources(composeInfo, bundleConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to collect resources: %w", err)
	}

	// Create a bundler
	bundler, err := bundler.NewBundler(options.Verbose)
	if err != nil {
		return nil, fmt.Errorf("failed to create bundler: %w", err)
	}
	defer bundler.Close()

	// Bundle the resources
	if options.Verbose {
		log.Printf("Bundling resources to: %s\n", options.OutputDir)
	}

	bundle, err := bundler.Bundle(resources, options.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("failed to bundle resources: %w", err)
	}

	// Create and return the result
	result := &Result{
		Resources:     bundle.Resources,
		ManifestPath:  bundle.ManifestPath,
		TotalSize:     bundle.Stats.TotalSize,
		ResourceCount: bundle.Stats.ResourceCount,
	}

	if options.Verbose {
		log.Println("Bundle created successfully!")
		log.Printf("Output directory: %s\n", options.OutputDir)
		log.Printf("Manifest file: %s\n", filepath.Base(bundle.ManifestPath))
		log.Printf("Total size: %.2f MB\n", float64(bundle.Stats.TotalSize)/1024/1024)
		log.Printf("Resource count: %d\n", bundle.Stats.ResourceCount)
	}

	return result, nil
}

// DetectComposeFile tries to detect a docker-compose file in the given directory
func DetectComposeFile(directory string) (string, error) {
	return parser.DetectComposeFile(directory)
}
