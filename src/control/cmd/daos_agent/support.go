//
// (C) Copyright 2019-2022 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package main

import (
	"os"

	"github.com/daos-stack/daos/src/control/common/cmdutil"
	"github.com/daos-stack/daos/src/control/lib/support"
)

type SupportCmd struct {
	CollectLog collectLogCmd `command:"collectlog" description:"Collect logs from client"`
}

// collectLogCmd is the struct representing the command to collect the log from client side.
type collectLogCmd struct {
	configCmd
	cmdutil.LogCmd
	Stop         bool   `short:"s" long:"Stop" description:"Stop the collectlog command on very first error"`
	TargetFolder string `short:"t" long:"loglocation" description:"Folder location where log is going to be copied"`
	Archive      bool   `short:"z" long:"archive" description:"Archive the log/config files"`
	CustomLogs   string `short:"c" long:"custom-logs" description:"Collect the Logs from given directory"`
}

func (cmd *collectLogCmd) Execute(_ []string) error {
	if cmd.TargetFolder == "" {
		cmd.TargetFolder = "/tmp/daos_support_client_logs"
	}

	cmd.Infof("Support Logs will be copied to %s", cmd.TargetFolder)
	cmd.Infof("Agent Config %s", cmd.cfg.LogFile)

	params := support.Params{}
	params.Stop = cmd.Stop
	params.TargetFolder = cmd.TargetFolder
	params.CustomLogs = cmd.CustomLogs

	err := support.CollectClientLog(cmd.Logger, params)
	if err != nil {
		return err
	}

	if cmd.Archive == true {
		err = support.ArchiveLogs(cmd.Logger, params)
		if err != nil {
			return err
		}

		err = os.RemoveAll(params.TargetFolder)
		if err != nil {
			return err
		}
	}

	return nil
}
