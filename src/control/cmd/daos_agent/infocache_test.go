//
// (C) Copyright 2020-2023 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package main

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"

	"github.com/daos-stack/daos/src/control/common"
	"github.com/daos-stack/daos/src/control/common/test"
	"github.com/daos-stack/daos/src/control/lib/cache"
	"github.com/daos-stack/daos/src/control/lib/control"
	"github.com/daos-stack/daos/src/control/lib/hardware"
	"github.com/daos-stack/daos/src/control/logging"
)

type testInfoCacheParams struct {
	mockGetAttachInfo      getAttachInfoFn
	mockScanFabric         fabricScanFn
	disableFabricCache     bool
	disableAttachInfoCache bool
	ctlInvoker             control.Invoker
}

func newTestInfoCache(t *testing.T, log logging.Logger, params testInfoCacheParams) *InfoCache {
	ic := &InfoCache{
		log:           log,
		getAttachInfo: params.mockGetAttachInfo,
		fabricScan:    params.mockScanFabric,
		getAddrInterface: func(name string) (addrFI, error) {
			return &mockNetInterface{
				addrs: []net.Addr{&net.IPNet{IP: net.IPv4(127, 0, 0, 1)}},
			}, nil
		},
		ctlInvoker: params.ctlInvoker,
		cache:      cache.ItemCache{},
	}
	if !params.disableAttachInfoCache {
		ic.EnableAttachInfoCache(0)
	}
	if !params.disableFabricCache {
		ic.EnableFabricCache(0)
	}
	return ic
}

func testFabricProviderSet(prov ...string) *hardware.FabricProviderSet {
	providers := []*hardware.FabricProvider{}
	for _, p := range prov {
		providers = append(providers, &hardware.FabricProvider{
			Name: p,
		})
	}
	return hardware.NewFabricProviderSet(providers...)
}

func TestAgent_NewInfoCache(t *testing.T) {
	for name, tc := range map[string]struct {
		cfg                *Config
		expEnabled         bool
		expIgnoredIfaces   common.StringSet
		expRefreshInterval time.Duration
	}{
		"default": {
			cfg:        &Config{},
			expEnabled: true,
		},
		"caches disabled": {
			cfg: &Config{
				DisableCache: true,
			},
		},
		"ignored interfaces": {
			cfg: &Config{
				ExcludeFabricIfaces: common.NewStringSet("eth0", "eth1"),
			},
			expEnabled:       true,
			expIgnoredIfaces: common.NewStringSet("eth0", "eth1"),
		},
		"refresh interval": {
			cfg: &Config{
				CacheRefreshIntervalMinutes: 5,
			},
			expEnabled:         true,
			expRefreshInterval: 5 * time.Minute,
		},
	} {
		t.Run(name, func(t *testing.T) {
			log, buf := logging.NewTestLogger(t.Name())
			defer test.ShowBufferOnFailure(t, buf)

			ic := NewInfoCache(log, tc.cfg)

			test.AssertEqual(t, tc.expEnabled, ic.IsAttachInfoCacheEnabled(), "")
			test.AssertEqual(t, tc.expEnabled, ic.IsFabricCacheEnabled(), "")

			test.AssertEqual(t, tc.expIgnoredIfaces, ic.ignoreIfaces, "")
			test.AssertEqual(t, tc.expRefreshInterval, ic.refreshInterval, "")
		})
	}
}

func TestAgent_InfoCache_EnableAttachInfoCache(t *testing.T) {
	for name, tc := range map[string]struct {
		ic              *InfoCache
		refreshInterval time.Duration
		expEnabled      bool
	}{
		"nil": {},
		"disabled": {
			ic:         newTestInfoCache(t, nil, testInfoCacheParams{disableAttachInfoCache: true}),
			expEnabled: true,
		},
		"already enabled": {
			ic:         newTestInfoCache(t, nil, testInfoCacheParams{}),
			expEnabled: true,
		},
		"refresh interval": {
			ic:              newTestInfoCache(t, nil, testInfoCacheParams{disableAttachInfoCache: true}),
			refreshInterval: time.Minute,
			expEnabled:      true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.ic.EnableAttachInfoCache(tc.refreshInterval)

			test.AssertEqual(t, tc.expEnabled, tc.ic.IsAttachInfoCacheEnabled(), "")
		})
	}
}

func TestAgent_InfoCache_DisableAttachInfoCache(t *testing.T) {
	for name, tc := range map[string]struct {
		ic *InfoCache
	}{
		"nil": {},
		"already disabled": {
			ic: newTestInfoCache(t, nil, testInfoCacheParams{disableAttachInfoCache: true}),
		},
		"enabled": {
			ic: newTestInfoCache(t, nil, testInfoCacheParams{}),
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.ic.DisableAttachInfoCache()

			test.AssertFalse(t, tc.ic.IsAttachInfoCacheEnabled(), "")
		})
	}
}

func TestAgent_InfoCache_EnableFabricCache(t *testing.T) {
	for name, tc := range map[string]struct {
		ic              *InfoCache
		refreshInterval time.Duration
		expEnabled      bool
	}{
		"nil": {},
		"disabled": {
			ic:         newTestInfoCache(t, nil, testInfoCacheParams{disableFabricCache: true}),
			expEnabled: true,
		},
		"already enabled": {
			ic:         newTestInfoCache(t, nil, testInfoCacheParams{}),
			expEnabled: true,
		},
		"refresh interval": {
			ic:              newTestInfoCache(t, nil, testInfoCacheParams{disableFabricCache: true}),
			refreshInterval: time.Minute,
			expEnabled:      true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.ic.EnableFabricCache(tc.refreshInterval)

			test.AssertEqual(t, tc.expEnabled, tc.ic.IsFabricCacheEnabled(), "")
		})
	}
}

func TestAgent_InfoCache_DisableFabricCache(t *testing.T) {
	for name, tc := range map[string]struct {
		ic *InfoCache
	}{
		"nil": {},
		"already disabled": {
			ic: newTestInfoCache(t, nil, testInfoCacheParams{disableFabricCache: true}),
		},
		"enabled": {
			ic: newTestInfoCache(t, nil, testInfoCacheParams{}),
		},
	} {
		t.Run(name, func(t *testing.T) {
			tc.ic.DisableFabricCache()

			test.AssertFalse(t, tc.ic.IsFabricCacheEnabled(), "")
		})
	}
}

func TestAgent_InfoCache_AddProvider(t *testing.T) {
	for name, tc := range map[string]struct {
		ic           *InfoCache
		input        string
		expProviders common.StringSet
	}{
		"nil": {
			input: "something",
		},
		"empty": {
			ic:           &InfoCache{},
			input:        "something",
			expProviders: common.NewStringSet("something"),
		},
		"add": {
			ic: &InfoCache{
				providers: common.NewStringSet("something"),
			},
			input:        "something else",
			expProviders: common.NewStringSet("something", "something else"),
		},
		"ignore empty string": {
			ic:    &InfoCache{},
			input: "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			log, buf := logging.NewTestLogger(t.Name())
			defer test.ShowBufferOnFailure(t, buf)

			if tc.ic != nil {
				tc.ic.log = log
			}

			tc.ic.AddProvider(tc.input)

			if tc.ic == nil {
				return
			}
			if diff := cmp.Diff(tc.expProviders, tc.ic.providers); diff != "" {
				t.Fatalf("want-, got+:\n%s", diff)
			}
		})
	}
}

func TestAgent_InfoCache_GetAttachInfo(t *testing.T) {
	ctlResp := &control.GetAttachInfoResp{
		System:       "dontcare",
		ServiceRanks: []*control.PrimaryServiceRank{{Rank: 1, Uri: "my uri"}},
		MSRanks:      []uint32{0, 1, 2, 3},
		ClientNetHint: control.ClientNetworkHint{
			Provider:    "ofi+tcp",
			NetDevClass: uint32(hardware.Ether),
		},
	}

	for name, tc := range map[string]struct {
		getInfoCache func(logging.Logger) *InfoCache
		remoteResp   *control.GetAttachInfoResp
		remoteErr    error
		expErr       error
		expResp      *control.GetAttachInfoResp
		expRemote    bool
		expCached    bool
	}{
		"nil": {
			expErr: errors.New("nil"),
		},
		"disabled": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				return newTestInfoCache(t, l, testInfoCacheParams{
					disableAttachInfoCache: true,
				})
			},
			remoteResp: ctlResp,
			expResp:    ctlResp,
			expRemote:  true,
		},
		"disabled fails fetch": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				return newTestInfoCache(t, l, testInfoCacheParams{
					disableAttachInfoCache: true,
				})
			},
			remoteErr: errors.New("mock remote"),
			expErr:    errors.New("mock remote"),
			expRemote: true,
		},
		"enabled but empty": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				return newTestInfoCache(t, l, testInfoCacheParams{})
			},
			remoteResp: ctlResp,
			expResp:    ctlResp,
			expRemote:  true,
			expCached:  true,
		},
		"enabled but empty fails fetch": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				return newTestInfoCache(t, l, testInfoCacheParams{})
			},
			remoteErr: errors.New("mock remote"),
			expErr:    errors.New("mock remote"),
			expRemote: true,
			expCached: true,
		},
		"enabled and cached": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				ic := newTestInfoCache(t, l, testInfoCacheParams{})
				ic.cache.Set(attachInfoKey, cache.NewItem(ctlResp))
				return ic
			},
			remoteErr: errors.New("shouldn't call remote"),
			expResp:   ctlResp,
			expCached: true,
		},
		"bad data type": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				ic := newTestInfoCache(t, l, testInfoCacheParams{})
				ic.cache.Set(attachInfoKey, cache.NewItem("bad data"))
				return ic
			},
			remoteErr: errors.New("shouldn't call remote"),
			expErr:    errors.New("data type"),
			expCached: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			log, buf := logging.NewTestLogger(t.Name())
			defer test.ShowBufferOnFailure(t, buf)

			var ic *InfoCache
			if tc.getInfoCache != nil {
				ic = tc.getInfoCache(log)
			}

			calledRemote := false
			if ic != nil {
				ic.getAttachInfo = func(_ context.Context, _ control.UnaryInvoker, _ *control.GetAttachInfoReq) (*control.GetAttachInfoResp, error) {
					calledRemote = true
					return tc.remoteResp, tc.remoteErr
				}
			}

			resp, err := ic.GetAttachInfo(test.Context(t), "dontcare")

			test.CmpErr(t, tc.expErr, err)
			if diff := cmp.Diff(tc.expResp, resp); diff != "" {
				t.Fatalf("want-, got+:\n%s", diff)
			}

			test.AssertEqual(t, tc.expRemote, calledRemote, "")

			if ic == nil {
				return
			}

			test.AssertEqual(t, tc.expCached, ic.cache.Has(attachInfoKey), "")
			if tc.expCached && tc.expResp != nil {
				cached, err := ic.cache.Get(test.Context(t), attachInfoKey)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(tc.expResp, cached); diff != "" {
					t.Fatalf("want-, got+:\n%s", diff)
				}
			}
		})
	}
}

func TestAgent_InfoCache_GetFabricDevice(t *testing.T) {
	testSet := hardware.NewFabricInterfaceSet(&hardware.FabricInterface{
		Name:          "dev0",
		NetInterfaces: common.NewStringSet("test0"),
		DeviceClass:   hardware.Ether,
		Providers:     hardware.NewFabricProviderSet(&hardware.FabricProvider{Name: "testprov"}),
	})
	for name, tc := range map[string]struct {
		getInfoCache    func(logging.Logger) *InfoCache
		devClass        hardware.NetDevClass
		provider        string
		fabricResp      *hardware.FabricInterfaceSet
		fabricErr       error
		expResult       *FabricInterface
		expErr          error
		expScan         bool
		expCachedFabric *hardware.FabricInterfaceSet
	}{
		"nil": {
			expErr: errors.New("nil"),
		},
		"disabled": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				return newTestInfoCache(t, l, testInfoCacheParams{
					disableFabricCache: true,
				})
			},
			devClass:   hardware.Ether,
			provider:   "testprov",
			fabricResp: testSet,
			expScan:    true,
			expResult: &FabricInterface{
				Name:        "test0",
				Domain:      "dev0",
				NetDevClass: hardware.Ether,
			},
		},
		"disabled fails fetch": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				return newTestInfoCache(t, l, testInfoCacheParams{
					disableFabricCache: true,
				})
			},
			devClass:  hardware.Ether,
			provider:  "testprov",
			fabricErr: errors.New("mock fabric scan"),
			expScan:   true,
			expErr:    errors.New("mock fabric scan"),
		},
		"enabled but empty": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				return newTestInfoCache(t, l, testInfoCacheParams{})
			},
			devClass:   hardware.Ether,
			provider:   "testprov",
			fabricResp: testSet,
			expScan:    true,
			expResult: &FabricInterface{
				Name:        "test0",
				Domain:      "dev0",
				NetDevClass: hardware.Ether,
			},
			expCachedFabric: testSet,
		},
		"enabled but empty fails fetch": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				return newTestInfoCache(t, l, testInfoCacheParams{})
			},
			devClass:  hardware.Ether,
			provider:  "testprov",
			fabricErr: errors.New("mock fabric scan"),
			expScan:   true,
			expErr:    errors.New("mock fabric scan"),
		},
		"enabled and cached": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				ic := newTestInfoCache(t, l, testInfoCacheParams{})
				nf := NUMAFabricFromScan(test.Context(t), l, testSet)
				nf.getAddrInterface = ic.getAddrInterface
				ic.cache.Set(fabricKey, cache.NewItem(nf))
				return ic
			},
			devClass:  hardware.Ether,
			provider:  "testprov",
			fabricErr: errors.New("shouldn't call scan"),
			expResult: &FabricInterface{
				Name:        "test0",
				Domain:      "dev0",
				NetDevClass: hardware.Ether,
			},
			expCachedFabric: hardware.NewFabricInterfaceSet(&hardware.FabricInterface{
				Name:          "dev0",
				NetInterfaces: common.NewStringSet("test0"),
				DeviceClass:   hardware.Ether,
				Providers:     hardware.NewFabricProviderSet(&hardware.FabricProvider{Name: "testprov"}),
			}),
		},
		"bad data type": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				ic := newTestInfoCache(t, l, testInfoCacheParams{})
				ic.cache.Set(fabricKey, cache.NewItem("garbage"))
				return ic
			},
			devClass:  hardware.Ether,
			provider:  "testprov",
			fabricErr: errors.New("shouldn't call scan"),
			expErr:    errors.New("data type"),
		},
		"requested not found": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				ic := newTestInfoCache(t, l, testInfoCacheParams{})
				nf := NUMAFabricFromScan(test.Context(t), l, testSet)
				nf.getAddrInterface = ic.getAddrInterface
				ic.cache.Set(fabricKey, cache.NewItem(nf))
				return ic
			},
			devClass:  hardware.Ether,
			provider:  "bad",
			fabricErr: errors.New("shouldn't call scan"),
			expErr:    errors.New("no suitable fabric interface"),
			expCachedFabric: hardware.NewFabricInterfaceSet(&hardware.FabricInterface{
				Name:          "dev0",
				NetInterfaces: common.NewStringSet("test0"),
				DeviceClass:   hardware.Ether,
				Providers:     hardware.NewFabricProviderSet(&hardware.FabricProvider{Name: "testprov"}),
			}),
		},
	} {
		t.Run(name, func(t *testing.T) {
			log, buf := logging.NewTestLogger(t.Name())
			defer test.ShowBufferOnFailure(t, buf)

			var ic *InfoCache
			if tc.getInfoCache != nil {
				ic = tc.getInfoCache(log)
			}

			calledScan := false
			if ic != nil {
				ic.fabricScan = func(_ context.Context, _ ...string) (*hardware.FabricInterfaceSet, error) {
					calledScan = true
					return tc.fabricResp, tc.fabricErr
				}
			}

			result, err := ic.GetFabricDevice(test.Context(t), 0, tc.devClass, tc.provider)

			test.CmpErr(t, tc.expErr, err)
			if diff := cmp.Diff(tc.expResult, result, cmpopts.IgnoreUnexported(FabricInterface{})); diff != "" {
				t.Fatalf("want-, got+:\n%s", diff)
			}

			test.AssertEqual(t, tc.expScan, calledScan, "")

			if ic == nil {
				return
			}

			if tc.expCachedFabric != nil {
				data, err := ic.cache.Get(test.Context(t), fabricKey)
				if err != nil {
					t.Fatal(err)
				}

				cached, ok := data.(*NUMAFabric)
				test.AssertTrue(t, ok, "bad cached data type")

				expNF := NUMAFabricFromScan(test.Context(t), log, tc.expCachedFabric)
				if diff := cmp.Diff(expNF.numaMap, cached.numaMap, cmpopts.IgnoreUnexported(FabricInterface{})); diff != "" {
					t.Fatalf("want-, got+:\n%s", diff)
				}
			}
		})
	}
}

func TestAgent_InfoCache_Refresh(t *testing.T) {
	ctlResp := &control.GetAttachInfoResp{
		System:       "dontcare",
		ServiceRanks: []*control.PrimaryServiceRank{{Rank: 1, Uri: "my uri"}},
		MSRanks:      []uint32{0, 1, 2, 3},
		ClientNetHint: control.ClientNetworkHint{
			Provider:    "ofi+tcp",
			NetDevClass: uint32(hardware.Ether),
		},
	}

	testSet := hardware.NewFabricInterfaceSet(&hardware.FabricInterface{
		Name:          "dev0",
		NetInterfaces: common.NewStringSet("test0"),
		DeviceClass:   hardware.Ether,
		Providers:     hardware.NewFabricProviderSet(&hardware.FabricProvider{Name: "testprov"}),
	})

	for name, tc := range map[string]struct {
		getInfoCache        func(logging.Logger) *InfoCache
		attachInfoResp      *control.GetAttachInfoResp
		attachInfoErr       error
		fabricResp          *hardware.FabricInterfaceSet
		fabricErr           error
		expErr              error
		expScan             bool
		expRemote           bool
		expCachedFabric     *hardware.FabricInterfaceSet
		expCachedAttachInfo *control.GetAttachInfoResp
	}{
		"nil": {
			expErr: errors.New("nil"),
		},
		"both disabled": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				return newTestInfoCache(t, l, testInfoCacheParams{
					disableFabricCache:     true,
					disableAttachInfoCache: true,
				})
			},
		},
		"both enabled": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				return newTestInfoCache(t, l, testInfoCacheParams{})
			},
			attachInfoResp:      ctlResp,
			fabricResp:          testSet,
			expScan:             true,
			expRemote:           true,
			expCachedFabric:     testSet,
			expCachedAttachInfo: ctlResp,
		},
		"fabric disabled": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				return newTestInfoCache(t, l, testInfoCacheParams{
					disableFabricCache: true,
				})
			},
			attachInfoResp:      ctlResp,
			fabricErr:           errors.New("should not call scan"),
			expRemote:           true,
			expCachedAttachInfo: ctlResp,
		},
		"attach info disabled": {
			getInfoCache: func(l logging.Logger) *InfoCache {
				return newTestInfoCache(t, l, testInfoCacheParams{
					disableAttachInfoCache: true,
				})
			},
			attachInfoErr:   errors.New("should not call remote"),
			fabricResp:      testSet,
			expScan:         true,
			expCachedFabric: testSet,
		},
	} {
		t.Run(name, func(t *testing.T) {
			log, buf := logging.NewTestLogger(t.Name())
			defer test.ShowBufferOnFailure(t, buf)

			var ic *InfoCache
			if tc.getInfoCache != nil {
				ic = tc.getInfoCache(log)
			}

			calledScan := false
			calledRemote := false
			if ic != nil {
				ic.fabricScan = func(_ context.Context, _ ...string) (*hardware.FabricInterfaceSet, error) {
					calledScan = true
					return tc.fabricResp, tc.fabricErr
				}
				ic.getAttachInfo = func(_ context.Context, _ control.UnaryInvoker, _ *control.GetAttachInfoReq) (*control.GetAttachInfoResp, error) {
					calledRemote = true
					return tc.attachInfoResp, tc.attachInfoErr
				}
			}

			err := ic.Refresh(test.Context(t))

			test.CmpErr(t, tc.expErr, err)
			test.AssertEqual(t, tc.expScan, calledScan, "")
			test.AssertEqual(t, tc.expRemote, calledRemote, "")

			if tc.expCachedFabric != nil {
				data, err := ic.cache.Get(test.Context(t), fabricKey)
				if err != nil {
					t.Fatal(err)
				}

				cached, ok := data.(*NUMAFabric)
				test.AssertTrue(t, ok, "bad cached data type")

				expNF := NUMAFabricFromScan(test.Context(t), log, tc.expCachedFabric)
				if diff := cmp.Diff(expNF.numaMap, cached.numaMap, cmpopts.IgnoreUnexported(FabricInterface{})); diff != "" {
					t.Fatalf("want-, got+:\n%s", diff)
				}
			}

			if tc.expCachedAttachInfo != nil {
				cached, err := ic.cache.Get(test.Context(t), attachInfoKey)
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(tc.expCachedAttachInfo, cached); diff != "" {
					t.Fatalf("want-, got+:\n%s", diff)
				}
			}
		})
	}
}
