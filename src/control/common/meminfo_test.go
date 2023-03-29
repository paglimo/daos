//
// (C) Copyright 2019-2023 Intel Corporation.
//
// SPDX-License-Identifier: BSD-2-Clause-Patent
//

package common

import (
	"strings"
	"testing"

	"github.com/dustin/go-humanize"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"

	. "github.com/daos-stack/daos/src/control/common/test"
	"github.com/daos-stack/daos/src/control/logging"
)

func TestCommon_getMemInfo(t *testing.T) {
	// Just a simple test to verify that we get something -- it should
	// pretty much never error.
	_, err := GetMemInfo()
	if err != nil {
		t.Fatal(err)
	}
}

func TestCommon_parseMemInfo(t *testing.T) {
	for name, tc := range map[string]struct {
		input     string
		expOut    *MemInfo
		expFreeMB int
		expErr    error
	}{
		"none parsed": {
			expOut:    &MemInfo{},
			expFreeMB: 0,
		},
		"2MB pagesize": {
			input: `
MemTotal:           1024 kB
MemFree:            1024 kB
MemAvailable:       1024 kB
HugePages_Total:    1024
HugePages_Free:     1023
HugePages_Rsvd:        0
HugePages_Surp:        0
Hugepagesize:       2048 kB
			`,
			expOut: &MemInfo{
				HugepagesTotal:  1024,
				HugepagesFree:   1023,
				HugepageSizeKiB: 2048,
				MemTotalKiB:     1024,
				MemFreeKiB:      1024,
				MemAvailableKiB: 1024,
			},
			expFreeMB: 2046,
		},
		"1GB pagesize": {
			input: `
HugePages_Total:      16
HugePages_Free:       16
HugePages_Rsvd:        0
HugePages_Surp:        0
Hugepagesize:       1048576 kB
			`,
			expOut: &MemInfo{
				HugepagesTotal:  16,
				HugepagesFree:   16,
				HugepageSizeKiB: 1048576,
			},
			expFreeMB: 16384,
		},
		"weird pagesize": {
			input: `
Hugepagesize:       blerble 1 GB
			`,
			expErr: errors.New("unable to parse"),
		},
		"weird pagesize unit": {
			input: `
Hugepagesize:       1 GB
			`,
			expErr: errors.New("unhandled size unit \"GB\""),
		},
	} {
		t.Run(name, func(t *testing.T) {
			rdr := strings.NewReader(tc.input)

			gotOut, gotErr := parseMemInfo(rdr)
			CmpErr(t, tc.expErr, gotErr)
			if tc.expErr != nil {
				return
			}

			if diff := cmp.Diff(tc.expOut, gotOut); diff != "" {
				t.Fatalf("unexpected output (-want, +got)\n%s\n", diff)
			}

			if gotOut.HugepagesFreeMB() != tc.expFreeMB {
				t.Fatalf("expected FreeMB() to be %d, got %d",
					tc.expFreeMB, gotOut.HugepagesFreeMB())
			}
		})
	}
}

func TestCommon_CalcMinHugepages(t *testing.T) {
	for name, tc := range map[string]struct {
		input      *MemInfo
		numTargets int
		expPages   int
		expErr     error
	}{
		"no pages": {
			input:      &MemInfo{},
			numTargets: 1,
			expErr:     errors.New("invalid system hugepage size"),
		},
		"no targets": {
			input: &MemInfo{
				HugepageSizeKiB: 2048,
			},
			expErr: errors.New("numTargets"),
		},
		"2KB pagesize; 16 targets": {
			input: &MemInfo{
				HugepageSizeKiB: 2048,
			},
			numTargets: 16,
			expPages:   8192,
		},
		"2KB pagesize; 31 targets": {
			input: &MemInfo{
				HugepageSizeKiB: 2048,
			},
			numTargets: 31,
			expPages:   15872,
		},
		"1GB pagesize; 16 targets": {
			input: &MemInfo{
				HugepageSizeKiB: 1048576,
			},
			numTargets: 16,
			expPages:   16,
		},
	} {
		t.Run(name, func(t *testing.T) {
			gotPages, gotErr := CalcMinHugepages(tc.input.HugepageSizeKiB, tc.numTargets)
			CmpErr(t, tc.expErr, gotErr)
			if tc.expErr != nil {
				return
			}

			if gotPages != tc.expPages {
				t.Fatalf("expected %d, got %d", tc.expPages, gotPages)
			}
		})
	}
}

func TestCommon_CalcScmSize(t *testing.T) {
	for name, tc := range map[string]struct {
		memTotal uint64
		memHuge  uint64
		rsvSys   uint64
		rsvEng   uint64
		engCount int
		expSize  uint64
		expErr   error
	}{
		"no mem": {
			expErr: errors.New("requires nonzero total mem"),
		},
		"no engines": {
			memTotal: humanize.GiByte,
			expErr:   errors.New("requires nonzero nr engines"),
		},
		"default values; low mem": {
			memTotal: humanize.GiByte * 18,
			memHuge:  humanize.GiByte * 12,
			engCount: 1,
			expErr:   errors.New("insufficient ram"),
		},
		"default values; high mem": {
			memTotal: humanize.GiByte * 23,
			memHuge:  humanize.GiByte * 12,
			engCount: 1,
			expSize:  humanize.GiByte * 4,
		},
		"custom values; low sys reservation": {
			rsvSys:   humanize.GiByte * 4,
			memTotal: humanize.GiByte * 18,
			memHuge:  humanize.GiByte * 12,
			engCount: 2,
		},
		"custom values; high eng reservation": {
			rsvEng:   humanize.GiByte * 3,
			memTotal: humanize.GiByte * 23,
			memHuge:  humanize.GiByte * 12,
			engCount: 2,
			expErr:   errors.New("insufficient ram"),
		},
	} {
		t.Run(name, func(t *testing.T) {
			log, buf := logging.NewTestLogger(name)
			defer ShowBufferOnFailure(t, buf)

			gotSize, gotErr := CalcScmSize(log, tc.memTotal, tc.memHuge, tc.rsvSys,
				tc.rsvEng, tc.engCount)
			CmpErr(t, tc.expErr, gotErr)
			if tc.expErr != nil {
				return
			}

			if gotSize != tc.expSize {
				t.Fatalf("expected %d, got %d", tc.expSize, gotSize)
			}
		})
	}
}
