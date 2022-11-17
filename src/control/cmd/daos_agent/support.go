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
	CollectLog collectLogCmd `command:"collectlog" description:"Collect logs from client"`
}

// collectLogCmd is the struct representing the command to collect the log from client side.
type collectLogCmd struct {
	cmdutil.LogCmd
	TargetFolder string `short:"s" long:"loglocation" description:"Folder location where log is going to be copied"`
	Archive bool `short:"z" long:"archive" description:"Archive the log/config files"`
}

func (cmd *collectLogCmd) Execute(_ []string) error {
	if cmd.TargetFolder == "" {
		cmd.TargetFolder = "/tmp/daos_support_logs"
	}

	err := support.CollectClientLog(cmd.TargetFolder)
	if err != nil {
		return err
	}

	if cmd.Archive == true {
		err = support.ArchiveLogs(cmd.TargetFolder)
		if err != nil {
			return err
		}
	}

	return nil
}
