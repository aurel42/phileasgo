package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration wraps time.Duration to support extended units (d, w) in YAML/JSON.
type Duration time.Duration

// Common durations.
const (
	Day  = 24 * time.Hour
	Week = 7 * Day
)

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	dur, err := ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (d Duration) MarshalYAML() (interface{}, error) {
	return time.Duration(d).String(), nil
}

// ParseDuration parses a duration string, supporting d and w.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	// fast path for standard units or simple numbers (assume ns if number?)
	// Standard time.ParseDuration doesn't like 'd' or 'w'.

	// Check for 'd' or 'w'
	if strings.ContainsAny(s, "dw") {
		// Simple recursive logic or regex replace?
		// Regex to find (value)(unit) pairs is safer.
		// But time.ParseDuration allows composite "2h45m".
		// Implementing a full parser is complex.
		// Let's support simple single units or standard composite if no d/w.

		// If it contains d/w, let's try to handle it.
		// A simple approach: convert "1d" -> "24h", "1w" -> "168h".
		// But "1d2h" -> "24h2h"? Valid.

		// Regex replacement approach
		// Warning: this is simple and might break if they write "100msd" (unlikely)

		// Let's manually parse common cases or just simple regex replace.
		// converting 1d to 24h, But we need to calculate the value.
		// "2d" -> 48h.

		return parseExtendedDuration(s)
	}

	return time.ParseDuration(s)
}

var unitMap = map[string]time.Duration{
	"ns": time.Nanosecond,
	"us": time.Microsecond,
	"µs": time.Microsecond,
	"ms": time.Millisecond,
	"s":  time.Second,
	"m":  time.Minute,
	"h":  time.Hour,
	"d":  Day,
	"w":  Week,
}

func parseExtendedDuration(s string) (time.Duration, error) {
	// Simple scanner
	var total time.Duration

	// Regexp to match number + unit
	// valid number: ints or floats
	re := regexp.MustCompile(`([0-9.]+)([a-zµ]+)`)
	matches := re.FindAllStringSubmatch(s, -1)

	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid duration format: %s", s)
	}

	for _, match := range matches {
		valStr := match[1]
		unitStr := match[2]

		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number in duration: %s", valStr)
		}

		base, ok := unitMap[unitStr]
		if !ok {
			return 0, fmt.Errorf("unknown unit: %s", unitStr)
		}

		total += time.Duration(val * float64(base))
	}

	return total, nil
}

// Distance represents a distance in meters.
type Distance float64

// UnmarshalYAML implements yaml.Unmarshaler.
func (d *Distance) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		// Try decoding as number (if user just wrote 1000)
		var f float64
		if errNum := value.Decode(&f); errNum == nil {
			*d = Distance(f)
			return nil
		}
		return err
	}

	dist, err := ParseDistance(s)
	if err != nil {
		return err
	}
	*d = Distance(dist)
	return nil
}

// MarshalYAML implements yaml.Marshaler.
func (d Distance) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf("%.2fm", float64(d)), nil
}

func ParseDistance(s string) (float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	// Check unit
	var mult float64
	var numStr string

	switch {
	case strings.HasSuffix(s, "km"):
		mult = 1000
		numStr = strings.TrimSuffix(s, "km")
	case strings.HasSuffix(s, "nm"):
		mult = 1852
		numStr = strings.TrimSuffix(s, "nm")
	case strings.HasSuffix(s, "m"):
		mult = 1
		numStr = strings.TrimSuffix(s, "m")
	case strings.HasSuffix(s, "ft"):
		mult = 0.3048
		numStr = strings.TrimSuffix(s, "ft")
	default:
		// No unit? assume meters? or error?
		// "we need to accept values in m, km, nm" -> implies unit requirement or default.
		// Let's allow unitless as meters for backward compat if any.
		mult = 1
		numStr = s
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(numStr), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid distance number: %w", err)
	}

	return val * mult, nil
}
