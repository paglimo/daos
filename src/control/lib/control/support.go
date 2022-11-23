//
// (C) Copyright 2018-2022 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package control

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	ctlpb "github.com/daos-stack/daos/src/control/common/proto/ctl"
)

var DmgLogCollectCmd = [...]string{
	"dmg system get-prop",
	"dmg system query",
	"dmg system list-pools",
	"dmg system leader-query",
	"dmg system get-attr",
	"dmg network scan",
	"dmg storage scan",
	"dmg storage scan -n",
	"dmg storage scan -m",
	"dmg storage query list-pools -v",
	"dmg storage query usage",
}

const DmgListDeviceCmd = "dmg storage query list-devices"
const DmgDeviceHealthCmd = "dmg storage query device-health"

var DasoAgnetInfoCmd = [...]string{
	"daos_agent version",
	"daos_agent net-scan",
	"daos_agent dump-topology",
}

var SysInfoCmd = [...]string{
	"daos_server version",
	"dmesg",
	"lspci -D",
	"top -bcn1 -w512",
}

type (
	// CollectLogReq contains the parameters for a network scan request.
	CollectLogReq struct {
		unaryRequest
		TargetFolder string
		Continue     bool
	}

	// CollectLogResp contains the results of a network scan.
	CollectLogResp struct {
		HostErrorsResp
	}
)

// CollectLog concurrently performs log collection across all hosts
// supplied in the request's hostlist, or all configured hosts if not
// explicitly specified. The function blocks until all results (successful
// or otherwise) are received, and returns a single response structure
// containing results for all host log collection operations.
func CollectLog(ctx context.Context, rpcClient UnaryInvoker, req *CollectLogReq) (*CollectLogResp, error) {
	req.setRPC(func(ctx context.Context, conn *grpc.ClientConn) (proto.Message, error) {
		return ctlpb.NewCtlSvcClient(conn).CollectLog(ctx, &ctlpb.CollectLogReq{
			TargetFolder: req.TargetFolder,
			Continue:     req.Continue,
		})
	})

	ur, err := rpcClient.InvokeUnaryRPC(ctx, req)
	if err != nil {
		return nil, err
	}

	nsr := new(CollectLogResp)
	for _, hostResp := range ur.Responses {
		if hostResp.Error != nil {
			if err := nsr.addHostError(hostResp.Addr, hostResp.Error); err != nil {
				return nil, err
			}
			continue
		}
	}

	return nsr, nil
}
