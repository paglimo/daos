//
// (C) Copyright 2019-2022 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package main

import (
	"fmt"
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

	var LogCollection = map[string][]string{
		"CollectAgnetCmd":  support.AgnetCmd,
		"CollectClientLog": {""},
		"CollectSystemCmd": support.SystemCmd,
	}

	// Copy the custome log location
	if cmd.CustomLogs != "" {
		LogCollection["CollectCustomLogs"] = []string{""}
	}

	params := support.Params{}
	params.TargetFolder = cmd.TargetFolder
	params.CustomLogs = cmd.CustomLogs
	for logfunc, logcmdset := range LogCollection {
		for _, logcmd := range logcmdset {
			cmd.Debugf("Log Function %s -- Log Collect Cmd %s ", logfunc, logcmd)
			params.LogFunction = logfunc
			params.LogCmd = logcmd

			err := support.CollectSupportLog(cmd.Logger, params)
			if err != nil {
				fmt.Println(err)
				if cmd.Stop == true {
					return err
				}
			}
		}
	}

	if cmd.Archive == true {
		cmd.Debugf("Archiving the Log Folder %s", cmd.TargetFolder)
		err := support.ArchiveLogs(cmd.Logger, params)
		if err != nil {
			return err
		}

		for i := 1; i < 3; i++ {
			os.RemoveAll(cmd.TargetFolder)
		}
	}

	return nil
}
