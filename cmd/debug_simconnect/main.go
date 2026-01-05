package main

import (
	"fmt"
	"log"
	"time"
	"unsafe"

	"phileasgo/pkg/sim/simconnect"
)

const (
	AppName = "PhileasGoDebug"
)

// We will test definitions one by one
type TestDef struct {
	Name string
	Unit string
	Type uint32
}

var Candidates = []TestDef{
	{"AMBIENT VISIBILITY", "Meters", simconnect.DATATYPE_FLOAT64},
	{"CLOUD COVERAGE DENSITY", "Percent", simconnect.DATATYPE_FLOAT64},
	{"AMBIENT IN CLOUD", "Bool", simconnect.DATATYPE_FLOAT64},
	{"Ambient In Cloud", "Bool", simconnect.DATATYPE_FLOAT64}, // Case sensitivity check
	{"AMBIENT IN CLOUD", "Enum", simconnect.DATATYPE_INT32},   // Type check
}

func main() {
	fmt.Println("Starting SimConnect Debugger...")

	// 1. Load DLL
	if err := simconnect.LoadDLL("lib/SimConnect.dll"); err != nil {
		log.Fatalf("Failed to load DLL: %v", err)
	}

	// 2. Open Connection
	h, err := simconnect.Open(AppName)
	if err != nil {
		log.Fatalf("Failed to open connection: %v", err)
	}
	defer simconnect.Close(h)
	fmt.Println("Connected to SimConnect!")

	// 3. Test Loop
	for i, c := range Candidates {
		fmt.Printf("[%d] Testing Variable: %s (%s)...\n", i, c.Name, c.Unit)

		// Define a unique ID for this test
		defID := uint32(i + 100)
		reqID := uint32(i + 100)

		// Add Definition
		err := simconnect.AddToDataDefinition(h, defID, c.Name, c.Unit, c.Type)
		if err != nil {
			fmt.Printf("  -> AddToDataDefinition FAILED locally: %v\n", err)
			continue
		}

		// Request Data (Once)
		err = simconnect.RequestDataOnSimObject(h, reqID, defID, simconnect.OBJECT_ID_USER, simconnect.PERIOD_ONCE, 0, 0, 0, 0)
		if err != nil {
			fmt.Printf("  -> RequestDataOnSimObject FAILED locally: %v\n", err)
			continue
		}

		// Wait for response or exception
		waitAndListen(h, reqID)
	}
}

func waitAndListen(h uintptr, expectedReqID uint32) {
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			fmt.Println("  -> Timeout waiting for response.")
			return
		case <-ticker.C:
			ppData, _, err := simconnect.GetNextDispatch(h)
			if err != nil {
				// No message is not an error here usually, but our wrapper might return err
				continue
			}
			if ppData == nil {
				continue
			}

			recv := (*simconnect.Recv)(ppData)
			switch recv.ID {
			case simconnect.RECV_ID_EXCEPTION:
				exc := (*simconnect.RecvException)(ppData)
				fmt.Printf("  -> EXCEPTION ID=%d SendID=%d Index=%d\n", exc.Exception, exc.SendID, exc.Index)
				return // Stop waiting on exception

			case simconnect.RECV_ID_SIMOBJECT_DATA:
				data := (*simconnect.RecvSimobjectData)(ppData)
				if data.RequestID == expectedReqID {
					fmt.Printf("  -> SUCCESS: Received Data (Size=%d)\n", data.Size)

					// Print the float value if it's float
					if data.DefineCount > 0 { // Just basic check
						valVal := *(*float64)(unsafe.Pointer(uintptr(ppData) + unsafe.Sizeof(*data)))
						fmt.Printf("     Value: %f\n", valVal)
					}
					return
				}
			}
		}
	}
}
