//
// (C) Copyright 2020-2021 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

// Package build provides an importable repository of variables set at build time.
package build

import (
	"fmt"
	"runtime/debug"
	"strings"
)

var (
	// ConfigDir should be set via linker flag using the value of CONF_DIR.
	ConfigDir string = "./"
	// DaosVersion should be set via linker flag using the value of DAOS_VERSION.
	DaosVersion string = "unset"
	// ControlPlaneName defines a consistent name for the control plane server.
	ControlPlaneName = "DAOS Control Server"
	// DataPlaneName defines a consistent name for the engine.
	DataPlaneName = "DAOS I/O Engine"
	// ManagementServiceName defines a consistent name for the Management Service.
	ManagementServiceName = "DAOS Management Service"
	// AgentName defines a consistent name for the compute node agent.
	AgentName = "DAOS Agent"

	// DefaultControlPort defines the default control plane listener port.
	DefaultControlPort = 10001

	// DefaultSystemName defines the default DAOS system name.
	DefaultSystemName = "daos_server"
)

func revString() string {
	var revString string
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				revString = fmt.Sprintf("-g%s", fmt.Sprintf("%10s", setting.Value)[0:10])
			case "-tags":
				if strings.Contains(setting.Value, "release") {
					return ""
				}
			}
		}
	}
	return revString
}

// VersionString returns a string containing the name, version, and for non-release builds,
// the revision of the binary.
func VersionString(name string) string {
	return fmt.Sprintf("%s version %s%s", name, DaosVersion, revString())
}
