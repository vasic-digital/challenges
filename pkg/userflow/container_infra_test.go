package userflow

import (
	"testing"

	containers_health "digital.vasic.containers/pkg/health"
	containers_logging "digital.vasic.containers/pkg/logging"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTestEnvironment_Defaults(t *testing.T) {
	te, err := NewTestEnvironment()
	if err != nil {
		t.Skipf(
			"skipping: no container runtime available: %v",
			err,
		)
	}

	require.NotNil(t, te)
	assert.NotNil(t, te.runtime, "runtime should be set")
	assert.NotNil(t, te.compose, "compose should be set")
	assert.NotNil(t, te.health, "health should be set")
	assert.NotNil(t, te.registry, "registry should be set")
	assert.NotNil(t, te.eventBus, "eventBus should be set")
	assert.Equal(t,
		"userflow-test", te.projectName,
		"default project name",
	)
	assert.Equal(t,
		"docker-compose.test.yml", te.composeFile,
		"default compose file",
	)
}

func TestNewTestEnvironment_WithOptions(t *testing.T) {
	logger := containers_logging.NewStdLogger("test")
	groups := []PlatformGroup{
		{
			Name:     "api",
			Services: []string{"catalog-api", "postgres"},
			CPULimit: 2.0,
			MemoryMB: 4096,
		},
	}

	te, err := NewTestEnvironment(
		WithComposeFile("custom-compose.yml"),
		WithProjectName("custom-project"),
		WithPlatformGroups(groups),
		WithLogger(logger),
	)
	if err != nil {
		t.Skipf(
			"skipping: no container runtime available: %v",
			err,
		)
	}

	require.NotNil(t, te)
	assert.Equal(t,
		"custom-compose.yml", te.composeFile,
		"compose file should be overridden",
	)
	assert.Equal(t,
		"custom-project", te.projectName,
		"project name should be overridden",
	)
	assert.Len(t, te.groups, 1,
		"should have one platform group",
	)
	assert.Equal(t,
		"api", te.groups[0].Name,
		"group name should match",
	)
}

func TestPlatformGroup_Fields(t *testing.T) {
	tests := []struct {
		name     string
		group    PlatformGroup
		wantSvcs int
		wantCPU  float64
		wantMem  int
	}{
		{
			name: "api group",
			group: PlatformGroup{
				Name:     "api",
				Services: []string{"catalog-api", "postgres"},
				CPULimit: 2.0,
				MemoryMB: 4096,
			},
			wantSvcs: 2,
			wantCPU:  2.0,
			wantMem:  4096,
		},
		{
			name: "web group",
			group: PlatformGroup{
				Name:        "web",
				Services:    []string{"catalog-web"},
				CPULimit:    1.0,
				MemoryMB:    2048,
				ComposeFile: "web-compose.yml",
			},
			wantSvcs: 1,
			wantCPU:  1.0,
			wantMem:  2048,
		},
		{
			name: "empty group",
			group: PlatformGroup{
				Name: "empty",
			},
			wantSvcs: 0,
			wantCPU:  0.0,
			wantMem:  0,
		},
		{
			name: "group with health targets",
			group: PlatformGroup{
				Name:     "monitored",
				Services: []string{"svc-a"},
				CPULimit: 0.5,
				MemoryMB: 512,
				HealthTargets: []containers_health.HealthTarget{
					{
						Name: "svc-a",
						Host: "localhost",
						Port: "8080",
						Type: containers_health.HealthHTTP,
						Path: "/health",
					},
				},
			},
			wantSvcs: 1,
			wantCPU:  0.5,
			wantMem:  512,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t,
				tt.wantSvcs, len(tt.group.Services),
				"service count",
			)
			assert.Equal(t,
				tt.wantCPU, tt.group.CPULimit,
				"CPU limit",
			)
			assert.Equal(t,
				tt.wantMem, tt.group.MemoryMB,
				"memory limit",
			)
		})
	}
}

func TestTestEnvironment_Accessors(t *testing.T) {
	te, err := NewTestEnvironment()
	if err != nil {
		t.Skipf(
			"skipping: no container runtime available: %v",
			err,
		)
	}

	require.NotNil(t, te)

	rt := te.Runtime()
	assert.NotNil(t, rt, "Runtime() should return non-nil")
	assert.NotEmpty(t, rt.Name(),
		"runtime should have a name",
	)

	reg := te.Registry()
	assert.NotNil(t, reg, "Registry() should return non-nil")

	bus := te.EventBus()
	assert.NotNil(t, bus, "EventBus() should return non-nil")
}
