package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/iamlongalong/dockerbundle/pkg/bundle"
)

var (
	composeFile    string
	outputDir      string
	includeImages  bool
	includeVolumes bool
	verbose        bool
	autoDetect     bool
	version        bool
)

const (
	// Version is the version of the dockerbundle tool
	Version = "0.1.0"
)

func init() {
	flag.StringVar(&composeFile, "compose", "", "Path to the docker-compose.yml file")
	flag.StringVar(&outputDir, "output", "", "Directory where the bundle will be created")
	flag.BoolVar(&includeImages, "images", true, "Include Docker images in the bundle")
	flag.BoolVar(&includeVolumes, "volumes", false, "Include volume data in the bundle")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flag.BoolVar(&autoDetect, "auto", false, "Auto-detect docker-compose file in current directory")
	flag.BoolVar(&version, "version", false, "Print version information")

	// Add shorthand flags
	flag.StringVar(&composeFile, "c", "", "Path to the docker-compose.yml file (shorthand)")
	flag.StringVar(&outputDir, "o", "", "Directory where the bundle will be created (shorthand)")
	flag.BoolVar(&includeImages, "i", true, "Include Docker images in the bundle (shorthand)")
	flag.BoolVar(&includeVolumes, "v", false, "Include volume data in the bundle (shorthand)")
	flag.BoolVar(&verbose, "V", false, "Enable verbose logging (shorthand)")
	flag.BoolVar(&autoDetect, "a", false, "Auto-detect docker-compose file in current directory (shorthand)")
}

func main() {
	// Parse flags
	flag.Parse()

	// Print version and exit if requested
	if version {
		log.Printf("dockerbundle version %s\n", Version)
		return
	}

	// Auto-detect compose file if requested
	if autoDetect && composeFile == "" {
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			os.Exit(1)
		}

		detected, err := bundle.DetectComposeFile(cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error detecting compose file: %v\n", err)
			os.Exit(1)
		}

		composeFile = detected
		log.Printf("Detected compose file: %s\n", composeFile)
	}

	// Validate required flags
	if composeFile == "" {
		fmt.Fprintf(os.Stderr, "Compose file path is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Use default output directory if not specified
	if outputDir == "" {
		baseDir := filepath.Dir(composeFile)
		timestamp := time.Now().Format("20060102-150405")
		outputDir = filepath.Join(baseDir, fmt.Sprintf("bundle-%s", timestamp))
		if verbose {
			log.Printf("Using default output directory: %s\n", outputDir)
		}
	}

	// Create bundle options
	options := bundle.Options{
		ComposeFilePath: composeFile,
		OutputDir:       outputDir,
		IncludeImages:   includeImages,
		IncludeVolumes:  includeVolumes,
		Verbose:         verbose,
	}

	// Bundle the compose configuration
	result, err := bundle.Bundle(options)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error bundling compose configuration: %v\n", err)
		os.Exit(1)
	}

	// Print summary
	log.Println("Bundle created successfully!")
	log.Printf("Output directory: %s\n", outputDir)
	log.Printf("Total size: %.2f MB\n", float64(result.TotalSize)/1024/1024)
	log.Printf("Resource count: %d\n", result.ResourceCount)
}
