package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FilesystemStorage implements asset storage using local filesystem
type FilesystemStorage struct {
	basePath string
}

// NewFilesystemStorage creates a new filesystem storage instance
func NewFilesystemStorage(basePath string) (*FilesystemStorage, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &FilesystemStorage{
		basePath: basePath,
	}, nil
}

// Save saves a file to the filesystem
func (fs *FilesystemStorage) Save(ctx context.Context, file io.Reader, storagePath string) error {
	// Create full path
	fullPath := filepath.Join(fs.basePath, storagePath)

	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	// Copy data
	if _, err := io.Copy(f, file); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Get retrieves a file from the filesystem
func (fs *FilesystemStorage) Get(ctx context.Context, storagePath string) (io.ReadCloser, error) {
	fullPath := filepath.Join(fs.basePath, storagePath)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("file not found: %w", err)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// Delete removes a file from the filesystem
func (fs *FilesystemStorage) Delete(ctx context.Context, storagePath string) error {
	fullPath := filepath.Join(fs.basePath, storagePath)

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, consider it already deleted
			return nil
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}
