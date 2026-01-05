package main

import (
	"context"
	"log/slog"

	"phileasgo/pkg/config"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/sim/mocksim"
	"phileasgo/pkg/sim/simconnect"
)

func initializeSimClient(ctx context.Context, cfg *config.Config) (sim.Client, error) {
	simSource := cfg.Sim.Provider

	// Default to SimConnect unless explicitly set to "mock"
	if simSource == "mock" {
		slog.Info("Sim Source: Mock")
		return mocksim.NewClient(mocksim.DefaultConfig()), nil
	}

	// Default or Explicit SimConnect
	slog.Info("Sim Source: SimConnect (Default)")
	// SimConnect Client - DLL is in lib/ folder, copied to bin/ by Makefile
	sc, err := simconnect.NewClient("PhileasGo", "")
	if err != nil {
		slog.Error("Failed to create SimConnect client, falling back to Mock", "error", err)
		return mocksim.NewClient(mocksim.DefaultConfig()), nil
	}
	return sc, nil
}
