package announcement

import (
	"sync"

	"phileasgo/pkg/model"
)

// Base provides a thread-safe foundation for implementing Announcements.
type Base struct {
	mu            sync.RWMutex
	id            string
	narrativeType model.NarrativeType
	repeatable    bool
	status        Status
	held          *model.Narrative

	// Infrastructure
	DataProvider DataProvider
	Events       EventRecorder // Specialized interface for logging

	// UI metadata
	title     string
	summary   string
	imagePath string
	poi       *model.POI
}

func NewBase(id string, nType model.NarrativeType, repeatable bool, dp DataProvider, events EventRecorder) *Base {
	return &Base{
		id:            id,
		narrativeType: nType,
		repeatable:    repeatable,
		status:        StatusIdle,
		DataProvider:  dp,
		Events:        events,
	}
}

func (b *Base) ID() string                { return b.id }
func (b *Base) Type() model.NarrativeType { return b.narrativeType }
func (b *Base) IsRepeatable() bool        { return b.repeatable }

func (b *Base) Status() Status {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.status
}

func (b *Base) SetStatus(s Status) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status = s
}

func (b *Base) GetHeldNarrative() *model.Narrative {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.held
}

func (a *Base) SetHeldNarrative(n *model.Narrative) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.held = n
}

func (a *Base) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status = StatusIdle
	a.held = nil
}

// UI Metadata defaults (can be overridden by embedding struct)
func (b *Base) Title() string     { return b.title }
func (b *Base) Summary() string   { return b.summary }
func (b *Base) ImagePath() string { return b.imagePath }
func (b *Base) POI() *model.POI   { return b.poi }

func (b *Base) SetPOI(p *model.POI) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.poi = p
}

// SetUIMetadata allows manual setup if not dynamic
func (b *Base) SetUIMetadata(title, summary, imagePath string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.title = title
	b.summary = summary
	b.imagePath = imagePath
}
