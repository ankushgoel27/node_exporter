// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !nomeminfo
// +build !nomeminfo

package collector

// #include <mach/mach_host.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type swapUsage struct {
	Total, Avail, Used uint64
}

// generic Sysctl buffer unmarshalling
func sysctlbyname(name string, data interface{}) (err error) {
	val, err := syscall.Sysctl(name)
	if err != nil {
		return err
	}

	buf := []byte(val)

	switch v := data.(type) {
	case *uint64:
		*v = *(*uint64)(unsafe.Pointer(&buf[0]))
		return
	}

	bbuf := bytes.NewBuffer([]byte(val))
	return binary.Read(bbuf, binary.LittleEndian, data)
}

func (c *meminfoCollector) getMemInfo() (map[string]float64, error) {
	infoCount := C.mach_msg_type_number_t(C.HOST_VM_INFO64_COUNT)
	vmstat := C.vm_statistics64_data_t{}
	ret := C.host_statistics64(
		C.host_t(C.mach_host_self()),
		C.HOST_VM_INFO64,
		C.host_info64_t(unsafe.Pointer(&vmstat)),
		&infoCount,
	)
	if ret != C.KERN_SUCCESS {
		return nil, fmt.Errorf("Couldn't get memory statistics, host_statistics returned %d", ret)
	}
	totalb, err := unix.Sysctl("hw.memsize")
	if err != nil {
		return nil, err
	}
	// Syscall removes terminating NUL which we need to cast to uint64
	total := binary.LittleEndian.Uint64([]byte(totalb + "\x00"))

	swUsage := swapUsage{}
	if err := sysctlbyname("vm.swapusage", &swUsage); err != nil {
		return nil, fmt.Errorf("Couldn't get memory swap usage. error %v", err)
	}

	ps := float64(C.natural_t(syscall.Getpagesize()))
	outmap := map[string]float64{
		"active_bytes":            ps * float64(vmstat.active_count),
		"inactive_bytes":          ps * float64(vmstat.inactive_count),
		"wired_bytes":             ps * float64(vmstat.wire_count),
		"free_bytes":              ps * float64(vmstat.free_count),
		"swapped_in_bytes_total":  ps * float64(vmstat.pageins),
		"swapped_out_bytes_total": ps * float64(vmstat.pageouts),
		"total_bytes":             float64(total),
		"MemFree_bytes":           ps * float64(vmstat.free_count),
		"SwapCached_bytes":        float64(swUsage.Used),
		"SwapFree_bytes":          float64(swUsage.Avail),
		"MemTotal_bytes":          float64(total),
		"MemAvailable_bytes":      float64(total) - ps*(float64(vmstat.active_count)+float64(vmstat.inactive_count)+float64(vmstat.wire_count)),
		"Cached_bytes":            ps * float64(vmstat.compressor_page_count),
		"Buffers_bytes":           ps * float64(0),
		"Slab_bytes":              ps * float64(0),
		"PageTables_bytes":        ps * float64(0),
		"SwapTotal_bytes":         float64(swUsage.Total),
	}
	//for k, v := range outmap {
	//	i := int64(v)
	//	fmt.Printf("%s = %d MB\n", k, i/(1024*1024))
	//}
	//fmt.Printf("\n")
	return outmap, nil
}
