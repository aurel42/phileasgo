package sapi

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"

	"phileasgo/pkg/tts"
)

// Provider implements tts.Provider using Windows SAPI5 via OLE.
type Provider struct {
	mu sync.Mutex
}

// NewProvider creates a new SAPI5 provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Synthesize generates a .wav file using SAPI5.
func (p *Provider) Synthesize(ctx context.Context, text, voiceID, outputPath string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := ole.CoInitialize(0); err != nil {
		// Already initialized
	} else {
		defer ole.CoUninitialize()
	}

	unknown, err := oleutil.CreateObject("SAPI.SpVoice")
	if err != nil {
		return "", fmt.Errorf("failed to create SAPI.SpVoice: %w", err)
	}
	voice, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		unknown.Release()
		return "", fmt.Errorf("QueryInterface SpVoice failed: %w", err)
	}
	defer voice.Release()

	if voiceID != "" {
		p.setVoiceByID(voice, voiceID)
	}

	unknownStream, err := oleutil.CreateObject("SAPI.SpFileStream")
	if err != nil {
		return "", fmt.Errorf("failed to create SAPI.SpFileStream: %w", err)
	}
	stream, err := unknownStream.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		unknownStream.Release()
		return "", fmt.Errorf("QueryInterface SpFileStream failed: %w", err)
	}
	defer stream.Release()

	fullPath := outputPath
	if !strings.HasSuffix(strings.ToLower(fullPath), ".wav") {
		fullPath += ".wav"
	}
	_, err = oleutil.CallMethod(stream, "Open", fullPath, 3, false)
	if err != nil {
		return "", fmt.Errorf("stream Open failed: %w", err)
	}
	defer func() {
		_, _ = oleutil.CallMethod(stream, "Close")
	}()

	_, err = oleutil.PutPropertyRef(voice, "AudioOutputStream", stream)
	if err != nil {
		return "", fmt.Errorf("failed to set AudioOutputStream: %w", err)
	}

	cleanText := tts.StripSpeakerLabels(text)

	_, err = oleutil.CallMethod(voice, "Speak", cleanText, 0)
	if err != nil {
		tts.Log("SAPI", cleanText, 0, err)
		return "", fmt.Errorf("Speak failed: %w", err)
	}

	tts.Log("SAPI", cleanText, 200, nil)

	return "wav", nil
}

// Voices lists available SAPI voices.
func (p *Provider) Voices(ctx context.Context) ([]tts.Voice, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := ole.CoInitialize(0); err != nil {
	} else {
		defer ole.CoUninitialize()
	}

	unknown, err := oleutil.CreateObject("SAPI.SpVoice")
	if err != nil {
		return nil, err
	}
	voice, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		unknown.Release()
		return nil, err
	}
	defer voice.Release()

	// GetVoices returns ISpeechObjectTokens.
	tokensVar, err := oleutil.CallMethod(voice, "GetVoices")
	if err != nil {
		tokensVar, err = oleutil.GetProperty(voice, "Voices")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get voices collection: %w", err)
	}
	tokens := tokensVar.ToIDispatch()
	if tokens == nil {
		return nil, fmt.Errorf("voices collection is nil")
	}
	defer tokens.Release()

	countVar, err := oleutil.GetProperty(tokens, "Count")
	if err != nil {
		return nil, fmt.Errorf("GetVoices Count failed: %w", err)
	}

	count := p.getVariantInt(countVar)

	var voices []tts.Voice
	_ = oleutil.ForEach(tokens, func(v *ole.VARIANT) error {
		if voice, ok := p.extractVoice(v); ok {
			voices = append(voices, voice)
		}
		return nil
	})

	if len(voices) == 0 {
		voices = p.fallbackManualEnum(tokens, count)
	}

	return voices, nil
}

func (p *Provider) getVariantInt(v *ole.VARIANT) int {
	val := v.Value()
	if val == nil {
		return int(v.Val)
	}
	switch it := val.(type) {
	case int32:
		return int(it)
	case int64:
		return int(it)
	case int:
		return it
	case uint32:
		return int(it)
	default:
		return int(v.Val)
	}
}

func (p *Provider) extractVoice(v *ole.VARIANT) (tts.Voice, bool) {
	item := v.ToIDispatch()
	if item == nil {
		return tts.Voice{}, false
	}
	defer item.Release()

	idVar, idErr := oleutil.CallMethod(item, "GetId")
	descVar, descErr := oleutil.CallMethod(item, "GetDescription", int32(0))

	if idErr == nil && descErr == nil && idVar != nil && descVar != nil {
		return tts.Voice{
			ID:   idVar.ToString(),
			Name: descVar.ToString(),
		}, true
	}
	return tts.Voice{}, false
}

func (p *Provider) fallbackManualEnum(tokens *ole.IDispatch, count int) []tts.Voice {
	var voices []tts.Voice
	for i := 0; i < count; i++ {
		itemVar, err := oleutil.GetProperty(tokens, "Item", i)
		if err != nil {
			itemVar, err = oleutil.CallMethod(tokens, "Item", i)
		}
		if err != nil {
			continue
		}
		item := itemVar.ToIDispatch()
		if item == nil {
			continue
		}
		idVar, _ := oleutil.CallMethod(item, "GetId")
		descVar, _ := oleutil.CallMethod(item, "GetDescription", int32(0))
		if idVar != nil && descVar != nil {
			voices = append(voices, tts.Voice{
				ID:   idVar.ToString(),
				Name: descVar.ToString(),
			})
		}
		item.Release()
	}
	return voices
}

func (p *Provider) setVoiceByID(voice *ole.IDispatch, voiceID string) {
	tokensVar, err := oleutil.CallMethod(voice, "GetVoices", "", "")
	if err != nil {
		return
	}
	tokens := tokensVar.ToIDispatch()
	defer tokens.Release()

	_ = oleutil.ForEach(tokens, func(v *ole.VARIANT) error {
		item := v.ToIDispatch()
		if item == nil {
			return nil
		}
		defer item.Release()
		idVar, _ := oleutil.CallMethod(item, "GetId")
		if idVar != nil && idVar.ToString() == voiceID {
			_, _ = oleutil.PutPropertyRef(voice, "Voice", item)
		}
		return nil
	})
}
