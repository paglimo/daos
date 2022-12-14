//
// (C) Copyright 2019-2022 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package main

import (
	"context"
	"fmt"
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
	// Total 8 group of for dmg support collection
	progress := support.ProgressBar{1, 8, 0, cmd.jsonOutputEnabled()}

	if cmd.TargetFolder == "" {
		cmd.TargetFolder = "/tmp/daos_support_server_logs"
	}
	cmd.Infof("Support logs will be copied to %s", cmd.TargetFolder)

	hostName, _ := support.GetHostName()
	var LogCollection = map[string][]string{
		"CopyServerConfig":     {""},
		"CollectSystemCmd":     support.SystemCmd,
		"CollectServerLog":     support.ServerLog,
		"CollectDaosServerCmd": support.DaosServerCmd,
	}

	if err := os.Mkdir(cmd.TargetFolder, 0700); err != nil && !os.IsExist(err) {
		return err
	}

	// Copy the custome log location
	if cmd.CustomLogs != "" {
		LogCollection["CollectCustomLogs"] = []string{""}
		progress.Total = progress.Total + 1
	}
	progress.Steps = 100 / progress.Total

	for logfunc, logcmdset := range LogCollection {
		for _, logcmd := range logcmdset {
			cmd.Debugf("Log Function %s -- Log Collect Cmd %s ", logfunc, logcmd)
			ctx := context.Background()
			req := &control.CollectLogReq{
				TargetFolder: cmd.TargetFolder,
				CustomLogs:   cmd.CustomLogs,
				LogFunction:  logfunc,
				LogCmd:       logcmd,
			}
			req.SetHostList(cmd.hostlist)
			resp, err := control.CollectLog(ctx, cmd.ctlInvoker, req)
			if err != nil && cmd.Stop == true {
				return err
			}
			if len(resp.GetHostErrors()) > 0 {
				var bld strings.Builder
				_ = pretty.PrintResponseErrors(resp, &bld)
				cmd.Info(bld.String())
				if cmd.Stop == true {
					return resp.Errors()
				}
			}
		}
		support.PrintProgress(&progress)
	}

	// Rsync the logs from servers
	req := &control.CollectLogReq{
		TargetFolder: cmd.TargetFolder,
		LogFunction:  "rsyncLog",
		LogCmd:       hostName,
	}
	cmd.Debugf("Rsync logs from servers to %s:%s ", hostName, cmd.TargetFolder)
	resp, err := control.CollectLog(context.Background(), cmd.ctlInvoker, req)
	if err != nil && cmd.Stop == true {
		return err
	}
	if len(resp.GetHostErrors()) > 0 {
		var bld strings.Builder
		_ = pretty.PrintResponseErrors(resp, &bld)
		cmd.Info(bld.String())
		if cmd.Stop == true {
			return resp.Errors()
		}
	}
	support.PrintProgress(&progress)

	// Collect dmg command output
	var DmgInfoCollection = map[string][]string{
		"CollectDmgCmd":      support.DmgCmd,
		"CollectDmgDiskInfo": {""},
	}

	params := support.Params{}
	params.Config = cmd.cfgCmd.config.Path
	params.TargetFolder = cmd.TargetFolder
	params.CustomLogs = cmd.CustomLogs
	params.JsonOutput = cmd.jsonOutputEnabled()
	params.Hostlist = strings.Join(cmd.hostlist, " ")
	for logfunc, logcmdset := range DmgInfoCollection {
		for _, logcmd := range logcmdset {
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
		support.PrintProgress(&progress)
	}

	if cmd.Archive == true {
		cmd.Infof("Archiving the Log Folder %s", cmd.TargetFolder)
		err := support.ArchiveLogs(cmd.Logger, params)
		if err != nil {
			return err
		}

		for i := 1; i < 3; i++ {
			os.RemoveAll(cmd.TargetFolder)
		}
	}

	support.PrintProgressEnd(&progress)

	return nil
}
