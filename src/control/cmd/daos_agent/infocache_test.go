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

	"github.com/daos-stack/daos/src/control/common"
	"github.com/daos-stack/daos/src/control/common/test"
	"github.com/daos-stack/daos/src/control/lib/cache"
	"github.com/daos-stack/daos/src/control/lib/control"
	"github.com/daos-stack/daos/src/control/lib/hardware"
	"github.com/daos-stack/daos/src/control/logging"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
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
		client: params.ctlInvoker,
		cache:  cache.NewItemCache(log),
	}
	if !params.disableAttachInfoCache {
		ic.EnableAttachInfoCache(0)
	}
	if !params.disableFabricCache {
		ic.EnableFabricCache()
	}
	return ic
}

func TestAgent_newCachedAttachInfo(t *testing.T) {
	log, buf := logging.NewTestLogger(t.Name())
	defer test.ShowBufferOnFailure(t, buf)

	expSys := "my_system"
	expRefreshInterval := time.Second
	expClient := control.NewMockInvoker(log, &control.MockInvokerConfig{})

	ai := newCachedAttachInfo(expRefreshInterval, expSys, expClient)

	test.AssertEqual(t, expSys, ai.system, "")
	test.AssertEqual(t, expRefreshInterval, ai.refreshInterval, "")
	test.AssertEqual(t, expClient, ai.rpcClient, "")
	test.AssertEqual(t, time.Time{}, ai.lastCached, "")
	if ai.lastResponse != nil {
		t.Fatalf("expected nothing cached, found:\n%+v", ai.lastResponse)
	}
	if ai.refreshFn == nil {
		t.Fatalf("expected refresh function to be non-nil")
	}
}

func TestAgent_cachedAttachInfo_Key(t *testing.T) {
	for name, tc := range map[string]struct {
		ai        *cachedAttachInfo
		expResult string
	}{
		"nil": {},
		"no system name": {
			ai:        newCachedAttachInfo(0, "", nil),
			expResult: "GetAttachInfo",
		},
		"system name": {
			ai:        newCachedAttachInfo(0, "my_system", nil),
			expResult: "GetAttachInfo-my_system",
		},
	} {
		t.Run(name, func(t *testing.T) {
			test.AssertEqual(t, tc.expResult, tc.ai.Key(), "")
		})
	}
}

func TestAgent_cachedAttachInfo_NeedsRefresh(t *testing.T) {
	for name, tc := range map[string]struct {
		ai        *cachedAttachInfo
		expResult bool
	}{
		"nil": {},
		"never cached": {
			ai:        newCachedAttachInfo(0, "test", nil),
			expResult: true,
		},
		"no refresh": {
			ai: &cachedAttachInfo{
				cacheItem: cacheItem{
					lastCached: time.Now().Add(-time.Minute),
				},
				lastResponse: &control.GetAttachInfoResp{},
			},
		},
		"expired": {
			ai: &cachedAttachInfo{
				cacheItem: cacheItem{
					lastCached:      time.Now().Add(-time.Minute),
					refreshInterval: time.Second,
				},
				lastResponse: &control.GetAttachInfoResp{},
			},
			expResult: true,
		},
		"not expired": {
			ai: &cachedAttachInfo{
				cacheItem: cacheItem{
					lastCached:      time.Now().Add(-time.Second),
					refreshInterval: time.Minute,
				},
				lastResponse: &control.GetAttachInfoResp{},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			test.AssertEqual(t, tc.expResult, tc.ai.NeedsRefresh(), "")
		})
	}
}

func TestAgent_cachedAttachInfo_Refresh(t *testing.T) {
	resp1 := &control.GetAttachInfoResp{
		System: "resp1",
		ServiceRanks: []*control.PrimaryServiceRank{
			{
				Rank: 1,
				Uri:  "rank one",
			},
			{
				Rank: 2,
				Uri:  "rank two",
			},
		},
		MSRanks: []uint32{0, 1, 2},
		ClientNetHint: control.ClientNetworkHint{
			Provider:    "prov",
			NetDevClass: uint32(hardware.Ether),
		},
	}

	resp2 := &control.GetAttachInfoResp{
		System: "resp2",
		ServiceRanks: []*control.PrimaryServiceRank{
			{
				Rank: 3,
				Uri:  "rank three",
			},
			{
				Rank: 4,
				Uri:  "rank four",
			},
		},
		MSRanks: []uint32{1, 3},
		ClientNetHint: control.ClientNetworkHint{
			Provider:    "other",
			NetDevClass: uint32(hardware.Infiniband),
		},
	}

	for name, tc := range map[string]struct {
		nilCache      bool
		ctlResult     *control.GetAttachInfoResp
		ctlErr        error
		alreadyCached *control.GetAttachInfoResp
		expErr        error
		expCached     *control.GetAttachInfoResp
	}{
		"nil": {
			nilCache: true,
			expErr:   errors.New("nil"),
		},
		"GetAttachInfo fails": {
			ctlErr: errors.New("mock GetAttachInfo"),
			expErr: errors.New("mock GetAttachInfo"),
		},
		"not initialized": {
			ctlResult: resp1,
			expCached: resp1,
		},
		"previously cached": {
			ctlResult:     resp2,
			alreadyCached: resp1,
			expCached:     resp2,
		},
	} {
		t.Run(name, func(t *testing.T) {
			var ai *cachedAttachInfo
			if !tc.nilCache {
				ai = newCachedAttachInfo(0, "test", control.DefaultClient())
				ai.refreshFn = func(_ context.Context, _ control.UnaryInvoker, _ *control.GetAttachInfoReq) (*control.GetAttachInfoResp, error) {
					return tc.ctlResult, tc.ctlErr
				}
				ai.lastResponse = tc.alreadyCached
				if ai.lastResponse != nil {
					ai.lastCached = time.Now()
				}
			}

			err := ai.Refresh(test.Context(t))

			test.CmpErr(t, tc.expErr, err)

			if ai == nil {
				return
			}

			if diff := cmp.Diff(tc.expCached, ai.lastResponse); diff != "" {
				t.Fatalf("-want, +got:\n%s", diff)
			}
		})
	}
}

func TestAgent_newCachedFabricInfo(t *testing.T) {
	log, buf := logging.NewTestLogger(t.Name())
	defer test.ShowBufferOnFailure(t, buf)

	expIgnored := common.NewStringSet("if0", "if1", "eth0")
	expProviders := []string{"prov1", "prov2"}

	cfi := newCachedFabricInfo(log, expIgnored, expProviders...)

	test.AssertEqual(t, time.Duration(0), cfi.refreshInterval, "")
	test.AssertEqual(t, time.Time{}, cfi.lastCached, "")
	if cfi.lastResults != nil {
		t.Fatalf("expected nothing cached, found:\n%+v", cfi.lastResults)
	}
	if cfi.refreshFn == nil {
		t.Fatalf("expected refresh function to be non-nil")
	}
}

func TestAgent_cachedFabricInfo_Key(t *testing.T) {
	for name, tc := range map[string]struct {
		cfi *cachedFabricInfo
	}{
		"nil": {},
		"normal": {
			cfi: newCachedFabricInfo(nil, nil),
		},
	} {
		t.Run(name, func(t *testing.T) {
			// should always be the same
			test.AssertEqual(t, fabricKey, tc.cfi.Key(), "")
		})
	}
}

func TestAgent_cachedFabricInfo_NeedsRefresh(t *testing.T) {
	for name, tc := range map[string]struct {
		nilCache  bool
		cacheTime time.Time
		expResult bool
	}{
		"nil": {
			nilCache: true,
		},
		"not initialized": {
			expResult: true,
		},
		"initialized": {
			cacheTime: time.Now().Add(-time.Minute),
		},
	} {
		t.Run(name, func(t *testing.T) {
			log, buf := logging.NewTestLogger(t.Name())
			defer test.ShowBufferOnFailure(t, buf)

			var cfi *cachedFabricInfo
			if !tc.nilCache {
				cfi = newCachedFabricInfo(log, nil)
				cfi.cacheItem.lastCached = tc.cacheTime
			}

			test.AssertEqual(t, tc.expResult, cfi.NeedsRefresh(), "")
		})
	}
}

func TestAgent_cachedFabricInfo_Refresh(t *testing.T) {
	scan1 := map[int][]*FabricInterface{
		2: {
			{Name: "two"},
		},
	}
	scan2 := map[int][]*FabricInterface{
		1: {
			{Name: "one"},
		},
		3: {
			{Name: "three"},
		},
	}

	for name, tc := range map[string]struct {
		nilCache      bool
		fabricResult  map[int][]*FabricInterface
		fabricErr     error
		alreadyCached map[int][]*FabricInterface
		expErr        error
		expCached     map[int][]*FabricInterface
	}{
		"nil": {
			nilCache: true,
			expErr:   errors.New("nil"),
		},
		"fabric scan fails": {
			fabricErr: errors.New("mock fabric scan"),
			expErr:    errors.New("mock fabric scan"),
		},
		"not initialized": {
			fabricResult: scan1,
			expCached:    scan1,
		},
		"previously cached": {
			fabricResult:  scan2,
			alreadyCached: scan1,
			expCached:     scan2,
		},
	} {
		t.Run(name, func(t *testing.T) {
			log, buf := logging.NewTestLogger(t.Name())
			defer test.ShowBufferOnFailure(t, buf)

			var cfi *cachedFabricInfo
			if !tc.nilCache {
				cfi = newCachedFabricInfo(log, nil)
				cfi.refreshFn = func(_ context.Context, _ ...string) (*NUMAFabric, error) {
					if tc.fabricResult != nil {
						return &NUMAFabric{
							numaMap: tc.fabricResult,
						}, nil
					}
					return nil, tc.fabricErr
				}
				if tc.alreadyCached != nil {
					cfi.lastResults = &NUMAFabric{
						numaMap: tc.alreadyCached,
					}
					cfi.lastCached = time.Now()
				}
			}

			err := cfi.Refresh(test.Context(t))

			test.CmpErr(t, tc.expErr, err)

			if cfi == nil {
				return
			}

			if tc.expCached == nil {
				if cfi.lastResults != nil {
					t.Fatalf("expected empty cache, got %+v", cfi.lastResults)
				}
				return
			}

			if diff := cmp.Diff(tc.expCached, cfi.lastResults.numaMap, cmpopts.IgnoreUnexported(FabricInterface{})); diff != "" {
				t.Fatalf("-want, +got:\n%s", diff)
			}
		})
	}
}

// func TestAgent_NewInfoCache(t *testing.T) {
// 	for name, tc := range map[string]struct {
// 		cfg                *Config
// 		expEnabled         bool
// 		expIgnoredIfaces   common.StringSet
// 		expRefreshInterval time.Duration
// 	}{
// 		"default": {
// 			cfg:        &Config{},
// 			expEnabled: true,
// 		},
// 		"caches disabled": {
// 			cfg: &Config{
// 				DisableCache: true,
// 			},
// 		},
// 		"ignored interfaces": {
// 			cfg: &Config{
// 				ExcludeFabricIfaces: common.NewStringSet("eth0", "eth1"),
// 			},
// 			expEnabled:       true,
// 			expIgnoredIfaces: common.NewStringSet("eth0", "eth1"),
// 		},
// 		"refresh interval": {
// 			cfg: &Config{
// 				AttachInfoRefresh: 5,
// 			},
// 			expEnabled:         true,
// 			expRefreshInterval: 5 * time.Minute,
// 		},
// 	} {
// 		t.Run(name, func(t *testing.T) {
// 			log, buf := logging.NewTestLogger(t.Name())
// 			defer test.ShowBufferOnFailure(t, buf)

// 			ic := NewInfoCache(test.Context(t), log, nil, tc.cfg)

// 			test.AssertEqual(t, tc.expEnabled, ic.IsAttachInfoCacheEnabled(), "")
// 			test.AssertEqual(t, tc.expEnabled, ic.IsFabricCacheEnabled(), "")

// 			test.AssertEqual(t, tc.expIgnoredIfaces, ic.ignoreIfaces, "")
// 			// test.AssertEqual(t, tc.expRefreshInterval, ic.refreshInterval, "")
// 		})
// 	}
// }

// func TestAgent_InfoCache_EnableAttachInfoCache(t *testing.T) {
// 	for name, tc := range map[string]struct {
// 		ic              *InfoCache
// 		refreshInterval time.Duration
// 		expEnabled      bool
// 	}{
// 		"nil": {},
// 		"disabled": {
// 			ic:         newTestInfoCache(t, nil, testInfoCacheParams{disableAttachInfoCache: true}),
// 			expEnabled: true,
// 		},
// 		"already enabled": {
// 			ic:         newTestInfoCache(t, nil, testInfoCacheParams{}),
// 			expEnabled: true,
// 		},
// 		"refresh interval": {
// 			ic:              newTestInfoCache(t, nil, testInfoCacheParams{disableAttachInfoCache: true}),
// 			refreshInterval: time.Minute,
// 			expEnabled:      true,
// 		},
// 	} {
// 		t.Run(name, func(t *testing.T) {
// 			tc.ic.EnableAttachInfoCache(tc.refreshInterval)

// 			test.AssertEqual(t, tc.expEnabled, tc.ic.IsAttachInfoCacheEnabled(), "")
// 		})
// 	}
// }

// func TestAgent_InfoCache_DisableAttachInfoCache(t *testing.T) {
// 	for name, tc := range map[string]struct {
// 		ic *InfoCache
// 	}{
// 		"nil": {},
// 		"already disabled": {
// 			ic: newTestInfoCache(t, nil, testInfoCacheParams{disableAttachInfoCache: true}),
// 		},
// 		"enabled": {
// 			ic: newTestInfoCache(t, nil, testInfoCacheParams{}),
// 		},
// 	} {
// 		t.Run(name, func(t *testing.T) {
// 			tc.ic.DisableAttachInfoCache()

// 			test.AssertFalse(t, tc.ic.IsAttachInfoCacheEnabled(), "")
// 		})
// 	}
// }

// func TestAgent_InfoCache_EnableFabricCache(t *testing.T) {
// 	for name, tc := range map[string]struct {
// 		ic         *InfoCache
// 		expEnabled bool
// 	}{
// 		"nil": {},
// 		"disabled": {
// 			ic:         newTestInfoCache(t, nil, testInfoCacheParams{disableFabricCache: true}),
// 			expEnabled: true,
// 		},
// 		"already enabled": {
// 			ic:         newTestInfoCache(t, nil, testInfoCacheParams{}),
// 			expEnabled: true,
// 		},
// 	} {
// 		t.Run(name, func(t *testing.T) {
// 			tc.ic.EnableFabricCache()

// 			test.AssertEqual(t, tc.expEnabled, tc.ic.IsFabricCacheEnabled(), "")
// 		})
// 	}
// }

// func TestAgent_InfoCache_DisableFabricCache(t *testing.T) {
// 	for name, tc := range map[string]struct {
// 		ic *InfoCache
// 	}{
// 		"nil": {},
// 		"already disabled": {
// 			ic: newTestInfoCache(t, nil, testInfoCacheParams{disableFabricCache: true}),
// 		},
// 		"enabled": {
// 			ic: newTestInfoCache(t, nil, testInfoCacheParams{}),
// 		},
// 	} {
// 		t.Run(name, func(t *testing.T) {
// 			tc.ic.DisableFabricCache()

// 			test.AssertFalse(t, tc.ic.IsFabricCacheEnabled(), "")
// 		})
// 	}
// }

// func TestAgent_InfoCache_AddProvider(t *testing.T) {
// 	for name, tc := range map[string]struct {
// 		ic           *InfoCache
// 		input        string
// 		expProviders common.StringSet
// 	}{
// 		"nil": {
// 			input: "something",
// 		},
// 		"empty": {
// 			ic:           &InfoCache{},
// 			input:        "something",
// 			expProviders: common.NewStringSet("something"),
// 		},
// 		"add": {
// 			ic: &InfoCache{
// 				providers: common.NewStringSet("something"),
// 			},
// 			input:        "something else",
// 			expProviders: common.NewStringSet("something", "something else"),
// 		},
// 		"ignore empty string": {
// 			ic:    &InfoCache{},
// 			input: "",
// 		},
// 	} {
// 		t.Run(name, func(t *testing.T) {
// 			log, buf := logging.NewTestLogger(t.Name())
// 			defer test.ShowBufferOnFailure(t, buf)

// 			if tc.ic != nil {
// 				tc.ic.log = log
// 			}

// 			tc.ic.AddProvider(tc.input)

// 			if tc.ic == nil {
// 				return
// 			}
// 			if diff := cmp.Diff(tc.expProviders, tc.ic.providers); diff != "" {
// 				t.Fatalf("want-, got+:\n%s", diff)
// 			}
// 		})
// 	}
// }

// func TestAgent_InfoCache_GetAttachInfo(t *testing.T) {
// 	ctlResp := &control.GetAttachInfoResp{
// 		System:       "dontcare",
// 		ServiceRanks: []*control.PrimaryServiceRank{{Rank: 1, Uri: "my uri"}},
// 		MSRanks:      []uint32{0, 1, 2, 3},
// 		ClientNetHint: control.ClientNetworkHint{
// 			Provider:    "ofi+tcp",
// 			NetDevClass: uint32(hardware.Ether),
// 		},
// 	}

// 	for name, tc := range map[string]struct {
// 		getInfoCache func(logging.Logger) *InfoCache
// 		remoteResp   *control.GetAttachInfoResp
// 		remoteErr    error
// 		expErr       error
// 		expResp      *control.GetAttachInfoResp
// 		expRemote    bool
// 		expCached    bool
// 	}{
// 		"nil": {
// 			expErr: errors.New("nil"),
// 		},
// 		"disabled": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				return newTestInfoCache(t, l, testInfoCacheParams{
// 					disableAttachInfoCache: true,
// 				})
// 			},
// 			remoteResp: ctlResp,
// 			expResp:    ctlResp,
// 			expRemote:  true,
// 		},
// 		"disabled fails fetch": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				return newTestInfoCache(t, l, testInfoCacheParams{
// 					disableAttachInfoCache: true,
// 				})
// 			},
// 			remoteErr: errors.New("mock remote"),
// 			expErr:    errors.New("mock remote"),
// 			expRemote: true,
// 		},
// 		"enabled but empty": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				return newTestInfoCache(t, l, testInfoCacheParams{})
// 			},
// 			remoteResp: ctlResp,
// 			expResp:    ctlResp,
// 			expRemote:  true,
// 			expCached:  true,
// 		},
// 		"enabled but empty fails fetch": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				return newTestInfoCache(t, l, testInfoCacheParams{})
// 			},
// 			remoteErr: errors.New("mock remote"),
// 			expErr:    errors.New("mock remote"),
// 			expRemote: true,
// 			expCached: true,
// 		},
// 		"enabled and cached": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				ic := newTestInfoCache(t, l, testInfoCacheParams{})
// 				ic.cache.Set(test.Context(t), newCachedAttachInfo(0, "dontcare", nil))
// 				return ic
// 			},
// 			remoteErr: errors.New("shouldn't call remote"),
// 			expResp:   ctlResp,
// 			expCached: true,
// 		},
// 	} {
// 		t.Run(name, func(t *testing.T) {
// 			log, buf := logging.NewTestLogger(t.Name())
// 			defer test.ShowBufferOnFailure(t, buf)

// 			var ic *InfoCache
// 			if tc.getInfoCache != nil {
// 				ic = tc.getInfoCache(log)
// 			}

// 			calledRemote := false
// 			if ic != nil {
// 				ic.getAttachInfo = func(_ context.Context, _ control.UnaryInvoker, _ *control.GetAttachInfoReq) (*control.GetAttachInfoResp, error) {
// 					calledRemote = true
// 					return tc.remoteResp, tc.remoteErr
// 				}
// 			}

// 			resp, err := ic.GetAttachInfo(test.Context(t), "dontcare")

// 			test.CmpErr(t, tc.expErr, err)
// 			if diff := cmp.Diff(tc.expResp, resp); diff != "" {
// 				t.Fatalf("want-, got+:\n%s", diff)
// 			}

// 			test.AssertEqual(t, tc.expRemote, calledRemote, "")

// 			if ic == nil {
// 				return
// 			}

// 			test.AssertEqual(t, tc.expCached, ic.cache.Has(attachInfoKey), "")
// 			if tc.expCached && tc.expResp != nil {
// 				cached, unlockItem, err := ic.cache.Get(test.Context(t), attachInfoKey)
// 				if err != nil {
// 					t.Fatal(err)
// 				}
// 				defer unlockItem()
// 				if diff := cmp.Diff(tc.expResp, cached); diff != "" {
// 					t.Fatalf("want-, got+:\n%s", diff)
// 				}
// 			}
// 		})
// 	}
// }

// func TestAgent_InfoCache_GetFabricDevice(t *testing.T) {
// 	testSet := hardware.NewFabricInterfaceSet(&hardware.FabricInterface{
// 		Name:          "dev0",
// 		NetInterfaces: common.NewStringSet("test0"),
// 		DeviceClass:   hardware.Ether,
// 		Providers:     hardware.NewFabricProviderSet(&hardware.FabricProvider{Name: "testprov"}),
// 	})
// 	for name, tc := range map[string]struct {
// 		getInfoCache    func(logging.Logger) *InfoCache
// 		devClass        hardware.NetDevClass
// 		provider        string
// 		fabricResp      *hardware.FabricInterfaceSet
// 		fabricErr       error
// 		expResult       *FabricInterface
// 		expErr          error
// 		expScan         bool
// 		expCachedFabric *hardware.FabricInterfaceSet
// 	}{
// 		"nil": {
// 			expErr: errors.New("nil"),
// 		},
// 		"disabled": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				return newTestInfoCache(t, l, testInfoCacheParams{
// 					disableFabricCache: true,
// 				})
// 			},
// 			devClass:   hardware.Ether,
// 			provider:   "testprov",
// 			fabricResp: testSet,
// 			expScan:    true,
// 			expResult: &FabricInterface{
// 				Name:        "test0",
// 				Domain:      "dev0",
// 				NetDevClass: hardware.Ether,
// 			},
// 		},
// 		"disabled fails fetch": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				return newTestInfoCache(t, l, testInfoCacheParams{
// 					disableFabricCache: true,
// 				})
// 			},
// 			devClass:  hardware.Ether,
// 			provider:  "testprov",
// 			fabricErr: errors.New("mock fabric scan"),
// 			expScan:   true,
// 			expErr:    errors.New("mock fabric scan"),
// 		},
// 		"enabled but empty": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				return newTestInfoCache(t, l, testInfoCacheParams{})
// 			},
// 			devClass:   hardware.Ether,
// 			provider:   "testprov",
// 			fabricResp: testSet,
// 			expScan:    true,
// 			expResult: &FabricInterface{
// 				Name:        "test0",
// 				Domain:      "dev0",
// 				NetDevClass: hardware.Ether,
// 			},
// 			expCachedFabric: testSet,
// 		},
// 		"enabled but empty fails fetch": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				return newTestInfoCache(t, l, testInfoCacheParams{})
// 			},
// 			devClass:  hardware.Ether,
// 			provider:  "testprov",
// 			fabricErr: errors.New("mock fabric scan"),
// 			expScan:   true,
// 			expErr:    errors.New("mock fabric scan"),
// 		},
// 		"enabled and cached": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				ic := newTestInfoCache(t, l, testInfoCacheParams{})
// 				nf := NUMAFabricFromScan(test.Context(t), l, testSet)
// 				nf.getAddrInterface = ic.getAddrInterface
// 				ic.cache.Set(test.Context(t), &cachedFabricInfo{
// 					lastResults: nf,
// 				})
// 				return ic
// 			},
// 			devClass:  hardware.Ether,
// 			provider:  "testprov",
// 			fabricErr: errors.New("shouldn't call scan"),
// 			expResult: &FabricInterface{
// 				Name:        "test0",
// 				Domain:      "dev0",
// 				NetDevClass: hardware.Ether,
// 			},
// 			expCachedFabric: hardware.NewFabricInterfaceSet(&hardware.FabricInterface{
// 				Name:          "dev0",
// 				NetInterfaces: common.NewStringSet("test0"),
// 				DeviceClass:   hardware.Ether,
// 				Providers:     hardware.NewFabricProviderSet(&hardware.FabricProvider{Name: "testprov"}),
// 			}),
// 		},
// 		"requested not found": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				ic := newTestInfoCache(t, l, testInfoCacheParams{})
// 				nf := NUMAFabricFromScan(test.Context(t), l, testSet)
// 				nf.getAddrInterface = ic.getAddrInterface
// 				ic.cache.Set(test.Context(t), &cachedFabricInfo{
// 					lastResults: nf,
// 				})
// 				return ic
// 			},
// 			devClass:  hardware.Ether,
// 			provider:  "bad",
// 			fabricErr: errors.New("shouldn't call scan"),
// 			expErr:    errors.New("no suitable fabric interface"),
// 			expCachedFabric: hardware.NewFabricInterfaceSet(&hardware.FabricInterface{
// 				Name:          "dev0",
// 				NetInterfaces: common.NewStringSet("test0"),
// 				DeviceClass:   hardware.Ether,
// 				Providers:     hardware.NewFabricProviderSet(&hardware.FabricProvider{Name: "testprov"}),
// 			}),
// 		},
// 	} {
// 		t.Run(name, func(t *testing.T) {
// 			log, buf := logging.NewTestLogger(t.Name())
// 			defer test.ShowBufferOnFailure(t, buf)

// 			var ic *InfoCache
// 			if tc.getInfoCache != nil {
// 				ic = tc.getInfoCache(log)
// 			}

// 			calledScan := false
// 			if ic != nil {
// 				ic.fabricScan = func(_ context.Context, _ ...string) (*NUMAFabric, error) {
// 					calledScan = true
// 					return NUMAFabricFromScan(test.Context(t), log, tc.fabricResp), tc.fabricErr
// 				}
// 			}

// 			result, err := ic.GetFabricDevice(test.Context(t), 0, tc.devClass, tc.provider)

// 			test.CmpErr(t, tc.expErr, err)
// 			if diff := cmp.Diff(tc.expResult, result, cmpopts.IgnoreUnexported(FabricInterface{})); diff != "" {
// 				t.Fatalf("want-, got+:\n%s", diff)
// 			}

// 			test.AssertEqual(t, tc.expScan, calledScan, "")

// 			if ic == nil {
// 				return
// 			}

// 			if tc.expCachedFabric != nil {
// 				data, unlock, err := ic.cache.Get(test.Context(t), fabricKey)
// 				if err != nil {
// 					t.Fatal(err)
// 				}
// 				defer unlock()

// 				cached, ok := data.(*cachedFabricInfo)
// 				test.AssertTrue(t, ok, "bad cached data type")

// 				expNF := NUMAFabricFromScan(test.Context(t), log, tc.expCachedFabric)
// 				if diff := cmp.Diff(expNF.numaMap, cached.lastResults.numaMap, cmpopts.IgnoreUnexported(FabricInterface{})); diff != "" {
// 					t.Fatalf("want-, got+:\n%s", diff)
// 				}
// 			}
// 		})
// 	}
// }

// func TestAgent_InfoCache_Refresh(t *testing.T) {
// 	ctlResp := &control.GetAttachInfoResp{
// 		System:       "dontcare",
// 		ServiceRanks: []*control.PrimaryServiceRank{{Rank: 1, Uri: "my uri"}},
// 		MSRanks:      []uint32{0, 1, 2, 3},
// 		ClientNetHint: control.ClientNetworkHint{
// 			Provider:    "ofi+tcp",
// 			NetDevClass: uint32(hardware.Ether),
// 		},
// 	}

// 	testSet := hardware.NewFabricInterfaceSet(&hardware.FabricInterface{
// 		Name:          "dev0",
// 		NetInterfaces: common.NewStringSet("test0"),
// 		DeviceClass:   hardware.Ether,
// 		Providers:     hardware.NewFabricProviderSet(&hardware.FabricProvider{Name: "testprov"}),
// 	})

// 	for name, tc := range map[string]struct {
// 		getInfoCache        func(logging.Logger) *InfoCache
// 		attachInfoResp      *control.GetAttachInfoResp
// 		attachInfoErr       error
// 		fabricResp          *hardware.FabricInterfaceSet
// 		fabricErr           error
// 		expErr              error
// 		expScan             bool
// 		expRemote           bool
// 		expCachedFabric     *hardware.FabricInterfaceSet
// 		expCachedAttachInfo *control.GetAttachInfoResp
// 	}{
// 		"nil": {
// 			expErr: errors.New("nil"),
// 		},
// 		"both disabled": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				return newTestInfoCache(t, l, testInfoCacheParams{
// 					disableFabricCache:     true,
// 					disableAttachInfoCache: true,
// 				})
// 			},
// 		},
// 		"both enabled": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				return newTestInfoCache(t, l, testInfoCacheParams{})
// 			},
// 			attachInfoResp:      ctlResp,
// 			fabricResp:          testSet,
// 			expScan:             true,
// 			expRemote:           true,
// 			expCachedFabric:     testSet,
// 			expCachedAttachInfo: ctlResp,
// 		},
// 		"fabric disabled": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				return newTestInfoCache(t, l, testInfoCacheParams{
// 					disableFabricCache: true,
// 				})
// 			},
// 			attachInfoResp:      ctlResp,
// 			fabricErr:           errors.New("should not call scan"),
// 			expRemote:           true,
// 			expCachedAttachInfo: ctlResp,
// 		},
// 		"attach info disabled": {
// 			getInfoCache: func(l logging.Logger) *InfoCache {
// 				return newTestInfoCache(t, l, testInfoCacheParams{
// 					disableAttachInfoCache: true,
// 				})
// 			},
// 			attachInfoErr:   errors.New("should not call remote"),
// 			fabricResp:      testSet,
// 			expScan:         true,
// 			expCachedFabric: testSet,
// 		},
// 	} {
// 		t.Run(name, func(t *testing.T) {
// 			log, buf := logging.NewTestLogger(t.Name())
// 			defer test.ShowBufferOnFailure(t, buf)

// 			var ic *InfoCache
// 			if tc.getInfoCache != nil {
// 				ic = tc.getInfoCache(log)
// 			}

// 			calledScan := false
// 			calledRemote := false
// 			if ic != nil {
// 				ic.fabricScan = func(_ context.Context, _ ...string) (*NUMAFabric, error) {
// 					calledScan = true
// 					return NUMAFabricFromScan(test.Context(t), log, tc.fabricResp), tc.fabricErr
// 				}
// 				ic.getAttachInfo = func(_ context.Context, _ control.UnaryInvoker, _ *control.GetAttachInfoReq) (*control.GetAttachInfoResp, error) {
// 					calledRemote = true
// 					return tc.attachInfoResp, tc.attachInfoErr
// 				}
// 			}

// 			err := ic.Refresh(test.Context(t))

// 			test.CmpErr(t, tc.expErr, err)
// 			test.AssertEqual(t, tc.expScan, calledScan, "")
// 			test.AssertEqual(t, tc.expRemote, calledRemote, "")

// 			if tc.expCachedFabric != nil {
// 				data, unlock, err := ic.cache.Get(test.Context(t), fabricKey)
// 				if err != nil {
// 					t.Fatal(err)
// 				}
// 				defer unlock()

// 				cached, ok := data.(*cachedFabricInfo)
// 				test.AssertTrue(t, ok, "bad cached data type")

// 				expNF := NUMAFabricFromScan(test.Context(t), log, tc.expCachedFabric)
// 				if diff := cmp.Diff(expNF.numaMap, cached.lastResults.numaMap, cmpopts.IgnoreUnexported(FabricInterface{})); diff != "" {
// 					t.Fatalf("want-, got+:\n%s", diff)
// 				}
// 			}

// 			if tc.expCachedAttachInfo != nil {
// 				data, unlock, err := ic.cache.Get(test.Context(t), attachInfoKey)
// 				if err != nil {
// 					t.Fatal(err)
// 				}
// 				defer unlock()

// 				cached, ok := data.(*cachedAttachInfo)
// 				test.AssertTrue(t, ok, "bad cached data type")

// 				if diff := cmp.Diff(tc.expCachedAttachInfo, cached.lastResponse); diff != "" {
// 					t.Fatalf("want-, got+:\n%s", diff)
// 				}
// 			}
// 		})
// 	}
// }
