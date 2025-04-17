package model

// BundleConfig represents the configuration for the docker bundle operation
type BundleConfig struct {
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
	// Compress determines whether to compress the output as tar.gz
	Compress bool
}

// ResourceType represents the type of a resource in the bundle
type ResourceType string

const (
	// ResourceTypeImage represents a Docker image
	ResourceTypeImage ResourceType = "image"
	// ResourceTypeConfig represents a configuration file
	ResourceTypeConfig ResourceType = "config"
	// ResourceTypeVolume represents volume data
	ResourceTypeVolume ResourceType = "volume"
	// ResourceTypeNetwork represents network configuration
	ResourceTypeNetwork ResourceType = "network"
)

// Resource represents a resource that needs to be bundled
type Resource struct {
	// Type is the type of the resource
	Type ResourceType
	// Name is the name/identifier of the resource
	Name string
	// Source is the source of the resource (e.g., image name, file path)
	Source string
	// Target is the target path within the bundle
	Target string
	// Metadata contains additional information about the resource
	Metadata map[string]interface{}
}

// Bundle represents the result of the bundling operation
type Bundle struct {
	// Config is the original configuration
	Config BundleConfig
	// Resources is the list of resources included in the bundle
	Resources []Resource
	// ManifestPath is the path to the manifest file within the bundle
	ManifestPath string
	// Stats contains statistics about the bundle
	Stats BundleStats
}

// BundleStats contains statistics about the bundle
type BundleStats struct {
	// ImageCount is the total number of Docker images
	ImageCount int
	// ImageSize is the total size of all images in bytes
	ImageSize int64
	// ConfigCount is the total number of config files
	ConfigCount int
	// ConfigSize is the total size of all config files in bytes
	ConfigSize int64
	// VolumeCount is the total number of volumes
	VolumeCount int
	// VolumeSize is the total size of all volumes in bytes
	VolumeSize int64
	// TotalSize is the total size of the bundle in bytes
	TotalSize int64
	// ResourceCount is the total number of resources in the bundle
	ResourceCount int
}

// Secret represents a Docker secret
type Secret struct {
	// Name is the name of the secret
	Name string
	// Type is the type of the secret (file, external, etc.)
	Type string
	// Source is the source of the secret (file path, etc.)
	Source string
	// Target is the target path within the bundle
	Target string
}

// BindMount represents a bind mount volume
type BindMount struct {
	// Name is the name of the bind mount
	Name string
	// Source is the source path on the host
	Source string
	// Target is the target path in the container
	Target string
	// ReadOnly indicates if the mount is read-only
	ReadOnly bool
}

// ServiceInfo represents information about a service defined in docker-compose
type ServiceInfo struct {
	// Name is the name of the service
	Name string
	// ImageName is the name of the image used by the service
	ImageName string
	// ImageID is the ID of the image
	ImageID string
	// Ports are the ports exposed by the service
	Ports []string
	// Volumes are the volumes used by the service
	Volumes []string
	// EnvFiles are the environment files used by the service
	EnvFiles []string
	// ConfigFiles are the config files referenced by the service
	ConfigFiles []string
	// Secrets are the secrets used by the service
	Secrets []string
	// DependsOn are the services this service depends on
	DependsOn []string
}

// ComposeInfo represents information about a docker-compose configuration
type ComposeInfo struct {
	// Version is the version of the docker-compose file
	Version string
	// Services is the list of services defined in the docker-compose file
	Services []ServiceInfo
	// Networks is the list of networks defined in the docker-compose file
	Networks []string
	// Volumes is the list of volumes defined in the docker-compose file
	Volumes []string
	// BindMounts is the list of bind mounts defined in the docker-compose file
	BindMounts []BindMount
	// ConfigFiles is the list of all config files referenced in the docker-compose file
	ConfigFiles []string
	// EnvFiles is the list of all environment files referenced in the docker-compose file
	EnvFiles []string
	// Secrets is the list of all secrets referenced in the docker-compose file
	Secrets []Secret
	// ComposeFilePath is the absolute path to the docker-compose file
	ComposeFilePath string
}

// BundleManifest represents the manifest file that describes the bundle contents
type BundleManifest struct {
	// CreatedAt is the timestamp when the bundle was created
	CreatedAt string `json:"createdAt"`
	// ComposeVersion is the version of the docker-compose file
	ComposeVersion string `json:"composeVersion"`
	// Images is the list of images included in the bundle
	Images []ImageInfo `json:"images"`
	// Configs is the list of config files included in the bundle
	Configs []ConfigInfo `json:"configs"`
	// Volumes is the list of volumes included in the bundle
	Volumes []VolumeInfo `json:"volumes"`
	// Networks is the list of networks included in the bundle
	Networks []string `json:"networks"`
}

// ImageInfo represents information about a Docker image in the bundle
type ImageInfo struct {
	// Name is the name of the image
	Name string `json:"name"`
	// ID is the ID of the image
	ID string `json:"id"`
	// Size is the size of the image in bytes
	Size int64 `json:"size"`
	// Path is the path to the image tar file within the bundle
	Path string `json:"path"`
	// Services is the list of services that use this image
	Services []string `json:"services"`
}

// ConfigInfo represents information about a config file in the bundle
type ConfigInfo struct {
	// Name is the name of the config file
	Name string `json:"name"`
	// Path is the path to the config file within the bundle
	Path string `json:"path"`
	// Size is the size of the config file in bytes
	Size int64 `json:"size"`
	// Services is the list of services that use this config file
	Services []string `json:"services"`
}

// VolumeInfo represents information about a volume in the bundle
type VolumeInfo struct {
	// Name is the name of the volume
	Name string `json:"name"`
	// Path is the path to the volume data within the bundle
	Path string `json:"path"`
	// Size is the size of the volume data in bytes
	Size int64 `json:"size"`
	// Services is the list of services that use this volume
	Services []string `json:"services"`
}
