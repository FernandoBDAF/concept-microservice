package config

import (
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Reloader handles configuration file watching and reloading
type Reloader struct {
	config     *Config
	configPath string
	watcher    *fsnotify.Watcher
	mu         sync.RWMutex
	callbacks  []func(*Config)
	stop       chan struct{}
}

// NewReloader creates a new configuration reloader
func NewReloader(config *Config, configPath string) (*Reloader, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	return &Reloader{
		config:     config,
		configPath: configPath,
		watcher:    watcher,
		callbacks:  make([]func(*Config), 0),
		stop:       make(chan struct{}),
	}, nil
}

// Start begins watching the configuration file for changes
func (r *Reloader) Start() error {
	// Watch the directory containing the config file
	configDir := filepath.Dir(r.configPath)
	if err := r.watcher.Add(configDir); err != nil {
		return fmt.Errorf("failed to watch config directory: %w", err)
	}

	go r.watch()
	return nil
}

// Stop stops watching the configuration file
func (r *Reloader) Stop() {
	close(r.stop)
	r.watcher.Close()
}

// OnReload registers a callback function to be called when the configuration is reloaded
func (r *Reloader) OnReload(callback func(*Config)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.callbacks = append(r.callbacks, callback)
}

// watch watches for configuration file changes
func (r *Reloader) watch() {
	for {
		select {
		case <-r.stop:
			return
		case event, ok := <-r.watcher.Events:
			if !ok {
				return
			}

			// Check if the event is for our config file
			if filepath.Clean(event.Name) != filepath.Clean(r.configPath) {
				continue
			}

			// Handle file events
			switch {
			case event.Has(fsnotify.Write):
				// Wait a bit to ensure the file is completely written
				time.Sleep(100 * time.Millisecond)
				if err := r.reload(); err != nil {
					fmt.Printf("Failed to reload config: %v\n", err)
				}
			case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
				// File was removed or renamed, try to watch it again
				if err := r.watcher.Add(r.configPath); err != nil {
					fmt.Printf("Failed to watch config file: %v\n", err)
				}
			}

		case err, ok := <-r.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("Watcher error: %v\n", err)
		}
	}
}

// reload reloads the configuration file
func (r *Reloader) reload() error {
	// Load new configuration
	newConfig, err := LoadConfig(r.configPath)
	if err != nil {
		return fmt.Errorf("failed to load new config: %w", err)
	}

	// Update the configuration
	r.mu.Lock()
	r.config = newConfig
	callbacks := make([]func(*Config), len(r.callbacks))
	copy(callbacks, r.callbacks)
	r.mu.Unlock()

	// Notify callbacks
	for _, callback := range callbacks {
		callback(newConfig)
	}

	return nil
}

// GetConfig returns the current configuration
func (r *Reloader) GetConfig() *Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}
