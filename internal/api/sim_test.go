package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"phileasgo/pkg/sim"
	"testing"
)

type mockSimClient struct {
	sim.Client
	lastCmd  string
	lastArgs map[string]any
	err      error
}

func (m *mockSimClient) ExecuteCommand(ctx context.Context, cmd string, args map[string]any) error {
	m.lastCmd = cmd
	m.lastArgs = args
	return m.err
}

func (m *mockSimClient) GetState() sim.State { return sim.StateActive }

func TestSimCommandHandler_HandleCommand(t *testing.T) {
	mockSim := &mockSimClient{}
	handler := NewSimCommandHandler(mockSim)

	t.Run("Valid command", func(t *testing.T) {
		reqBody, _ := json.Marshal(SimCommandRequest{
			Command: "land",
			Args:    map[string]any{"force": true},
		})
		req := httptest.NewRequest("POST", "/api/sim/command", bytes.NewBuffer(reqBody))
		rr := httptest.NewRecorder()

		handler.HandleCommand(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}
		if mockSim.lastCmd != "land" {
			t.Errorf("Expected command 'land', got '%s'", mockSim.lastCmd)
		}
		if mockSim.lastArgs["force"] != true {
			t.Error("Args not passed correctly")
		}
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/sim/command", bytes.NewBufferString("{invalid}"))
		rr := httptest.NewRecorder()

		handler.HandleCommand(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rr.Code)
		}
	})

	t.Run("Command Error", func(t *testing.T) {
		mockSim.err = errors.New("sim error")
		reqBody, _ := json.Marshal(SimCommandRequest{Command: "fail"})
		req := httptest.NewRequest("POST", "/api/sim/command", bytes.NewBuffer(reqBody))
		rr := httptest.NewRecorder()

		handler.HandleCommand(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rr.Code)
		}
	})
}
