package domain

import (
	"fmt"
	"reflect"
	"time"
)

// Version represents a model version
type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// VersionedModel interface defines methods for versioned models
type VersionedModel interface {
	Model
	GetVersion() Version
	SetVersion(Version)
}

// VersionedBaseModel extends BaseModel with versioning support
type VersionedBaseModel struct {
	BaseModel
	Version Version `json:"version"`
}

// GetVersion returns the model's version
func (m *VersionedBaseModel) GetVersion() Version {
	return m.Version
}

// SetVersion sets the model's version
func (m *VersionedBaseModel) SetVersion(v Version) {
	m.Version = v
}

// VersionError represents a version-related error
type VersionError struct {
	Message string
	Err     error
}

func (e *VersionError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// VersionCompatibilityError represents a version compatibility error
type VersionCompatibilityError struct {
	Current  Version
	Required Version
	Message  string
}

func (e *VersionCompatibilityError) Error() string {
	return fmt.Sprintf("version compatibility error: %s (current: %s, required: %s)",
		e.Message, e.Current, e.Required)
}

// VersionCheck checks if the current version is compatible with the required version
func VersionCheck(current, required Version) error {
	if current.Major != required.Major {
		return &VersionCompatibilityError{
			Current:  current,
			Required: required,
			Message:  "major version mismatch",
		}
	}
	if current.Minor < required.Minor {
		return &VersionCompatibilityError{
			Current:  current,
			Required: required,
			Message:  "minor version too old",
		}
	}
	return nil
}

// MigrationFunc defines a function that migrates a model from one version to another
type MigrationFunc func(interface{}) (interface{}, error)

// MigrationRegistry stores migration functions for different model types and versions
type MigrationRegistry struct {
	migrations map[reflect.Type]map[Version]MigrationFunc
}

// NewMigrationRegistry creates a new MigrationRegistry
func NewMigrationRegistry() *MigrationRegistry {
	return &MigrationRegistry{
		migrations: make(map[reflect.Type]map[Version]MigrationFunc),
	}
}

// RegisterMigration registers a migration function for a specific model type and version
func (r *MigrationRegistry) RegisterMigration(modelType reflect.Type, fromVersion Version, migration MigrationFunc) {
	if _, exists := r.migrations[modelType]; !exists {
		r.migrations[modelType] = make(map[Version]MigrationFunc)
	}
	r.migrations[modelType][fromVersion] = migration
}

// Migrate migrates a model to the target version
func (r *MigrationRegistry) Migrate(model interface{}, targetVersion Version) (interface{}, error) {
	modelType := reflect.TypeOf(model)
	migrations, exists := r.migrations[modelType]
	if !exists {
		return nil, &VersionError{
			Message: "no migrations found for model type",
		}
	}

	versionedModel, ok := model.(VersionedModel)
	if !ok {
		return nil, &VersionError{
			Message: "model does not implement VersionedModel interface",
		}
	}

	currentVersion := versionedModel.GetVersion()
	if currentVersion == targetVersion {
		return model, nil
	}

	migration, exists := migrations[currentVersion]
	if !exists {
		return nil, &VersionError{
			Message: fmt.Sprintf("no migration found from version %s", currentVersion),
		}
	}

	migratedModel, err := migration(model)
	if err != nil {
		return nil, &VersionError{
			Message: "migration failed",
			Err:     err,
		}
	}

	return migratedModel, nil
}

// NewVersionedBaseModel creates a new VersionedBaseModel with current timestamps and initial version
func NewVersionedBaseModel() *VersionedBaseModel {
	now := time.Now()
	return &VersionedBaseModel{
		BaseModel: BaseModel{
			CreatedAt: now,
			UpdatedAt: now,
		},
		Version: Version{Major: 1, Minor: 0, Patch: 0},
	}
}
