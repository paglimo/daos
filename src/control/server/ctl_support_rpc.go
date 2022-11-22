//
// (C) Copyright 2019-2022 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package server

import (
	// "os"
	// "path/filepath"

	"golang.org/x/net/context"

	ctlpb "github.com/daos-stack/daos/src/control/common/proto/ctl"
	"github.com/daos-stack/daos/src/control/lib/support"
)

// CollectLog retrieves details of network interfaces on remote hosts.
func (c *ControlService) CollectLog(ctx context.Context, req *ctlpb.CollectLogReq) (*ctlpb.CollectLogResp, error) {
	c.log.Infof("CollectLog: LogFolder location is %s", req.Loglocation)

	params := support.Params{}
	params.Continue = req.Continue

	err := support.CollectServerLog(req.Loglocation, c.log, params)
	if err != nil {
		return nil, err
	}

	resp := new(ctlpb.CollectLogResp)
	return resp, nil
}
