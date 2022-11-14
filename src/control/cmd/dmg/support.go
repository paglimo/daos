//
// (C) Copyright 2019-2022 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package main

import (
	"context"
	"strings"
	"os"

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
	TargetFolder string `short:"s" long:"loglocation" description:"Folder location where log is going to be copied"`
}

func (cmd *collectLogCmd) Execute(_ []string) error {
	if err := os.Mkdir(cmd.TargetFolder, 0700); err != nil && !os.IsExist(err) {
		return err
	}

	ctx := context.Background()
	req := &control.CollectLogReq{
		Loglocation: cmd.TargetFolder,
	}

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

	err = support.Collectdmglog(cmd.TargetFolder, cmd.cfgCmd.config.Path)
	if err != nil {
		return err
	}

	cmd.Info(bld.String())
	return resp.Errors()
}
