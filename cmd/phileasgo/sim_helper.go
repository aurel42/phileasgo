package main

import (
	"context"
	"log/slog"
	"time"

	"phileasgo/pkg/config"
	"phileasgo/pkg/sim"
	"phileasgo/pkg/sim/mocksim"
	"phileasgo/pkg/sim/simconnect"
)

func initializeSimClient(ctx context.Context, cfgProv config.Provider) (sim.Client, error) {
	simSource := cfgProv.SimProvider(ctx)

	// Default to SimConnect unless explicitly set to "mock"
	if simSource == "mock" {
		slog.Info("Sim Source: Mock")
		return mocksim.NewClient(mocksim.Config{
			DurationParked: cfgProv.MockDurationParked(ctx),
			DurationTaxi:   cfgProv.MockDurationTaxi(ctx),
			DurationHold:   cfgProv.MockDurationHold(ctx),
			StartLat:       cfgProv.MockStartLat(ctx),
			StartLon:       cfgProv.MockStartLon(ctx),
			StartAlt:       cfgProv.MockStartAlt(ctx),
			StartHeading:   cfgProv.MockStartHeading(ctx),
		}), nil
	}

	// Default or Explicit SimConnect
	slog.Info("Sim Source: SimConnect (Default)")
	appCfg := cfgProv.AppConfig()
	sc, err := simconnect.NewClient("PhileasGo", "", appCfg.Sim.ProcessName, time.Duration(appCfg.Sim.ReconnectInterval))
	if err != nil {
		slog.Error("Failed to create SimConnect client, falling back to Mock", "error", err)
		return mocksim.NewClient(mocksim.Config{
			DurationParked: cfgProv.MockDurationParked(ctx),
			DurationTaxi:   cfgProv.MockDurationTaxi(ctx),
			DurationHold:   cfgProv.MockDurationHold(ctx),
			StartLat:       cfgProv.MockStartLat(ctx),
			StartLon:       cfgProv.MockStartLon(ctx),
			StartAlt:       cfgProv.MockStartAlt(ctx),
			StartHeading:   cfgProv.MockStartHeading(ctx),
		}), nil
	}
	return sc, nil
}
