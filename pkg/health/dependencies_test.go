package health

import (
	"context"
	"testing"
)

// mockDependencyChecker implements DependencyChecker for testing
type mockDependencyChecker struct {
	name   string
	status Status
	deps   []Dependency
}

func (m *mockDependencyChecker) Name() string {
	return m.name
}

func (m *mockDependencyChecker) Check(ctx context.Context) ComponentHealth {
	return ComponentHealth{
		Name:   m.name,
		Status: m.status,
	}
}

func (m *mockDependencyChecker) Dependencies() []Dependency {
	return m.deps
}

func TestDependencyRelationConstants(t *testing.T) {
	if DependencyRequired != "required" {
		t.Errorf("Unexpected DependencyRequired value: %s", DependencyRequired)
	}
	if DependencyOptional != "optional" {
		t.Errorf("Unexpected DependencyOptional value: %s", DependencyOptional)
	}
}

func TestNewDependencyAwareMonitor(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	if monitor == nil {
		t.Fatal("NewDependencyAwareMonitor returned nil")
	}
	if monitor.Monitor == nil {
		t.Error("Embedded Monitor is nil")
	}
	if monitor.dependencies == nil {
		t.Error("Dependencies map is nil")
	}
}

func TestDependencyAwareMonitorRegisterChecker(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	// Checker with dependencies
	checker := &mockDependencyChecker{
		name:   "service",
		status: StatusHealthy,
		deps: []Dependency{
			{Name: "database", Relation: DependencyRequired},
			{Name: "cache", Relation: DependencyOptional},
		},
	}
	monitor.RegisterChecker(checker)

	// Verify dependencies were registered
	deps := monitor.GetDependencies("service")
	if len(deps) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(deps))
	}
}

func TestDependencyAwareMonitorRegisterDependency(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	// Register a regular checker
	regularChecker := &mockChecker{name: "api", status: StatusHealthy}
	monitor.RegisterChecker(regularChecker)

	// Manually register dependencies
	monitor.RegisterDependency("api", Dependency{Name: "database", Relation: DependencyRequired})
	monitor.RegisterDependency("api", Dependency{Name: "cache", Relation: DependencyOptional})

	deps := monitor.GetDependencies("api")
	if len(deps) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(deps))
	}
}

func TestDependencyAwareMonitorCheckWithDependencies(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	// Register database
	dbChecker := &mockChecker{name: "database", status: StatusHealthy}
	monitor.RegisterChecker(dbChecker)

	// Register service with database dependency
	serviceChecker := &mockDependencyChecker{
		name:   "service",
		status: StatusHealthy,
		deps: []Dependency{
			{Name: "database", Relation: DependencyRequired},
		},
	}
	monitor.RegisterChecker(serviceChecker)

	ctx := context.Background()
	result := monitor.CheckWithDependencies(ctx)

	// Verify service has dependency info
	service, exists := result["service"]
	if !exists {
		t.Fatal("Service not in results")
	}
	if len(service.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(service.Dependencies))
	}
	if service.DependencyStatus["database"] != StatusHealthy {
		t.Errorf("Expected database to be healthy, got %s", service.DependencyStatus["database"])
	}
}

func TestDependencyAwareMonitorUnhealthyDependency(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	// Register unhealthy database
	dbChecker := &mockChecker{name: "database", status: StatusUnhealthy}
	monitor.RegisterChecker(dbChecker)

	// Register service with database dependency
	serviceChecker := &mockDependencyChecker{
		name:   "service",
		status: StatusHealthy,
		deps: []Dependency{
			{Name: "database", Relation: DependencyRequired},
		},
	}
	monitor.RegisterChecker(serviceChecker)

	ctx := context.Background()
	result := monitor.CheckWithDependencies(ctx)

	// Service should be unhealthy due to required dependency
	service := result["service"]
	if service.Status != StatusUnhealthy {
		t.Errorf("Expected service to be unhealthy, got %s", service.Status)
	}
	if service.DependencyStatus["database"] != StatusUnhealthy {
		t.Errorf("Expected database status to be unhealthy")
	}
}

func TestDependencyAwareMonitorDegradedDependency(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	// Register degraded database
	dbChecker := &mockChecker{name: "database", status: StatusDegraded}
	monitor.RegisterChecker(dbChecker)

	// Register service with database dependency
	serviceChecker := &mockDependencyChecker{
		name:   "service",
		status: StatusHealthy,
		deps: []Dependency{
			{Name: "database", Relation: DependencyRequired},
		},
	}
	monitor.RegisterChecker(serviceChecker)

	ctx := context.Background()
	result := monitor.CheckWithDependencies(ctx)

	// Service should be degraded due to required dependency being degraded
	service := result["service"]
	if service.Status != StatusDegraded {
		t.Errorf("Expected service to be degraded, got %s", service.Status)
	}
}

func TestDependencyAwareMonitorOptionalDependency(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	// Register unhealthy cache (optional)
	cacheChecker := &mockChecker{name: "cache", status: StatusUnhealthy}
	monitor.RegisterChecker(cacheChecker)

	// Register service with optional cache dependency
	serviceChecker := &mockDependencyChecker{
		name:   "service",
		status: StatusHealthy,
		deps: []Dependency{
			{Name: "cache", Relation: DependencyOptional},
		},
	}
	monitor.RegisterChecker(serviceChecker)

	ctx := context.Background()
	result := monitor.CheckWithDependencies(ctx)

	// Service should still be healthy (optional dependency doesn't affect status)
	service := result["service"]
	if service.Status != StatusHealthy {
		t.Errorf("Expected service to remain healthy, got %s", service.Status)
	}
}

func TestDependencyAwareMonitorMissingDependency(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	// Register service with dependency that doesn't exist
	serviceChecker := &mockDependencyChecker{
		name:   "service",
		status: StatusHealthy,
		deps: []Dependency{
			{Name: "nonexistent", Relation: DependencyRequired},
		},
	}
	monitor.RegisterChecker(serviceChecker)

	ctx := context.Background()
	result := monitor.CheckWithDependencies(ctx)

	// Service should be unhealthy due to missing required dependency
	service := result["service"]
	if service.Status != StatusUnhealthy {
		t.Errorf("Expected service to be unhealthy, got %s", service.Status)
	}
	if service.DependencyStatus["nonexistent"] != StatusUnhealthy {
		t.Errorf("Expected missing dependency to be unhealthy")
	}
}

func TestDependencyAwareMonitorGetHealthSummary(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	// Register components
	healthyChecker := &mockChecker{name: "healthy", status: StatusHealthy}
	degradedChecker := &mockChecker{name: "degraded", status: StatusDegraded}
	unhealthyChecker := &mockChecker{name: "unhealthy", status: StatusUnhealthy}

	monitor.RegisterChecker(healthyChecker)
	monitor.RegisterChecker(degradedChecker)
	monitor.RegisterChecker(unhealthyChecker)

	ctx := context.Background()
	summary := monitor.GetHealthSummary(ctx)

	if summary.TotalComponents != 3 {
		t.Errorf("Expected 3 components, got %d", summary.TotalComponents)
	}

	if summary.HealthyCounts[StatusHealthy] != 1 {
		t.Errorf("Expected 1 healthy component, got %d", summary.HealthyCounts[StatusHealthy])
	}
	if summary.HealthyCounts[StatusDegraded] != 1 {
		t.Errorf("Expected 1 degraded component, got %d", summary.HealthyCounts[StatusDegraded])
	}
	if summary.HealthyCounts[StatusUnhealthy] != 1 {
		t.Errorf("Expected 1 unhealthy component, got %d", summary.HealthyCounts[StatusUnhealthy])
	}

	if len(summary.ComponentsWithIssues) != 2 {
		t.Errorf("Expected 2 components with issues, got %d", len(summary.ComponentsWithIssues))
	}
}

func TestDependencyAwareMonitorGetHealthSummaryWithDependencyIssues(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	// Register unhealthy database
	dbChecker := &mockChecker{name: "database", status: StatusUnhealthy}
	monitor.RegisterChecker(dbChecker)

	// Register service depending on database
	serviceChecker := &mockDependencyChecker{
		name:   "service",
		status: StatusHealthy,
		deps: []Dependency{
			{Name: "database", Relation: DependencyRequired},
		},
	}
	monitor.RegisterChecker(serviceChecker)

	ctx := context.Background()
	summary := monitor.GetHealthSummary(ctx)

	if len(summary.DependencyIssues) == 0 {
		t.Error("Expected dependency issues to be reported")
	}
}

func TestComponentWithDeps(t *testing.T) {
	comp := ComponentWithDeps{
		ComponentHealth: ComponentHealth{
			Name:   "test",
			Status: StatusHealthy,
		},
		Dependencies: []Dependency{
			{Name: "dep1", Relation: DependencyRequired},
		},
		DependencyStatus: map[string]Status{
			"dep1": StatusHealthy,
		},
		DependencyMessages: map[string]string{
			"dep1": "OK",
		},
	}

	if comp.Name != "test" {
		t.Errorf("Unexpected name: %s", comp.Name)
	}
	if len(comp.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(comp.Dependencies))
	}
	if comp.DependencyStatus["dep1"] != StatusHealthy {
		t.Errorf("Unexpected dependency status: %s", comp.DependencyStatus["dep1"])
	}
}

func TestGetDependenciesEmpty(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	deps := monitor.GetDependencies("nonexistent")
	if len(deps) != 0 {
		t.Errorf("Expected empty dependencies for non-existent component, got %d", len(deps))
	}
}

func TestMultipleDependencies(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	// Register all dependencies
	dbChecker := &mockChecker{name: "database", status: StatusHealthy}
	cacheChecker := &mockChecker{name: "cache", status: StatusHealthy}
	queueChecker := &mockChecker{name: "queue", status: StatusHealthy}
	monitor.RegisterChecker(dbChecker)
	monitor.RegisterChecker(cacheChecker)
	monitor.RegisterChecker(queueChecker)

	// Register service with multiple dependencies
	serviceChecker := &mockDependencyChecker{
		name:   "service",
		status: StatusHealthy,
		deps: []Dependency{
			{Name: "database", Relation: DependencyRequired},
			{Name: "cache", Relation: DependencyOptional},
			{Name: "queue", Relation: DependencyRequired},
		},
	}
	monitor.RegisterChecker(serviceChecker)

	ctx := context.Background()
	result := monitor.CheckWithDependencies(ctx)

	service := result["service"]
	if len(service.Dependencies) != 3 {
		t.Errorf("Expected 3 dependencies, got %d", len(service.Dependencies))
	}
	if service.Status != StatusHealthy {
		t.Errorf("Expected healthy service, got %s", service.Status)
	}
}

func TestMixedDependencyStatus(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	// Register with mixed status
	dbChecker := &mockChecker{name: "database", status: StatusHealthy}   // OK
	cacheChecker := &mockChecker{name: "cache", status: StatusUnhealthy} // Fail (optional)
	queueChecker := &mockChecker{name: "queue", status: StatusDegraded}  // Degraded (required)
	monitor.RegisterChecker(dbChecker)
	monitor.RegisterChecker(cacheChecker)
	monitor.RegisterChecker(queueChecker)

	// Register service
	serviceChecker := &mockDependencyChecker{
		name:   "service",
		status: StatusHealthy,
		deps: []Dependency{
			{Name: "database", Relation: DependencyRequired},
			{Name: "cache", Relation: DependencyOptional},
			{Name: "queue", Relation: DependencyRequired},
		},
	}
	monitor.RegisterChecker(serviceChecker)

	ctx := context.Background()
	result := monitor.CheckWithDependencies(ctx)

	service := result["service"]
	// Should be degraded because queue (required) is degraded
	if service.Status != StatusDegraded {
		t.Errorf("Expected degraded service, got %s", service.Status)
	}
}

func TestHasCircularDependency(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	// Register A depends on B
	monitor.RegisterDependency("A", Dependency{Name: "B", Relation: DependencyRequired})

	// Check if B depends on A would create a cycle
	if !monitor.HasCircularDependency("B", Dependency{Name: "A", Relation: DependencyRequired}) {
		t.Error("Expected circular dependency to be detected (B -> A when A -> B exists)")
	}

	// Check that C depends on A does not create a cycle
	if monitor.HasCircularDependency("C", Dependency{Name: "A", Relation: DependencyRequired}) {
		t.Error("Should not detect circular dependency for C -> A")
	}
}

func TestHasCircularDependencyChain(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	// Create a chain: A -> B -> C
	monitor.RegisterDependency("A", Dependency{Name: "B", Relation: DependencyRequired})
	monitor.RegisterDependency("B", Dependency{Name: "C", Relation: DependencyRequired})

	// Check if C depends on A would create a cycle (A -> B -> C -> A)
	if !monitor.HasCircularDependency("C", Dependency{Name: "A", Relation: DependencyRequired}) {
		t.Error("Expected circular dependency to be detected (C -> A when A -> B -> C exists)")
	}

	// D depends on C should not create a cycle
	if monitor.HasCircularDependency("D", Dependency{Name: "C", Relation: DependencyRequired}) {
		t.Error("Should not detect circular dependency for D -> C")
	}
}

func TestHasCircularDependencyNoExistingDeps(t *testing.T) {
	monitor := NewDependencyAwareMonitor()

	// No existing dependencies - should never detect a cycle
	if monitor.HasCircularDependency("A", Dependency{Name: "B", Relation: DependencyRequired}) {
		t.Error("Should not detect circular dependency when no dependencies exist")
	}
}
