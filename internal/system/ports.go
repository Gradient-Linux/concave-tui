package system

import (
	"fmt"
	"sync"

	"github.com/Gradient-Linux/concave-tui/internal/suite"
)

// PortConflict describes a detected suite port collision.
type PortConflict struct {
	Port          int
	ExistingSuite string
	NewSuite      string
	Service       string
}

type portRegistration struct {
	Suite   string
	Service string
}

var (
	portRegistryMu sync.Mutex
	portRegistry   = map[int]portRegistration{}
)

// CheckConflicts returns port conflicts between a suite and installed/runtime suites.
func CheckConflicts(newSuite suite.Suite, installedSuites []string) ([]PortConflict, error) {
	portRegistryMu.Lock()
	defer portRegistryMu.Unlock()

	registry, err := buildPortRegistry(installedSuites)
	if err != nil {
		return nil, err
	}
	for port, registration := range portRegistry {
		registry[port] = registration
	}
	return checkConflicts(newSuite, registry), nil
}

// IsMLflowDeduplicated reports whether MLflow is already satisfied by an installed suite.
func IsMLflowDeduplicated(installedSuites []string) bool {
	for _, name := range installedSuites {
		if name == "boosting" || name == "flow" {
			return true
		}
	}
	return false
}

// Register adds suite ports to the runtime registry for started suites.
func Register(s suite.Suite) error {
	portRegistryMu.Lock()
	defer portRegistryMu.Unlock()

	for _, mapping := range s.Ports {
		if existing, ok := portRegistry[mapping.Port]; ok {
			if existing.Suite == s.Name || sharedMLflow(existing.Suite, s.Name, mapping.Port) {
				continue
			}
			return fmt.Errorf("port %d already used by %s (%s)", mapping.Port, existing.Suite, existing.Service)
		}
		portRegistry[mapping.Port] = portRegistration{
			Suite:   s.Name,
			Service: mapping.Service,
		}
	}
	return nil
}

// Deregister removes a suite from the runtime registry for stopped suites.
func Deregister(s suite.Suite) error {
	portRegistryMu.Lock()
	defer portRegistryMu.Unlock()

	for _, mapping := range s.Ports {
		if existing, ok := portRegistry[mapping.Port]; ok && existing.Suite == s.Name {
			delete(portRegistry, mapping.Port)
		}
	}
	return nil
}

func buildPortRegistry(installedSuites []string) (map[int]portRegistration, error) {
	registry := make(map[int]portRegistration)
	for _, name := range installedSuites {
		s, err := suite.Get(name)
		if err != nil {
			return nil, err
		}
		for _, mapping := range s.Ports {
			if existing, ok := registry[mapping.Port]; ok && sharedMLflow(existing.Suite, s.Name, mapping.Port) {
				continue
			}
			registry[mapping.Port] = portRegistration{
				Suite:   s.Name,
				Service: mapping.Service,
			}
		}
	}
	return registry, nil
}

func checkConflicts(newSuite suite.Suite, registry map[int]portRegistration) []PortConflict {
	conflicts := make([]PortConflict, 0)
	for _, mapping := range newSuite.Ports {
		existing, ok := registry[mapping.Port]
		if !ok || sharedMLflow(existing.Suite, newSuite.Name, mapping.Port) {
			continue
		}
		conflicts = append(conflicts, PortConflict{
			Port:          mapping.Port,
			ExistingSuite: existing.Suite,
			NewSuite:      newSuite.Name,
			Service:       existing.Service,
		})
	}
	return conflicts
}

func sharedMLflow(existingSuite, newSuite string, port int) bool {
	if port != 5000 {
		return false
	}
	return (existingSuite == "boosting" && newSuite == "flow") ||
		(existingSuite == "flow" && newSuite == "boosting")
}
