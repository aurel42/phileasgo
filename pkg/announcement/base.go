package announcement

import (
	"sync"

	"phileasgo/pkg/model"
)

// BaseAnnouncement provides a thread-safe foundation for implementing Announcements.
type BaseAnnouncement struct {
	mu            sync.RWMutex
	id            string
	narrativeType model.NarrativeType
	repeatable    bool
	status        Status
	held          *model.Narrative

	// UI metadata
	title     string
	summary   string
	imagePath string
	poi       *model.POI
}

func NewBaseAnnouncement(id string, nType model.NarrativeType, repeatable bool) *BaseAnnouncement {
	return &BaseAnnouncement{
		id:            id,
		narrativeType: nType,
		repeatable:    repeatable,
		status:        StatusIdle,
	}
}

func (b *BaseAnnouncement) ID() string                { return b.id }
func (b *BaseAnnouncement) Type() model.NarrativeType { return b.narrativeType }
func (b *BaseAnnouncement) IsRepeatable() bool        { return b.repeatable }

func (b *BaseAnnouncement) Status() Status {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.status
}

func (b *BaseAnnouncement) SetStatus(s Status) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.status = s
}

func (b *BaseAnnouncement) GetHeldNarrative() *model.Narrative {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.held
}

func (a *BaseAnnouncement) SetHeldNarrative(n *model.Narrative) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.held = n
}

func (a *BaseAnnouncement) Reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status = StatusIdle
	a.held = nil
}

// UI Metadata defaults (can be overridden by embedding struct)
func (b *BaseAnnouncement) Title() string     { return b.title }
func (b *BaseAnnouncement) Summary() string   { return b.summary }
func (b *BaseAnnouncement) ImagePath() string { return b.imagePath }
func (b *BaseAnnouncement) POI() *model.POI   { return b.poi }

func (b *BaseAnnouncement) SetPOI(p *model.POI) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.poi = p
}

// SetUIMetadata allows manual setup if not dynamic
func (b *BaseAnnouncement) SetUIMetadata(title, summary, imagePath string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.title = title
	b.summary = summary
	b.imagePath = imagePath
}
