//
// (C) Copyright 2019-2022 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package main

import (
	"github.com/daos-stack/daos/src/control/common/cmdutil"
	"github.com/daos-stack/daos/src/control/lib/support"
)

type SupportCmd struct {
	CollectLog collectLogCmd `command:"collectlog" description:"Collect logs from server"`
}

// collectLogCmd is the struct representing the command to scan the machine for network interface devices
// that match the given fabric provider.
type collectLogCmd struct {
	optCfgCmd
	cmdutil.LogCmd
	Continue     bool   `short:"c" long:"Continue" description:"Continue collecting logs and ignore any errors"`
	TargetFolder string `short:"s" long:"loglocation" description:"Folder location where log is going to be copied"`
	Archive      bool   `short:"z" long:"archive" description:"Archive the log/config files"`
}

func (cmd *collectLogCmd) Execute(_ []string) error {
	if cmd.TargetFolder == "" {
		cmd.TargetFolder = "/tmp/daos_support_server_logs"
	}

	cmd.Infof("Support Logs will be copied to %s", cmd.TargetFolder)

	params := support.Params{}
	params.Config = cmd.configPath()
	params.Continue = cmd.Continue
	params.TargetFolder = cmd.TargetFolder

	err := support.CollectServerLog(cmd.Logger, params)
	if err != nil {
		return err
	}

	if cmd.Archive == true {
		err = support.ArchiveLogs(cmd.Logger, params)
		if err != nil {
			return err
		}
	}

	return nil
}
