package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

func main() {
	fmt.Println("=== SimConnect.dll Detection Test ===")
	fmt.Println()

	// Check environment variable
	fmt.Println("1. Checking MSFS_SDK environment variable...")
	if sdkPath := os.Getenv("MSFS_SDK"); sdkPath != "" {
		fmt.Printf("   MSFS_SDK = %s\n", sdkPath)
		dllPath := filepath.Join(sdkPath, "SimConnect SDK", "lib", "SimConnect.dll")
		checkPath(dllPath)
	} else {
		fmt.Println("   MSFS_SDK not set")
	}
	fmt.Println()

	// Check common SDK paths
	fmt.Println("2. Checking common SDK paths...")
	sdkPaths := []string{
		`C:\MSFS 2024 SDK\SimConnect SDK\lib\SimConnect.dll`,
		`C:\MSFS SDK\SimConnect SDK\lib\SimConnect.dll`,
		`C:\Program Files (x86)\Microsoft Flight Simulator SDK\SimConnect SDK\lib\SimConnect.dll`,
	}
	for _, p := range sdkPaths {
		checkPath(p)
	}
	fmt.Println()

	// Check Steam registry
	fmt.Println("3. Checking Steam registry...")
	steamPaths := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\Steam App 1250410`,
		`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\Steam App 1250410`,
	}
	for _, regPath := range steamPaths {
		fmt.Printf("   Trying: HKLM\\%s\n", regPath)
		if key, err := registry.OpenKey(registry.LOCAL_MACHINE, regPath, registry.QUERY_VALUE); err == nil {
			if val, _, err := key.GetStringValue("InstallLocation"); err == nil {
				fmt.Printf("   -> InstallLocation = %s\n", val)
				checkPath(filepath.Join(val, "SimConnect.dll"))
			} else {
				fmt.Printf("   -> InstallLocation not found: %v\n", err)
			}
			key.Close()
		} else {
			fmt.Printf("   -> Key not found: %v\n", err)
		}
	}
	fmt.Println()

	// Check Microsoft Store paths
	fmt.Println("4. Checking Microsoft Store paths...")
	localAppData := os.Getenv("LOCALAPPDATA")
	fmt.Printf("   LOCALAPPDATA = %s\n", localAppData)

	// Store package locations
	storePaths := []string{
		filepath.Join(localAppData, "Packages", "Microsoft.FlightSimulator_8wekyb3d8bbwe"),
		filepath.Join(localAppData, "Packages", "Microsoft.FlightDashboard_8wekyb3d8bbwe"),
	}
	for _, p := range storePaths {
		if _, err := os.Stat(p); err == nil {
			fmt.Printf("   Found Store package: %s\n", p)
		}
	}
	fmt.Println()

	// Check WinSxS (Windows Side-by-Side) for SimConnect
	fmt.Println("5. Checking Windows system paths...")
	windowsDir := os.Getenv("WINDIR")
	winsxsGlob := filepath.Join(windowsDir, "WinSxS", "*simconnect*")
	matches, _ := filepath.Glob(winsxsGlob)
	if len(matches) > 0 {
		fmt.Println("   Found WinSxS SimConnect folders:")
		for _, m := range matches {
			fmt.Printf("     %s\n", m)
			// Look for DLL inside
			dllMatches, _ := filepath.Glob(filepath.Join(m, "SimConnect.dll"))
			for _, dll := range dllMatches {
				checkPath(dll)
			}
		}
	} else {
		fmt.Println("   No WinSxS SimConnect folders found")
	}
	fmt.Println()

	// Check common game install locations
	fmt.Println("6. Checking common MSFS install locations...")
	gameLocations := []string{
		`C:\Program Files\WindowsApps`,
		`D:\SteamLibrary\steamapps\common\MicrosoftFlightSimulator`,
		`C:\Program Files (x86)\Steam\steamapps\common\MicrosoftFlightSimulator`,
		`E:\SteamLibrary\steamapps\common\MicrosoftFlightSimulator`,
	}
	for _, loc := range gameLocations {
		if _, err := os.Stat(loc); err == nil {
			fmt.Printf("   Found: %s\n", loc)
			checkPath(filepath.Join(loc, "SimConnect.dll"))
		}
	}
	fmt.Println()

	// Check if SimConnect is registered globally
	fmt.Println("7. Checking for SimConnect in PATH or System32...")
	checkPath(filepath.Join(windowsDir, "System32", "SimConnect.dll"))
	checkPath(filepath.Join(windowsDir, "SysWOW64", "SimConnect.dll"))

	fmt.Println()
	fmt.Println("=== Test Complete ===")
}

func checkPath(path string) {
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("   ✓ FOUND: %s\n", path)
	} else {
		fmt.Printf("   ✗ NOT FOUND: %s\n", path)
	}
}
