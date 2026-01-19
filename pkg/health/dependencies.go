// Package health provides health check and monitoring capabilities for the Tor client.
package health

import (
	"context"
	"sync"
)

// DependencyRelation describes the relationship between components.
type DependencyRelation string

const (
	// DependencyRequired means the parent cannot function without the dependency.
	DependencyRequired DependencyRelation = "required"

	// DependencyOptional means the parent can function without the dependency,
	// but may have reduced functionality.
	DependencyOptional DependencyRelation = "optional"
)

// Dependency represents a dependency relationship between components.
type Dependency struct {
	// Name is the name of the dependency component.
	Name string

	// Relation defines how critical the dependency is.
	Relation DependencyRelation
}

// ComponentWithDeps extends ComponentHealth with dependency information.
type ComponentWithDeps struct {
	ComponentHealth

	// Dependencies lists this component's dependencies.
	Dependencies []Dependency `json:"dependencies,omitempty"`

	// DependencyStatus shows the health of each dependency.
	DependencyStatus map[string]Status `json:"dependency_status,omitempty"`

	// DependencyMessages provides details about dependency health.
	DependencyMessages map[string]string `json:"dependency_messages,omitempty"`
}

// DependencyChecker extends Checker to include dependency declarations.
type DependencyChecker interface {
	Checker
	// Dependencies returns the list of components this checker depends on.
	Dependencies() []Dependency
}

// DependencyAwareMonitor extends Monitor with dependency-aware health checking.
type DependencyAwareMonitor struct {
	*Monitor
	mu           sync.RWMutex
	dependencies map[string][]Dependency // component -> its dependencies
}

// NewDependencyAwareMonitor creates a new dependency-aware health monitor.
func NewDependencyAwareMonitor() *DependencyAwareMonitor {
	return &DependencyAwareMonitor{
		Monitor:      NewMonitor(),
		dependencies: make(map[string][]Dependency),
	}
}

// RegisterChecker registers a health checker and extracts its dependencies.
func (m *DependencyAwareMonitor) RegisterChecker(checker Checker) {
	m.Monitor.RegisterChecker(checker)

	// If the checker declares dependencies, record them
	if dc, ok := checker.(DependencyChecker); ok {
		m.mu.Lock()
		m.dependencies[checker.Name()] = dc.Dependencies()
		m.mu.Unlock()
	}
}

// RegisterDependency manually registers a dependency relationship.
func (m *DependencyAwareMonitor) RegisterDependency(component string, dep Dependency) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dependencies[component] = append(m.dependencies[component], dep)
}

// GetDependencies returns the dependencies for a component.
func (m *DependencyAwareMonitor) GetDependencies(component string) []Dependency {
	m.mu.RLock()
	defer m.mu.RUnlock()
	deps := m.dependencies[component]
	result := make([]Dependency, len(deps))
	copy(result, deps)
	return result
}

// CheckWithDependencies performs health checks and includes dependency information.
func (m *DependencyAwareMonitor) CheckWithDependencies(ctx context.Context) map[string]ComponentWithDeps {
	// First, get regular health check results
	health := m.Monitor.Check(ctx)

	// Build extended results with dependency information
	result := make(map[string]ComponentWithDeps)

	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, compHealth := range health.Components {
		extended := ComponentWithDeps{
			ComponentHealth: compHealth,
		}

		// Add dependency information if available
		if deps, ok := m.dependencies[name]; ok && len(deps) > 0 {
			extended.Dependencies = deps
			extended.DependencyStatus = make(map[string]Status)
			extended.DependencyMessages = make(map[string]string)

			for _, dep := range deps {
				if depHealth, exists := health.Components[dep.Name]; exists {
					extended.DependencyStatus[dep.Name] = depHealth.Status
					extended.DependencyMessages[dep.Name] = depHealth.Message
				} else {
					// Dependency component not found
					extended.DependencyStatus[dep.Name] = StatusUnhealthy
					extended.DependencyMessages[dep.Name] = "Dependency component not registered"
				}
			}

			// Adjust component status based on dependency health
			extended.ComponentHealth = m.adjustStatusForDependencies(extended.ComponentHealth, deps, health.Components)
		}

		result[name] = extended
	}

	return result
}

// adjustStatusForDependencies modifies a component's status based on its dependencies.
func (m *DependencyAwareMonitor) adjustStatusForDependencies(
	comp ComponentHealth,
	deps []Dependency,
	allComponents map[string]ComponentHealth,
) ComponentHealth {
	for _, dep := range deps {
		depHealth, exists := allComponents[dep.Name]
		if !exists {
			// Missing required dependency
			if dep.Relation == DependencyRequired {
				comp.Status = StatusUnhealthy
				comp.Message = "Required dependency '" + dep.Name + "' not available"
				return comp
			}
			continue
		}

		if dep.Relation == DependencyRequired && depHealth.Status == StatusUnhealthy {
			comp.Status = StatusUnhealthy
			comp.Message = "Required dependency '" + dep.Name + "' is unhealthy"
			return comp
		}

		// Degrade if required dependency is degraded
		if dep.Relation == DependencyRequired && depHealth.Status == StatusDegraded {
			if comp.Status == StatusHealthy {
				comp.Status = StatusDegraded
				comp.Message = "Required dependency '" + dep.Name + "' is degraded"
			}
		}
	}

	return comp
}

// HealthSummary provides a high-level summary of system health including dependencies.
type HealthSummary struct {
	OverallHealth

	// TotalComponents is the total number of registered components.
	TotalComponents int `json:"total_components"`

	// HealthyCounts tracks count of components by status.
	HealthyCounts map[Status]int `json:"status_counts"`

	// ComponentsWithIssues lists components that are not healthy.
	ComponentsWithIssues []string `json:"components_with_issues,omitempty"`

	// DependencyIssues lists dependency-related health issues.
	DependencyIssues []string `json:"dependency_issues,omitempty"`
}

// GetHealthSummary returns a summarized view of system health.
func (m *DependencyAwareMonitor) GetHealthSummary(ctx context.Context) HealthSummary {
	health := m.Monitor.Check(ctx)
	withDeps := m.CheckWithDependencies(ctx)

	summary := HealthSummary{
		OverallHealth:   health,
		TotalComponents: len(health.Components),
		HealthyCounts:   make(map[Status]int),
	}

	for name, comp := range health.Components {
		summary.HealthyCounts[comp.Status]++

		if comp.Status != StatusHealthy {
			summary.ComponentsWithIssues = append(summary.ComponentsWithIssues, name)
		}
	}

	// Check for dependency issues
	for name, comp := range withDeps {
		if len(comp.Dependencies) > 0 {
			for _, dep := range comp.Dependencies {
				if status, ok := comp.DependencyStatus[dep.Name]; ok {
					if status != StatusHealthy && dep.Relation == DependencyRequired {
						issue := name + " depends on " + dep.Name + " (" + string(status) + ")"
						summary.DependencyIssues = append(summary.DependencyIssues, issue)
					}
				}
			}
		}
	}

	return summary
}
