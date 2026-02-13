package skills

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"nekobot/pkg/logger"
)

// Watcher watches skill files for changes and emits events.
type Watcher struct {
	log      *logger.Logger
	manager  *Manager
	watcher  *fsnotify.Watcher
	events   chan SkillChangeEvent
	errors   chan error
	status   WatcherStatus
	mu       sync.RWMutex
	stopOnce sync.Once
}

// NewWatcher creates a new skill file watcher.
func NewWatcher(log *logger.Logger, manager *Manager) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		log:     log,
		manager: manager,
		watcher: fsw,
		events:  make(chan SkillChangeEvent, 100),
		errors:  make(chan error, 10),
		status: WatcherStatus{
			Active:     false,
			WatchPaths: []string{},
			EventCount: 0,
			ErrorCount: 0,
		},
	}, nil
}

// Start starts watching the skills directory.
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.status.Active {
		w.mu.Unlock()
		return nil // Already watching
	}

	// Add skills directory to watch
	skillsDir := w.manager.skillsDir
	if err := w.watcher.Add(skillsDir); err != nil {
		w.mu.Unlock()
		return err
	}

	w.status.Active = true
	w.status.WatchPaths = []string{skillsDir}
	w.mu.Unlock()

	w.log.Info("Skill watcher started",
		logger.String("dir", skillsDir))

	// Start event processing
	go w.processEvents(ctx)

	return nil
}

// Stop stops the watcher.
func (w *Watcher) Stop() error {
	var err error
	w.stopOnce.Do(func() {
		w.mu.Lock()
		w.status.Active = false
		w.mu.Unlock()

		err = w.watcher.Close()
		close(w.events)
		close(w.errors)
	})
	return err
}

// Events returns the channel of skill change events.
func (w *Watcher) Events() <-chan SkillChangeEvent {
	return w.events
}

// Errors returns the channel of watcher errors.
func (w *Watcher) Errors() <-chan error {
	return w.errors
}

// Status returns the current watcher status.
func (w *Watcher) Status() WatcherStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.status
}

// processEvents processes file system events and emits skill change events.
func (w *Watcher) processEvents(ctx context.Context) {
	// Debounce timer to avoid duplicate events
	debounceTimer := make(map[string]*time.Timer)
	debounceLock := sync.Mutex{}
	debounceDelay := 100 * time.Millisecond

	for {
		select {
		case <-ctx.Done():
			w.log.Info("Skill watcher stopped")
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only process .md files
			if !strings.HasSuffix(event.Name, ".md") {
				continue
			}

			// Skip temporary files
			if strings.HasSuffix(event.Name, ".tmp") || strings.HasSuffix(event.Name, "~") {
				continue
			}

			// Debounce events for the same file
			debounceLock.Lock()
			if timer, exists := debounceTimer[event.Name]; exists {
				timer.Stop()
			}
			debounceTimer[event.Name] = time.AfterFunc(debounceDelay, func() {
				w.handleFileEvent(event)
				debounceLock.Lock()
				delete(debounceTimer, event.Name)
				debounceLock.Unlock()
			})
			debounceLock.Unlock()

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}

			w.mu.Lock()
			w.status.ErrorCount++
			w.status.LastError = err
			w.mu.Unlock()

			w.log.Warn("Skill watcher error",
				logger.Error(err))

			select {
			case w.errors <- err:
			default:
				// Error channel full, drop error
			}
		}
	}
}

// handleFileEvent handles a file system event.
func (w *Watcher) handleFileEvent(event fsnotify.Event) {
	var changeType ChangeType

	if event.Op&fsnotify.Create == fsnotify.Create {
		changeType = ChangeTypeCreated
	} else if event.Op&fsnotify.Write == fsnotify.Write {
		changeType = ChangeTypeModified
	} else if event.Op&fsnotify.Remove == fsnotify.Remove {
		changeType = ChangeTypeDeleted
	} else if event.Op&fsnotify.Rename == fsnotify.Rename {
		changeType = ChangeTypeDeleted
	} else {
		return // Ignore other events
	}

	// Extract skill name from filename
	skillName := strings.TrimSuffix(filepath.Base(event.Name), ".md")

	w.log.Debug("Skill file changed",
		logger.String("type", string(changeType)),
		logger.String("skill", skillName),
		logger.String("path", event.Name))

	// Update internal state
	w.mu.Lock()
	w.status.LastEvent = time.Now()
	w.status.EventCount++
	w.mu.Unlock()

	// Emit change event
	changeEvent := SkillChangeEvent{
		Type:      changeType,
		SkillName: skillName,
		SkillID:   skillName, // Use name as ID for now
		Path:      event.Name,
		Timestamp: time.Now(),
	}

	select {
	case w.events <- changeEvent:
		// Event sent successfully
	default:
		// Event channel full, drop event
		w.log.Warn("Skill change event dropped (channel full)",
			logger.String("skill", skillName))
	}

	// Auto-reload skill if enabled
	if w.manager.autoReload {
		w.reloadSkill(event.Name, changeType)
	}
}

// reloadSkill reloads a skill after a file change.
func (w *Watcher) reloadSkill(path string, changeType ChangeType) {
	switch changeType {
	case ChangeTypeCreated, ChangeTypeModified:
		skill, err := w.manager.loadSkillFile(path)
		if err != nil {
			w.log.Warn("Failed to reload skill",
				logger.String("path", path),
				logger.Error(err))
			return
		}

		w.manager.registerSkill(skill)
		w.log.Info("Skill reloaded",
			logger.String("id", skill.ID),
			logger.String("name", skill.Name))

	case ChangeTypeDeleted:
		// Extract skill ID from path
		skillID := strings.TrimSuffix(filepath.Base(path), ".md")

		w.manager.mu.Lock()
		delete(w.manager.skills, skillID)
		w.manager.mu.Unlock()

		w.log.Info("Skill removed",
			logger.String("id", skillID))
	}
}
