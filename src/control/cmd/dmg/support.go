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
	Stop         bool   `short:"s" long:"stop" description:"Stop the collectlog command on very first error"`
	TargetFolder string `short:"t" long:"target" description:"Target Folder location where log will be copied"`
	Archive      bool   `short:"z" long:"archive" description:"Archive the log/config files"`
	CustomLogs   string `short:"c" long:"custom-logs" description:"Collect the Logs from given directory"`
}

func (cmd *collectLogCmd) Execute(_ []string) error {
	if cmd.TargetFolder == "" {
		cmd.TargetFolder = "/tmp/daos_support_server_logs"
	}

	if err := os.Mkdir(cmd.TargetFolder, 0700); err != nil && !os.IsExist(err) {
		return err
	}

	ctx := context.Background()
	req := &control.CollectLogReq{
		TargetFolder: cmd.TargetFolder,
		Stop:         cmd.Stop,
		CustomLogs:   cmd.CustomLogs,
		JsonOutput:   cmd.jsonOutputEnabled(),
	}

	req.SetHostList(cmd.hostlist)

	resp, err := control.CollectLog(ctx, cmd.ctlInvoker, req)

	if err != nil && cmd.Stop == true {
		return err
	}

	params := support.Params{}
	params.Hostlist = strings.Join(cmd.hostlist, " ")
	params.Stop = cmd.Stop
	params.TargetFolder = cmd.TargetFolder
	params.Config = cmd.cfgCmd.config.Path
	params.JsonOutput = cmd.jsonOutputEnabled()

	err = support.CollectDmgSysteminfo(cmd.Logger, params)
	if err != nil && cmd.Stop == true {
		return err
	}

	err = support.CollectDmgNodeinfo(cmd.Logger, params)
	if err != nil && cmd.Stop == true {
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

	var bld strings.Builder
	if err := pretty.PrintResponseErrors(resp, &bld); err != nil {
		return err
	}

	if cmd.jsonOutputEnabled() {
		return cmd.outputJSON(resp, err)
	}

	cmd.Info(bld.String())
	return resp.Errors()
}
