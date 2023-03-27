/*
 * Copyright 2019-2020 by Nedim Sabic Sabic
 * https://www.fibratus.io
 * All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package types

import (
	"encoding/binary"
	"fmt"
	htypes "github.com/rabbitstack/fibratus/pkg/handle/types"
	"github.com/rabbitstack/fibratus/pkg/kcap/section"
	"github.com/rabbitstack/fibratus/pkg/kevent/kparams"
	"github.com/rabbitstack/fibratus/pkg/pe"
	hndl "github.com/rabbitstack/fibratus/pkg/syscall/handle"
	"github.com/rabbitstack/fibratus/pkg/syscall/process"
	"github.com/rabbitstack/fibratus/pkg/util/bootid"
	"github.com/rabbitstack/fibratus/pkg/util/cmdline"
	"golang.org/x/sys/windows"
	"path/filepath"
	"sync"
	"time"
)

// PS encapsulates process' state such as allocated resources and other metadata.
type PS struct {
	mu sync.RWMutex
	// PID is the identifier of this process. This value is valid from the time a process is created until it is terminated.
	PID uint32 `json:"pid"`
	// Ppipd represents the parent of this process. Process identifier numbers are reused, so they only identify a process
	// for the lifetime of that process. It is possible that the process identified by `Ppid` is terminated,
	// so `Ppid` may not refer to a running process. It is also possible that `Ppid` incorrectly refers
	// to a process that reuses a process identifier.
	Ppid uint32 `json:"ppid"`
	// Name is the process' image name including file extension (e.g. cmd.exe)
	Name string `json:"name"`
	// Comm is the full process' command line (e.g. C:\Windows\system32\cmd.exe /cdir /-C /W)
	Comm string `json:"comm"`
	// Exe is the full name of the process' executable (e.g. C:\Windows\system32\cmd.exe)
	Exe string `json:"exe"`
	// Cwd designates the current working directory of the process.
	Cwd string `json:"cwd"`
	// SID is the security identifier under which this process is run.
	SID string `json:"sid"`
	// Args contains process' command line arguments (e.g. /cdir, /-C, /W)
	Args []string `json:"args"`
	// SessionID is the unique identifier for the current session.
	SessionID uint8 `json:"session"`
	// Envs contains process' environment variables indexed by env variable name.
	Envs map[string]string `json:"envs"`
	// Threads contains all the threads running in the address space of this process.
	Threads map[uint32]Thread `json:"-"`
	// Modules contains all the modules loaded by the process.
	Modules []Module `json:"modules"`
	// Handles represents the collection of handles allocated by the process.
	Handles htypes.Handles `json:"handles"`
	// PE stores the PE (Portable Executable) metadata.
	PE *pe.PE `json:"pe"`
	// Parent represents the reference to the parent process.
	Parent *PS `json:"parent"`
	// StartTime represents the process start time.
	StartTime time.Time `json:"started"`
	// uuid is a unique process identifier derived from boot ID and process sequence number
	uuid uint64
}

// UUID is meant to offer a more robust version of process ID that
// is resistant to being repeated. Process start key was introduced
// in Windows 10 1507 and is derived from _KUSER_SHARED_DATA.BootId and
// EPROCESS.SequenceNumber both of which increment and are unlikely to
// overflow. This method uses a combination of process start key and boot id
// to fabric a unique process identifier. If this is not possible, the uuid
// is computed by using the process start time.
func (ps *PS) UUID() uint64 {
	if ps.uuid != 0 {
		return ps.uuid
	}
	// assume the uuid is derived from boot ID and process start time
	ps.uuid = (bootid.Read() << 30) + uint64(ps.PID) | uint64(ps.StartTime.UnixNano())
	maj, _, patch := windows.RtlGetNtVersionNumbers()
	if maj >= 10 && patch >= 1507 {
		seqNum := querySequenceNumber(ps.PID)
		// prefer the most robust variant of the uuid which uses the
		// process sequence number obtained from the process object
		if seqNum != 0 {
			ps.uuid = (bootid.Read() << 30) | seqNum
		}
	}
	return ps.uuid
}

func querySequenceNumber(pid uint32) uint64 {
	proc, err := process.Open(process.QueryInformation, false, pid)
	if err != nil {
		return 0
	}
	defer proc.Close()
	buf := make([]byte, 8)
	_, err = process.QueryInfo(proc, process.SequenceNumberInformationClass, buf)
	if err != nil {
		return 0
	}
	return binary.BigEndian.Uint64(buf)
}

// String returns a string representation of the process' state.
func (ps *PS) String() string {
	return fmt.Sprintf(`
		Pid:  %d
		Ppid: %d
		Name: %s
		Cmdline: %s
		Exe:  %s
		Cwd:  %s
		SID:  %s
		Args: %s
		Session ID: %d
		Envs: %s
		`,
		ps.PID,
		ps.Ppid,
		ps.Name,
		ps.Comm,
		ps.Exe,
		ps.Cwd,
		ps.SID,
		ps.Args,
		ps.SessionID,
		ps.Envs,
	)
}

// Ancestors returns all ancestors of this process. The string slice contains
// the process image name followed by the process id.
func (ps *PS) Ancestors() []string {
	ancestors := make([]string, 0)
	walk := func(proc *PS) {
		ancestors = append(ancestors, fmt.Sprintf("%s (%d)", proc.Name, proc.PID))
	}
	Walk(walk, ps)
	return ancestors
}

// Thread stores metadata about a thread that's executing in process's address space.
type Thread struct {
	// Tid is the unique identifier of thread inside the process.
	Tid uint32
	// Pid is the identifier of the process to which this thread pertains.
	Pid uint32
	// IOPrio represents an I/O priority hint for scheduling I/O operations generated by the thread.
	IOPrio uint8
	// BasePrio is the scheduler priority of the thread.
	BasePrio uint8
	// PagePrio is a memory page priority hint for memory pages accessed by the thread.
	PagePrio uint8
	// UstackBase is the base address of the thread's user space stack.
	UstackBase kparams.Hex
	// UstackLimit is the limit of the thread's user space stack.
	UstackLimit kparams.Hex
	// KStackBase is the base address of the thread's kernel space stack.
	KstackBase kparams.Hex
	// KstackLimit is the limit of the thread's kernel space stack.
	KstackLimit kparams.Hex
	// Entrypoint is the starting address of the function to be executed by the thread.
	Entrypoint kparams.Hex
}

// String returns the thread as a human-readable string.
func (t Thread) String() string {
	return fmt.Sprintf("ID: %d IO prio: %d, Base prio: %d, Page prio: %d, Ustack base: %s, Ustack limit: %s, Kstack base: %s, Kstack limit: %s, Entrypoint: %s", t.Tid, t.IOPrio, t.BasePrio, t.PagePrio, t.UstackBase, t.UstackLimit, t.KstackBase, t.UstackLimit, t.Entrypoint)
}

// Module represents the data for all dynamic libraries/executables that reside in the process' address space.
type Module struct {
	// Size designates the size in bytes of the image file.
	Size uint32
	// Checksum is the checksum of the image file.
	Checksum uint32
	// Name represents the full path of this image.
	Name string
	// BaseAddress is the base address of process in which the image is loaded.
	BaseAddress kparams.Hex
	// DefaultBaseAddress is the default base address.
	DefaultBaseAddress kparams.Hex
}

// String returns the string representation of the module.
func (m Module) String() string {
	return fmt.Sprintf("Name: %s, Size: %d, Checksum: %d, Base address: %s, Default base address: %s", m.Name, m.Size, m.Checksum, m.BaseAddress, m.DefaultBaseAddress)
}

// FromKevent produces a new process state from kernel event.
func FromKevent(pid, ppid uint32, name, comm, exe, sid string, sessionID uint8) *PS {
	return &PS{
		PID:       pid,
		Ppid:      ppid,
		Name:      name,
		Comm:      comm,
		Exe:       exe,
		Args:      cmdline.Split(comm),
		SID:       sid,
		SessionID: sessionID,
		Threads:   make(map[uint32]Thread),
		Modules:   make([]Module, 0),
		Handles:   make([]htypes.Handle, 0),
	}
}

// ThreadFromKevent builds a thread info from kernel event.
func ThreadFromKevent(pid, tid uint32, ustackBase, ustackLimit, kstackBase, kstackLimit kparams.Hex, ioPrio, basePrio, pagePrio uint8, entrypoint kparams.Hex) Thread {
	return Thread{
		Pid:         pid,
		Tid:         tid,
		UstackBase:  ustackBase,
		UstackLimit: ustackLimit,
		KstackBase:  kstackBase,
		KstackLimit: kstackLimit,
		IOPrio:      ioPrio,
		BasePrio:    basePrio,
		PagePrio:    pagePrio,
		Entrypoint:  entrypoint,
	}
}

// ImageFromKevent constructs a module info from the corresponding kernel event.
func ImageFromKevent(size, checksum uint32, name string, baseAddress, defaultBaseAddress kparams.Hex) Module {
	return Module{
		Size:               size,
		Checksum:           checksum,
		Name:               name,
		BaseAddress:        baseAddress,
		DefaultBaseAddress: defaultBaseAddress,
	}
}

// NewPS produces a new process state from passed arguments.
func NewPS(pid, ppid uint32, exe, cwd, comm string, thread Thread, envs map[string]string) *PS {
	return &PS{
		PID:     pid,
		Ppid:    ppid,
		Name:    filepath.Base(exe),
		Exe:     exe,
		Comm:    comm,
		Cwd:     cwd,
		Args:    cmdline.Split(comm),
		Threads: map[uint32]Thread{thread.Tid: thread},
		Modules: make([]Module, 0),
		Handles: make([]htypes.Handle, 0),
		Envs:    envs,
	}
}

// NewFromKcap reconstructs the state of the process from kcap file.
func NewFromKcap(buf []byte, sec section.Section) (*PS, error) {
	ps := PS{
		Args:    make([]string, 0),
		Envs:    make(map[string]string),
		Handles: make([]htypes.Handle, 0),
		Modules: make([]Module, 0),
		Threads: make(map[uint32]Thread),
	}
	if err := ps.Unmarshal(buf, sec); err != nil {
		return nil, err
	}
	return &ps, nil
}

// AddThread adds a thread to process's state descriptor.
func (ps *PS) AddThread(thread Thread) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	ps.Threads[thread.Tid] = thread
}

// RemoveThread eliminates a thread from the process's state.
func (ps *PS) RemoveThread(tid uint32) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	delete(ps.Threads, tid)
}

// RLock acquires a read mutex on the process state.
func (ps *PS) RLock() {
	ps.mu.RLock()
}

// RUnlock releases a read mutex on the process sate.
func (ps *PS) RUnlock() {
	ps.mu.RUnlock()
}

// AddHandle adds a new handle to this process state.
func (ps *PS) AddHandle(handle htypes.Handle) {
	ps.Handles = append(ps.Handles, handle)
}

// RemoveHandle removes a handle with specified identifier from the list of allocated handles.
func (ps *PS) RemoveHandle(num hndl.Handle) {
	for i, h := range ps.Handles {
		if h.Num == num {
			ps.Handles = append(ps.Handles[:i], ps.Handles[i+1:]...)
			break
		}
	}
}

// AddModule adds a new module to this process state.
func (ps *PS) AddModule(mod Module) {
	m := ps.FindModule(mod.Name)
	if m != nil {
		return
	}
	ps.Modules = append(ps.Modules, mod)
}

// RemoveModule removes a module with specified full-path from this process state.
func (ps *PS) RemoveModule(name string) {
	for i, mod := range ps.Modules {
		if mod.Name == name {
			ps.Modules = append(ps.Modules[:i], ps.Modules[i+1:]...)
			break
		}
	}
}

// FindModule finds the module by name.
func (ps *PS) FindModule(name string) *Module {
	for _, mod := range ps.Modules {
		if filepath.Base(mod.Name) == name {
			return &mod
		}
	}
	return nil
}
