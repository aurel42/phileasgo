// Package audio provides audio playback functionality for narration.
package audio

import (
	"log/slog"
	"os"
	"sync"
	"time"

	"phileasgo/pkg/config"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/effects"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
)

// Service defines the interface for audio playback control.
type Service interface {
	// Play starts playback of an audio file. If startPaused is true, loads but pauses immediately.
	// onComplete is called when playback finishes (not when stopped/paused manually).
	Play(filepath string, startPaused bool, onComplete func()) error
	// Pause pauses current playback.
	Pause()
	// Resume resumes paused playback.
	Resume()
	// Stop stops current playback.
	Stop()
	// Shutdown stops playback and cleans up resources/files.
	Shutdown()

	// IsPlaying returns true if audio is currently playing (not paused).
	IsPlaying() bool
	// IsBusy returns true if audio is loaded/playing/paused (ctrl is not nil).
	IsBusy() bool
	// IsPaused returns true if playback is paused.
	IsPaused() bool
	// SetVolume sets playback volume (0.0 to 1.0).
	SetVolume(vol float64)
	// Volume returns current volume level.
	Volume() float64
	// SetUserPaused sets user-initiated pause state for auto-select control.
	SetUserPaused(paused bool)
	// IsUserPaused returns true if user has paused auto-selection.
	IsUserPaused() bool
	// ResetUserPause resets user pause state.
	ResetUserPause()
	// LastNarrationFile returns the path of the last played narration.
	LastNarrationFile() string
	// ReplayLastNarration replays the last narration. Returns true if successful.
	ReplayLastNarration(onComplete func()) bool
	// Position returns the current playback position.
	Position() time.Duration
	// Duration returns the total duration of the current audio.
	Duration() time.Duration
	// Remaining returns the remaining time of the current playback.
	Remaining() time.Duration
}

// Manager implements the Service interface using gopxl/beep.
type Manager struct {
	mu                 sync.RWMutex
	ctrl               *beep.Ctrl
	volume             float64
	isPaused           bool
	userPaused         bool
	lastNarrationFile  string
	speakerInitialized bool
	currentSampleRate  beep.SampleRate
	streamer           *effects.Volume // Added for volume control
	trackStreamer      beep.StreamSeekCloser
	trackFormat        beep.Format
	config             *config.NarratorConfig
}

// New creates a new Manager instance.
func New(cfg *config.NarratorConfig) *Manager {
	return &Manager{
		volume: 1.0,
		config: cfg,
	}
}

// Play starts playback of an audio file.
func (m *Manager) Play(filepath string, startPaused bool, onComplete func()) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Stop any current playback and close the file handle
	m.stopLocked()

	// Open and decode audio file
	streamer, format, err := m.decodeStreamer(filepath)
	if err != nil {
		return err
	}

	// Initialize speaker once at 48kHz if not done
	if err := m.ensureSpeakerInitialized(streamer); err != nil {
		return err
	}

	// Resample streamer to target rate
	resampled := beep.Resample(3, format.SampleRate, m.currentSampleRate, streamer)

	// Apply Audio Effects (if enabled)
	var finalStreamer beep.Streamer = resampled
	if m.config != nil && m.config.AudioEffects.Headset {
		finalStreamer = NewHeadsetFilter(resampled, float64(m.currentSampleRate), m.config.AudioEffects.LowCutoff, m.config.AudioEffects.HighCutoff)
		slog.Debug("Audio: Headset effect applied",
			"low", m.config.AudioEffects.LowCutoff,
			"high", m.config.AudioEffects.HighCutoff)
	}

	// Wrap in Volume control
	// Map 0-1 linear volume to Beep logic (Base 2)
	// Simple mapping: for now we pass it through, SetVolume handles calculation
	volStreamer := &effects.Volume{
		Streamer: finalStreamer,
		Base:     2,
		Volume:   volumeToPower(m.volume),
		Silent:   m.volume <= 0.01,
	}

	m.streamer = volStreamer
	m.trackStreamer = streamer
	m.trackFormat = format

	// Wrap in control for pause/resume
	m.ctrl = &beep.Ctrl{Streamer: volStreamer, Paused: startPaused}
	m.isPaused = startPaused

	// Play with callback to clean up when done
	speaker.Play(beep.Seq(m.ctrl, beep.Callback(func() {
		// Launch goroutine to handle pause and cleanup without blocking the speaker thread
		go func() {
			// Enforce pause_between_narrations if configured
			if m.config != nil && m.config.PauseDuration > 0 {
				time.Sleep(time.Duration(m.config.PauseDuration))
			}

			m.mu.Lock()
			m.ctrl = nil
			m.isPaused = false
			m.mu.Unlock()
			streamer.Close()

			if onComplete != nil {
				onComplete()
			}
		}()
	})))

	// Check if this is a new file or replay
	if m.lastNarrationFile != "" && m.lastNarrationFile != filepath {
		oldFile := m.lastNarrationFile
		// We can safely delete the old file now that the new one is loaded
		// Note: We don't need to lock for os.Remove as it's an OS operation and we have a local copy of the path
		if err := os.Remove(oldFile); err == nil {
			slog.Debug("Audio: Cleaned up previous narration artifact", "path", oldFile)
		} else if !os.IsNotExist(err) {
			slog.Warn("Audio: Failed to cleanup previous narration artifact", "path", oldFile, "error", err)
		}
	}

	m.lastNarrationFile = filepath

	if startPaused {
		slog.Info("Loaded audio in PAUSED state", "path", filepath)
	} else {
		slog.Debug("Playing audio", "path", filepath)
	}

	return nil
}

// Pause pauses current playback.
func (m *Manager) Pause() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ctrl != nil {
		speaker.Lock()
		m.ctrl.Paused = true
		speaker.Unlock()
		m.isPaused = true
	}
}

// Resume resumes paused playback.
func (m *Manager) Resume() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ctrl != nil && m.isPaused {
		speaker.Lock()
		m.ctrl.Paused = false
		speaker.Unlock()
		m.isPaused = false
	}
}

// Stop stops current playback.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopLocked()
}

func (m *Manager) stopLocked() {
	if m.trackStreamer != nil {
		m.trackStreamer.Close()
		m.trackStreamer = nil
	}
	if m.ctrl != nil {
		speaker.Clear()
		m.ctrl = nil
		m.isPaused = false
	}
}

func (m *Manager) ensureSpeakerInitialized(streamer beep.StreamSeekCloser) error {
	const targetSampleRate = 48000
	if !m.speakerInitialized {
		err := speaker.Init(beep.SampleRate(targetSampleRate), beep.SampleRate(targetSampleRate).N(time.Second/10))
		if err != nil {
			streamer.Close()
			slog.Error("Failed to initialize speaker", "error", err)
			return err
		}
		m.speakerInitialized = true
		m.currentSampleRate = beep.SampleRate(targetSampleRate)
	}
	return nil
}

// Shutdown stops playback and deletes any residual audio artifacts.
func (m *Manager) Shutdown() {
	m.Stop()

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.lastNarrationFile != "" {
		if err := os.Remove(m.lastNarrationFile); err == nil {
			slog.Debug("Audio: Shutdown cleanup of residual artifact", "path", m.lastNarrationFile)
		} else if !os.IsNotExist(err) {
			slog.Warn("Audio: Failed to cleanup residual artifact on shutdown", "path", m.lastNarrationFile, "error", err)
		}
		m.lastNarrationFile = ""
	}
}

// IsPlaying returns true if audio is currently playing.
func (m *Manager) IsPlaying() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ctrl != nil && !m.isPaused
}

// IsBusy returns true if audio is loaded (playing or paused).
func (m *Manager) IsBusy() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.ctrl != nil
}

// IsPaused returns true if playback is paused.
func (m *Manager) IsPaused() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isPaused
}

// SetVolume sets playback volume (0.0 to 1.0).
func (m *Manager) SetVolume(vol float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if vol < 0 {
		vol = 0
	} else if vol > 1 {
		vol = 1
	}
	m.volume = vol

	// Update live streamer if playing
	if m.streamer != nil {
		speaker.Lock()
		m.streamer.Volume = volumeToPower(vol)
		m.streamer.Silent = vol <= 0.01
		speaker.Unlock()
	}
}

// Volume returns current volume level.
func (m *Manager) Volume() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.volume
}

// SetUserPaused sets user-initiated pause state.
func (m *Manager) SetUserPaused(paused bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.userPaused = paused
	slog.Debug("User pause set", "paused", paused)
}

// IsUserPaused returns true if user has paused auto-selection.
func (m *Manager) IsUserPaused() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.userPaused
}

// ResetUserPause resets user pause state.
func (m *Manager) ResetUserPause() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.userPaused {
		slog.Debug("User pause reset")
	}
	m.userPaused = false
}

// LastNarrationFile returns the path of the last played narration.
func (m *Manager) LastNarrationFile() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastNarrationFile
}

// ReplayLastNarration replays the last narration.
func (m *Manager) ReplayLastNarration(onComplete func()) bool {
	m.mu.RLock()
	lastFile := m.lastNarrationFile
	m.mu.RUnlock()

	if lastFile == "" {
		return false
	}

	// Check if file exists
	if _, err := os.Stat(lastFile); os.IsNotExist(err) {
		return false
	}

	return m.Play(lastFile, false, onComplete) == nil
}

// Position returns the current playback position.
func (m *Manager) Position() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.trackStreamer == nil || m.trackFormat.SampleRate == 0 {
		return 0
	}
	return m.trackFormat.SampleRate.D(m.trackStreamer.Position())
}

// Duration returns the total duration of the current audio.
func (m *Manager) Duration() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.trackStreamer == nil || m.trackFormat.SampleRate == 0 {
		return 0
	}
	return m.trackFormat.SampleRate.D(m.trackStreamer.Len())
}

// Remaining returns the remaining time of the current playback.
func (m *Manager) Remaining() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.trackStreamer == nil || m.trackFormat.SampleRate == 0 {
		return 0
	}
	// beep.StreamSeekCloser.Len() returns total samples, Position() returns current sample index
	// So remaining = (Len - Position) / SampleRate
	remainingSamples := m.trackStreamer.Len() - m.trackStreamer.Position()
	if remainingSamples < 0 {
		return 0
	}
	return m.trackFormat.SampleRate.D(remainingSamples)
}

func (m *Manager) decodeStreamer(path string) (beep.StreamSeekCloser, beep.Format, error) {
	f, err := os.Open(path)
	if err != nil {
		slog.Error("Failed to open audio file", "path", path, "error", err)
		return nil, beep.Format{}, err
	}

	// Try MP3 first
	streamer, format, err := mp3.Decode(f)
	if err == nil {
		return streamer, format, nil
	}

	// Reopen file for WAV attempt (MP3 decode failure might leave file state uncertain)
	f.Close()
	f, err = os.Open(path)
	if err != nil {
		return nil, beep.Format{}, err
	}
	defer func() {
		if err != nil {
			f.Close()
		}
	}()

	streamer, format, err = wav.Decode(f)
	if err != nil {
		slog.Error("Failed to decode audio file", "path", path, "error", err)
		return nil, beep.Format{}, err
	}

	return streamer, format, nil
}
