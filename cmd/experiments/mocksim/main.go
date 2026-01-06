package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"phileasgo/pkg/sim/mocksim"
)

func main() {
	cfg := mocksim.Config{
		DurationParked: 5 * time.Second,
		DurationTaxi:   5 * time.Second,
		DurationHold:   5 * time.Second,
		StartLat:       51.6845,
		StartLon:       14.4234,
		StartAlt:       285.0,
		StartHeading:   180.0,
	}
	client := mocksim.NewClient(cfg)
	defer client.Close()

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	fmt.Println("Mock Simulator Started. Press Ctrl+C to exit.")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt, shutting down...")
		cancel()
	}()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tel, err := client.GetTelemetry(ctx)
			if err != nil {
				log.Printf("Error getting telemetry: %v", err)
				continue
			}
			fmt.Printf("[%s] State: Ground=%v | Pos: %.4f, %.4f | Alt: %.0f ft | Spd: %.0fkts | Hdg: %.0f\n",
				time.Now().Format("15:04:05"),
				tel.IsOnGround,
				tel.Latitude, tel.Longitude,
				tel.AltitudeMSL,
				tel.GroundSpeed,
				tel.Heading,
			)
		}
	}
}
