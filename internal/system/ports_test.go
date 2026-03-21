package system

import (
	"testing"

	"github.com/Gradient-Linux/concave-tui/internal/suite"
)

func TestCheckConflictsAndDeduplication(t *testing.T) {
	portRegistry = map[int]portRegistration{}

	neural := suite.Registry["neural"]
	conflicts, err := CheckConflicts(neural, []string{"flow"})
	if err != nil {
		t.Fatalf("CheckConflicts() error = %v", err)
	}
	if len(conflicts) != 1 || conflicts[0].Port != 8080 {
		t.Fatalf("unexpected conflicts %#v", conflicts)
	}

	boosting := suite.Registry["boosting"]
	conflicts, err = CheckConflicts(boosting, []string{"flow"})
	if err != nil {
		t.Fatalf("CheckConflicts() error = %v", err)
	}
	for _, conflict := range conflicts {
		if conflict.Port == 5000 {
			t.Fatalf("expected MLflow port 5000 to be deduplicated, got %#v", conflicts)
		}
	}

	if !IsMLflowDeduplicated([]string{"boosting"}) {
		t.Fatal("expected boosting to deduplicate MLflow")
	}
}

func TestRegisterAndDeregister(t *testing.T) {
	portRegistry = map[int]portRegistration{}

	boosting := suite.Registry["boosting"]
	if err := Register(boosting); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := Register(boosting); err != nil {
		t.Fatalf("Register() should be idempotent for same suite, got %v", err)
	}
	if err := Deregister(boosting); err != nil {
		t.Fatalf("Deregister() error = %v", err)
	}
	if len(portRegistry) != 0 {
		t.Fatalf("expected empty runtime registry, got %#v", portRegistry)
	}
}
