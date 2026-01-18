package api

import (
	"testing"
)

func TestFormatLogLine(t *testing.T) {
	input := `time=2026-01-18T06:50:46.074+01:00 level=INFO msg="DynamicConfigJob: Added dynamic interest" length="1613 " name=headframe qid=Q683835 category=industry longparam=thisiswaytooLongtobedisplayed`
	expected := "06:50:46 DynamicConfigJob: Added dynamic interest (category=industry, length=1613, name=headframe, qid=Q683835)"

	result := formatLogLine(input)
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}
