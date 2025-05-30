package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Version represents a semantic version (MAJOR.MINOR.PATCH)
type Version struct {
	Major int
	Minor int
	Patch int
}

// NewVersion creates a new Version from a string
func NewVersion(version string) (*Version, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid version format: %s", version)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid patch version: %s", parts[2])
	}

	return &Version{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil
}

// String returns the string representation of the version
func (v *Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare compares this version with another version
// Returns:
//
//	-1 if this version is less than the other version
//	 0 if the versions are equal
//	 1 if this version is greater than the other version
func (v *Version) Compare(other *Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}

	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}

	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	return 0
}

// IsCompatible checks if this version is compatible with another version
// Two versions are compatible if they have the same major version
func (v *Version) IsCompatible(other *Version) bool {
	return v.Major == other.Major
}

// VersionManager handles configuration versioning and migrations
type VersionManager struct {
	currentVersion *Version
	migrations     map[string]MigrationFunc
}

// MigrationFunc is a function that migrates configuration from one version to another
type MigrationFunc func(*Config) error

// NewVersionManager creates a new VersionManager
func NewVersionManager(currentVersion string) (*VersionManager, error) {
	version, err := NewVersion(currentVersion)
	if err != nil {
		return nil, err
	}

	return &VersionManager{
		currentVersion: version,
		migrations:     make(map[string]MigrationFunc),
	}, nil
}

// RegisterMigration registers a migration function for a specific version
func (vm *VersionManager) RegisterMigration(version string, migration MigrationFunc) error {
	_, err := NewVersion(version)
	if err != nil {
		return err
	}

	vm.migrations[version] = migration
	return nil
}

// Migrate migrates a configuration to the current version
func (vm *VersionManager) Migrate(config *Config) error {
	configVersion, err := NewVersion(config.Version)
	if err != nil {
		return fmt.Errorf("invalid config version: %s", config.Version)
	}

	// If versions are equal, no migration needed
	if configVersion.Compare(vm.currentVersion) == 0 {
		return nil
	}

	// If config version is newer than current version, error
	if configVersion.Compare(vm.currentVersion) > 0 {
		return fmt.Errorf("config version %s is newer than current version %s",
			configVersion, vm.currentVersion)
	}

	// Apply migrations in order
	current := configVersion
	for current.Compare(vm.currentVersion) < 0 {
		next := &Version{
			Major: current.Major,
			Minor: current.Minor,
			Patch: current.Patch + 1,
		}

		// If patch version overflows, increment minor
		if next.Patch > 9 {
			next.Patch = 0
			next.Minor++
		}

		// If minor version overflows, increment major
		if next.Minor > 9 {
			next.Minor = 0
			next.Major++
		}

		// Apply migration if exists
		if migration, exists := vm.migrations[next.String()]; exists {
			if err := migration(config); err != nil {
				return fmt.Errorf("migration to version %s failed: %v",
					next, err)
			}
		}

		// Update config version
		config.Version = next.String()
		current = next
	}

	return nil
}

// GetCurrentVersion returns the current version
func (vm *VersionManager) GetCurrentVersion() string {
	return vm.currentVersion.String()
}
