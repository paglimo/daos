//
// (C) Copyright 2019-2022 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package main

import (
	"context"
	"os"
	"strings"

	"github.com/daos-stack/daos/src/control/cmd/dmg/pretty"
	"github.com/daos-stack/daos/src/control/lib/control"
	"github.com/daos-stack/daos/src/control/lib/support"
)

// NetCmd is the struct representing the top-level network subcommand.
type SupportCmd struct {
	CollectLog collectLogCmd `command:"collectlog" description:"Collect logs from servers"`
}

// collectLogCmd is the struct representing the command to collect the logs from the servers for support purpose
type collectLogCmd struct {
	baseCmd
	cfgCmd
	ctlInvokerCmd
	hostListCmd
	jsonOutputCmd
	Continue     bool   `short:"c" long:"Continue" description:"Continue collecting logs and ignore any errors"`
	TargetFolder string `short:"s" long:"loglocation" description:"Folder location where log is going to be copied"`
	Archive      bool   `short:"z" long:"archive" description:"Archive the log/config files"`
}

func (cmd *collectLogCmd) Execute(_ []string) error {
	if cmd.TargetFolder == "" {
		cmd.TargetFolder = "/tmp/daos_support_logs"
	}

	if err := os.Mkdir(cmd.TargetFolder, 0700); err != nil && !os.IsExist(err) {
		return err
	}

	ctx := context.Background()
	req := &control.CollectLogReq{
		TargetFolder: cmd.TargetFolder,
		Continue:     cmd.Continue,
	}

	cmd.Infof("Support Logs will be copied to %s", cmd.TargetFolder)

	req.SetHostList(cmd.hostlist)

	resp, err := control.CollectLog(ctx, cmd.ctlInvoker, req)

	if cmd.jsonOutputEnabled() {
		return cmd.outputJSON(resp, err)
	}

	if err != nil {
		return err
	}

	var bld strings.Builder
	if err := pretty.PrintResponseErrors(resp, &bld); err != nil {
		return err
	}

	params := support.Params{}
	params.Hostlist = strings.Join(cmd.hostlist, " ")
	params.Continue = cmd.Continue
	params.TargetFolder = cmd.TargetFolder
	params.Config = cmd.cfgCmd.config.Path

	err = support.CollectDmgSysteminfo(cmd.Logger, params)
	if err != nil && cmd.Continue == false {
		return err
	}

	err = support.CollectDmgNodeinfo(cmd.Logger, params)
	if err != nil && cmd.Continue == false {
		return err
	}

	if cmd.Archive == true {
		err = support.ArchiveLogs(cmd.Logger, params)
		if err != nil {
			return err
		}
	}

	cmd.Info(bld.String())
	return resp.Errors()
}
