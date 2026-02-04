package announcement

import (
	"phileasgo/pkg/model"
	"testing"
)

func TestBase_TwoPass(t *testing.T) {
	b := NewBase("test", model.NarrativeTypePOI, false, nil, nil)

	if b.TwoPass() {
		t.Error("expected default TwoPass to be false")
	}

	b.SetTwoPass(true)
	if !b.TwoPass() {
		t.Error("expected TwoPass to be true after SetTwoPass(true)")
	}

	b.SetTwoPass(false)
	if b.TwoPass() {
		t.Error("expected TwoPass to be false after SetTwoPass(false)")
	}
}
