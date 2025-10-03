package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"github.com/openchami/inventory/pkg/resources"
	"github.com/openchami/inventory/pkg/versioning"
)

// FileBackend implements StorageBackend using file-based storage.
type FileBackend struct {
	baseDir         string
	mu              sync.RWMutex
	closed          bool
	versionRegistry *versioning.VersionRegistry
}

// genericFileStorage is an implementation of GenericStorage that wraps the FileBackend
// and scopes all operations to a single resource type.
type genericFileStorage struct {
	backend      *FileBackend
	resourceType string
}

// Load implements GenericStorage.Load
func (g *genericFileStorage) Load(ctx context.Context, uid string) (interface{}, error) {
	return g.backend.Load(ctx, g.resourceType, uid)
}

// LoadAll implements GenericStorage.LoadAll
func (g *genericFileStorage) LoadAll(ctx context.Context) ([]interface{}, error) {
	rawMessages, err := g.backend.LoadAll(ctx, g.resourceType)
	if err != nil {
		return nil, err
	}
	results := make([]interface{}, len(rawMessages))
	for i, v := range rawMessages {
		results[i] = v
	}
	return results, nil
}

// Delete implements GenericStorage.Delete
func (g *genericFileStorage) Delete(ctx context.Context, uid string) error {
	return g.backend.Delete(ctx, g.resourceType, uid)
}

// Exists implements GenericStorage.Exists
func (g *genericFileStorage) Exists(ctx context.Context, uid string) (bool, error) {
	return g.backend.Exists(ctx, g.resourceType, uid)
}

// List implements GenericStorage.List
func (g *genericFileStorage) List(ctx context.Context) ([]string, error) {
	return g.backend.List(ctx, g.resourceType)
}

// LoadWithVersion implements GenericStorage.LoadWithVersion
func (g *genericFileStorage) LoadWithVersion(ctx context.Context, uid string, version string) (interface{}, string, error) {
	return g.backend.LoadWithVersion(ctx, g.resourceType, uid, version)
}

// LoadAllWithVersion implements GenericStorage.LoadAllWithVersion
func (g *genericFileStorage) LoadAllWithVersion(ctx context.Context, version string) ([]interface{}, error) {
	rawMessages, err := g.backend.LoadAllWithVersion(ctx, g.resourceType, version)
	if err != nil {
		return nil, err
	}
	results := make([]interface{}, len(rawMessages))
	for i, v := range rawMessages {
		results[i] = v
	}
	return results, nil
}

func (g *genericFileStorage) Save(ctx context.Context, resource interface{}) error {
	val := reflect.ValueOf(resource)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("resource must be a pointer to a struct, but got %T", resource)
	}
	elem := val.Elem()

	// Access the nested 'Metadata' field
	metadataField := elem.FieldByName("Metadata")
	if !metadataField.IsValid() {
		return fmt.Errorf("resource of type %T is missing the 'Metadata' field", resource)
	}

	// Now, access the 'UID' field within the Metadata struct
	uidField := metadataField.FieldByName("UID")
	if !uidField.IsValid() || !uidField.CanSet() {
		return fmt.Errorf("resource of type %T is missing a settable 'UID' field within its Metadata", resource)
	}

	// Generate and set the UID
	uid, err := resources.GenerateUIDForResource(g.resourceType)
	if err != nil {
		return fmt.Errorf("failed to generate UID: %w", err)
	}
	uidField.SetString(uid)

	// Marshal the full resource object
	data, err := json.Marshal(resource)
	if err != nil {
		return fmt.Errorf("failed to marshal resource: %w", err)
	}

	// Call the underlying backend's save method
	return g.backend.Save(ctx, g.resourceType, uid, data)
}

// SaveWithVersion implements GenericStorage.SaveWithVersion
func (g *genericFileStorage) SaveWithVersion(ctx context.Context, resource interface{}, version string) error {
	val := reflect.ValueOf(resource)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("resource must be a pointer to a struct, but got %T", resource)
	}
	elem := val.Elem()

	// Access the nested 'Metadata.UID' field
	uidField := elem.FieldByName("Metadata").FieldByName("UID")
	if !uidField.IsValid() {
		return fmt.Errorf("resource of type %T is missing the 'Metadata.UID' field", resource)
	}
	uid := uidField.String()
	if uid == "" {
		return fmt.Errorf("resource UID is empty, cannot save with version")
	}

	data, err := json.Marshal(resource)
	if err != nil {
		return err
	}
	return g.backend.SaveWithVersion(ctx, g.resourceType, uid, data, version)
}

// NewFileBackend creates a new file-based storage backend.
func NewFileBackend(baseDir string) (*FileBackend, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory %s: %w", baseDir, err)
	}

	backend := &FileBackend{
		baseDir: baseDir,
	}

	return backend, nil
}

// ForType returns a generic file storage handler scoped to a specific resource type.
func (f *FileBackend) ForType(resourceType string) GenericStorage {
	return &genericFileStorage{
		backend:      f,
		resourceType: resourceType,
	}
}

// resourceTypeToDir maps resource type names to directory names
func (f *FileBackend) resourceTypeToDir(resourceType string) string {
	// =================================================================================
	// == IMPROVEMENT: Simplified the switch statement.
	// == This logic now correctly handles all resource types, including multi-word
	// == names, without needing to be manually updated.
	// =================================================================================
	return strings.ToLower(resourceType) + "s"
}

// getFilePath returns the file path for a specific resource
func (f *FileBackend) getFilePath(resourceType, uid string) string {
	dir := f.resourceTypeToDir(resourceType)
	return filepath.Join(f.baseDir, dir, uid+".json")
}

// getDirPath returns the directory path for a resource type
func (f *FileBackend) getDirPath(resourceType string) string {
	dir := f.resourceTypeToDir(resourceType)
	return filepath.Join(f.baseDir, dir)
}

// checkClosed returns an error if the backend has been closed
func (f *FileBackend) checkClosed() error {
	if f.closed {
		return fmt.Errorf("storage backend has been closed")
	}
	return nil
}

// LoadAll implements StorageBackend.LoadAll
func (f *FileBackend) LoadAll(ctx context.Context, resourceType string) ([]json.RawMessage, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkClosed(); err != nil {
		return nil, err
	}

	dirPath := f.getDirPath(resourceType)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []json.RawMessage{}, nil
		}
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	var resources []json.RawMessage
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		if !json.Valid(data) {
			continue
		}

		resources = append(resources, json.RawMessage(data))
	}

	return resources, nil
}

// Load implements StorageBackend.Load
func (f *FileBackend) Load(ctx context.Context, resourceType, uid string) (json.RawMessage, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkClosed(); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	filePath := f.getFilePath(resourceType, uid)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	if !json.Valid(data) {
		return nil, fmt.Errorf("invalid JSON in file %s: %w", filePath, ErrInvalidData)
	}

	return json.RawMessage(data), nil
}

// Save implements StorageBackend.Save
func (f *FileBackend) Save(ctx context.Context, resourceType, uid string, data json.RawMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkClosed(); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if !json.Valid(data) {
		return fmt.Errorf("invalid JSON data: %w", ErrInvalidData)
	}

	filePath := f.getFilePath(resourceType, uid)

	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	tempPath := filePath + ".tmp"

	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file %s: %w", tempPath, err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file %s to %s: %w", tempPath, filePath, err)
	}

	return nil
}

// Delete implements StorageBackend.Delete
func (f *FileBackend) Delete(ctx context.Context, resourceType, uid string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkClosed(); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	filePath := f.getFilePath(resourceType, uid)

	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file %s: %w", filePath, err)
	}

	return nil
}

// Exists implements StorageBackend.Exists
func (f *FileBackend) Exists(ctx context.Context, resourceType, uid string) (bool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkClosed(); err != nil {
		return false, err
	}

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	filePath := f.getFilePath(resourceType, uid)

	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	return true, nil
}

// List implements StorageBackend.List
func (f *FileBackend) List(ctx context.Context, resourceType string) ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkClosed(); err != nil {
		return nil, err
	}

	dirPath := f.getDirPath(resourceType)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	var uids []string
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		uid := strings.TrimSuffix(entry.Name(), ".json")
		uids = append(uids, uid)
	}

	return uids, nil
}

// Close implements StorageBackend.Close
func (f *FileBackend) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.closed = true
	return nil
}

// SetVersionRegistry sets the version registry for version-aware operations.
func (f *FileBackend) SetVersionRegistry(registry *versioning.VersionRegistry) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.versionRegistry = registry
}

// LoadWithVersion implements StorageBackend.LoadWithVersion
func (f *FileBackend) LoadWithVersion(ctx context.Context, resourceType, uid, version string) (json.RawMessage, string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkClosed(); err != nil {
		return nil, "", err
	}

	if f.versionRegistry == nil {
		return nil, "", fmt.Errorf("version registry not set")
	}

	select {
	case <-ctx.Done():
		return nil, "", ctx.Err()
	default:
	}

	rawData, err := f.Load(ctx, resourceType, uid)
	if err != nil {
		return nil, "", err
	}

	defaultVersion := f.versionRegistry.GetDefaultVersion(resourceType)
	if defaultVersion == "" {
		return rawData, "v1", nil
	}

	if version == "" || version == defaultVersion {
		return rawData, defaultVersion, nil
	}

	typeInfo, ok := f.versionRegistry.GetVersion(resourceType, version)
	if !ok {
		return nil, "", fmt.Errorf("unsupported version %s for %s", version, resourceType)
	}

	defaultTypeInfo, ok := f.versionRegistry.GetVersion(resourceType, defaultVersion)
	if !ok {
		return nil, "", fmt.Errorf("failed to get default version info")
	}

	defaultResource := defaultTypeInfo.Constructor()
	if err := json.Unmarshal(rawData, defaultResource); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal resource: %w", err)
	}

	if typeInfo.Converter != nil {
		converted, err := typeInfo.Converter.Convert(defaultResource, defaultVersion, version)
		if err != nil {
			return nil, "", fmt.Errorf("failed to convert from %s to %s: %w", defaultVersion, version, err)
		}

		convertedData, err := json.Marshal(converted)
		if err != nil {
			return nil, "", fmt.Errorf("failed to marshal converted resource: %w", err)
		}

		return json.RawMessage(convertedData), version, nil
	}

	return nil, "", fmt.Errorf("no converter available for %s version %s", resourceType, version)
}

// LoadAllWithVersion implements StorageBackend.LoadAllWithVersion
func (f *FileBackend) LoadAllWithVersion(ctx context.Context, resourceType, version string) ([]json.RawMessage, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if err := f.checkClosed(); err != nil {
		return nil, err
	}

	if f.versionRegistry == nil {
		return nil, fmt.Errorf("version registry not set")
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	rawResources, err := f.LoadAll(ctx, resourceType)
	if err != nil {
		return nil, err
	}

	defaultVersion := f.versionRegistry.GetDefaultVersion(resourceType)
	if defaultVersion == "" {
		return rawResources, nil
	}

	if version == "" || version == defaultVersion {
		return rawResources, nil
	}

	typeInfo, ok := f.versionRegistry.GetVersion(resourceType, version)
	if !ok {
		return nil, fmt.Errorf("unsupported version %s for %s", version, resourceType)
	}

	defaultTypeInfo, ok := f.versionRegistry.GetVersion(resourceType, defaultVersion)
	if !ok {
		return nil, fmt.Errorf("failed to get default version info")
	}

	if typeInfo.Converter == nil {
		return nil, fmt.Errorf("no converter available for %s version %s", resourceType, version)
	}

	var convertedResources []json.RawMessage
	for _, rawData := range rawResources {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		defaultResource := defaultTypeInfo.Constructor()
		if err := json.Unmarshal(rawData, defaultResource); err != nil {
			continue
		}

		converted, err := typeInfo.Converter.Convert(defaultResource, defaultVersion, version)
		if err != nil {
			continue
		}

		convertedData, err := json.Marshal(converted)
		if err != nil {
			continue
		}

		convertedResources = append(convertedResources, json.RawMessage(convertedData))
	}

	return convertedResources, nil
}

// SaveWithVersion implements StorageBackend.SaveWithVersion
func (f *FileBackend) SaveWithVersion(ctx context.Context, resourceType, uid string, data json.RawMessage, version string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.checkClosed(); err != nil {
		return err
	}

	if f.versionRegistry == nil {
		return fmt.Errorf("version registry not set")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	defaultVersion := f.versionRegistry.GetDefaultVersion(resourceType)
	if defaultVersion == "" {
		return f.Save(ctx, resourceType, uid, data)
	}

	if version == "" || version == defaultVersion {
		return f.Save(ctx, resourceType, uid, data)
	}

	typeInfo, ok := f.versionRegistry.GetVersion(resourceType, version)
	if !ok {
		return fmt.Errorf("unsupported version %s for %s", version, resourceType)
	}

	if typeInfo.Converter == nil {
		return fmt.Errorf("no converter available for %s version %s", resourceType, version)
	}

	resource := typeInfo.Constructor()
	if err := json.Unmarshal(data, resource); err != nil {
		return fmt.Errorf("failed to unmarshal resource: %w", err)
	}

	converted, err := typeInfo.Converter.Convert(resource, version, defaultVersion)
	if err != nil {
		return fmt.Errorf("failed to convert from %s to %s: %w", version, defaultVersion, err)
	}

	storageData, err := json.Marshal(converted)
	if err != nil {
		return fmt.Errorf("failed to marshal converted resource: %w", err)
	}

	return f.Save(ctx, resourceType, uid, json.RawMessage(storageData))
}
