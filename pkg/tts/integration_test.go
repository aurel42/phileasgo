package tts_test

import (
	"context"
	"os"
	"runtime"
	"testing"

	"phileasgo/pkg/tracker"
	"phileasgo/pkg/tts/edgetts"
	"phileasgo/pkg/tts/sapi"
)

func TestLocal_SAPI(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("SAPI only works on Windows")
	}
	if os.Getenv("TEST_TTS") == "" {
		t.Skip("Set TEST_TTS=1 to run SAPI integration test")
	}

	p := sapi.NewProvider(nil)
	outputPath := "test_sapi.wav"
	defer os.Remove(outputPath)

	format, err := p.Synthesize(context.Background(), "This is a local SAPI test.", "", outputPath)
	if err != nil {
		t.Fatalf("SAPI synthesis failed: %v", err)
	}

	if format != "wav" {
		t.Errorf("Expected wav, got %s", format)
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("Output file was not created: %v", err)
	}
}

func TestOnline_EdgeTTS(t *testing.T) {
	if os.Getenv("TEST_TTS") == "" {
		t.Skip("Set TEST_TTS=1 to run Edge TTS integration test")
	}

	p := edgetts.NewProvider(tracker.New())
	outputPath := "test_edge.mp3"
	defer os.Remove(outputPath)

	format, err := p.Synthesize(context.Background(), "This is an Edge TTS online test.", "", outputPath)
	if err != nil {
		t.Fatalf("Edge TTS synthesis failed: %v", err)
	}

	if format != "mp3" {
		t.Errorf("Expected mp3, got %s", format)
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Errorf("Output file was not created: %v", err)
	}
}

func TestVoices_SAPI(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("SAPI only works on Windows")
	}
	if os.Getenv("TEST_TTS") == "" {
		t.Skip("Set TEST_TTS=1 to run SAPI integration test")
	}

	p := sapi.NewProvider(nil)
	voices, err := p.Voices(context.Background())
	if err != nil {
		t.Fatalf("Voices failed: %v", err)
	}

	if len(voices) == 0 {
		t.Log("No SAPI voices found (this can happen in some Windows environments, synthesis is verified separately)")
	}

	for _, v := range voices {
		t.Logf("Found voice: %s (%s)", v.Name, v.ID)
	}
}
