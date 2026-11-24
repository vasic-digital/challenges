// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: LicenseRef-Source-Available-All-Rights-Reserved

package main

import (
	"os"
	"testing"

	"digital.vasic.challenges/pkg/userflow"
)

// TestMain seeds the package-level platformGroups variable with
// representative groups that the tests in main_test.go exercise.
// platformGroups is normally loaded at startup from the JSON file
// supplied via --platform-groups-file; in tests there is no flag
// parse, so we inject the data directly.
//
// The service names (catalog-api, catalog-web, postgres, redis)
// mirror the Catalogizer docker-compose topology. They are only
// used as opaque strings by the framework — no project-specific
// logic lives in this binary.
func TestMain(m *testing.M) {
	platformGroups = map[string]userflow.PlatformGroup{
		"api": {
			Name:     "api",
			Services: []string{"catalog-api", "postgres", "redis"},
			CPULimit: 2.0,
			MemoryMB: 4096,
		},
		"web": {
			Name:     "web",
			Services: []string{"catalog-api", "catalog-web", "postgres", "redis"},
			CPULimit: 3.0,
			MemoryMB: 6144,
		},
		"desktop": {
			Name:     "desktop",
			Services: []string{"catalog-api", "postgres", "redis"},
			CPULimit: 2.0,
			MemoryMB: 4096,
		},
		"wizard": {
			Name:     "wizard",
			Services: []string{"catalog-api", "postgres", "redis"},
			CPULimit: 2.0,
			MemoryMB: 4096,
		},
		"android": {
			Name:     "android",
			Services: []string{"catalog-api", "postgres", "redis"},
			CPULimit: 2.0,
			MemoryMB: 4096,
		},
		"tv": {
			Name:     "tv",
			Services: []string{"catalog-api", "postgres", "redis"},
			CPULimit: 2.0,
			MemoryMB: 4096,
		},
	}
	os.Exit(m.Run())
}
