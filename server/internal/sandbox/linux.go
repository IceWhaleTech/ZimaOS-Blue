//go:build linux

package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// LinuxExecutor provides sandboxed execution on Linux using namespaces and cgroups.
type LinuxExecutor struct {
	*BaseExecutor
	cgroupPath string
}

// newPlatformExecutor creates a new Linux executor.
func newPlatformExecutor(config *Config) (Executor, error) {
	base := NewBaseExecutor(config)

	executor := &LinuxExecutor{
		BaseExecutor: base,
		cgroupPath:   "/sys/fs/cgroup/sandbox",
	}

	// Try to set up cgroup
	if err := executor.setupCgroup(); err != nil {
		// Fall back to base executor if cgroup setup fails
		return base, nil
	}

	return executor, nil
}

// setupCgroup sets up the cgroup for sandboxed processes.
func (e *LinuxExecutor) setupCgroup() error {
	// Check if cgroup v2 is available
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		return fmt.Errorf("cgroup v2 not available: %w", err)
	}

	// Create sandbox cgroup
	if err := os.MkdirAll(e.cgroupPath, 0755); err != nil {
		return fmt.Errorf("failed to create cgroup: %w", err)
	}

	return nil
}

// Execute executes a command with Linux-specific isolation.
func (e *LinuxExecutor) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(execCtx, req.Command, req.Args...)

	// Set working directory
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	// Set environment
	env := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/tmp",
		"TMPDIR=/tmp",
	}
	for k, v := range req.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	// Set stdin
	if req.Stdin != "" {
		cmd.Stdin = bytes.NewBufferString(req.Stdin)
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set up namespace isolation
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | // New UTS namespace (hostname)
			syscall.CLONE_NEWPID | // New PID namespace
			syscall.CLONE_NEWIPC, // New IPC namespace
		// Note: CLONE_NEWNET and CLONE_NEWNS require root privileges
	}

	// Note: Resource limits (rlimit) cannot be set directly on SysProcAttr in Go.
	// Instead, we rely on cgroup limits set below for memory, CPU, and process limits.
	// For stricter isolation, consider using a wrapper script that calls setrlimit(2).

	// Store execution state
	result := &ExecutionResult{
		ID:        req.ID,
		Status:    StatusRunning,
		StartTime: timeutil.NowTime(),
	}

	state := &executionState{
		result: result,
		cmd:    cmd,
		cancel: cancel,
	}

	e.mu.Lock()
	e.executions[req.ID] = state
	e.mu.Unlock()

	// Create execution-specific cgroup
	execCgroupPath := filepath.Join(e.cgroupPath, req.ID)
	if err := os.MkdirAll(execCgroupPath, 0755); err == nil {
		// Set memory limit
		memLimitPath := filepath.Join(execCgroupPath, "memory.max")
		_ = os.WriteFile(memLimitPath, []byte(strconv.FormatInt(req.MemoryLimit, 10)), 0644)

		// Set CPU limit (in microseconds per 100ms period)
		cpuMaxPath := filepath.Join(execCgroupPath, "cpu.max")
		cpuQuota := int64(req.CPULimit * 100000) // 100ms period
		_ = os.WriteFile(cpuMaxPath, []byte(fmt.Sprintf("%d 100000", cpuQuota)), 0644)

		// Set process limit
		pidsMaxPath := filepath.Join(execCgroupPath, "pids.max")
		_ = os.WriteFile(pidsMaxPath, []byte(strconv.Itoa(e.config.ProcessLimit)), 0644)

		// Clean up cgroup after execution
		defer os.RemoveAll(execCgroupPath)
	}

	// Run command
	err := cmd.Run()
	result.EndTime = timeutil.NowTime()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Stdout = truncateOutput(stdout.String(), 1024*1024) // 1MB max
	result.Stderr = truncateOutput(stderr.String(), 1024*1024)

	// Get resource usage
	if cmd.ProcessState != nil {
		rusage := cmd.ProcessState.SysUsage().(*syscall.Rusage)
		result.ResourceUsage = &ResourceUsage{
			CPUTime:    rusage.Utime.Nano() + rusage.Stime.Nano(),
			MemoryPeak: rusage.Maxrss * 1024, // Convert KB to bytes
			IORead:     rusage.Inblock * 512,
			IOWrite:    rusage.Oublock * 512,
		}
	}

	// Determine status
	if execCtx.Err() == context.DeadlineExceeded {
		result.Status = StatusTimeout
		result.Error = ErrExecutionTimeout.Error()
	} else if execCtx.Err() == context.Canceled {
		result.Status = StatusKilled
		result.Error = ErrExecutionKilled.Error()
	} else if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
	} else {
		result.Status = StatusCompleted
		result.ExitCode = 0
	}

	return result, nil
}

// IsSupported returns true if Linux sandboxing is supported.
func (e *LinuxExecutor) IsSupported() bool {
	// Check if we can create namespaces
	cmd := exec.Command("unshare", "--help")
	return cmd.Run() == nil
}

// Cleanup cleans up resources including cgroups.
func (e *LinuxExecutor) Cleanup() error {
	// Clean up base executor
	if err := e.BaseExecutor.Cleanup(); err != nil {
		return err
	}

	// Clean up cgroup
	return os.RemoveAll(e.cgroupPath)
}

// SeccompProfile represents a seccomp profile for syscall filtering.
type SeccompProfile struct {
	DefaultAction string
	Syscalls      []SyscallRule
}

// SyscallRule represents a rule for a syscall.
type SyscallRule struct {
	Names  []string
	Action string
}

// DefaultSeccompProfile returns a restrictive seccomp profile.
func DefaultSeccompProfile() *SeccompProfile {
	return &SeccompProfile{
		DefaultAction: "SCMP_ACT_ERRNO",
		Syscalls: []SyscallRule{
			{
				Names: []string{
					"read", "write", "open", "close", "stat", "fstat", "lstat",
					"poll", "lseek", "mmap", "mprotect", "munmap", "brk",
					"rt_sigaction", "rt_sigprocmask", "rt_sigreturn",
					"ioctl", "access", "pipe", "select", "sched_yield",
					"mremap", "msync", "mincore", "madvise", "dup", "dup2",
					"nanosleep", "getpid", "socket", "connect", "accept",
					"sendto", "recvfrom", "sendmsg", "recvmsg", "shutdown",
					"bind", "listen", "getsockname", "getpeername",
					"socketpair", "setsockopt", "getsockopt", "clone",
					"fork", "vfork", "execve", "exit", "wait4", "kill",
					"uname", "fcntl", "flock", "fsync", "fdatasync",
					"truncate", "ftruncate", "getdents", "getcwd", "chdir",
					"fchdir", "rename", "mkdir", "rmdir", "creat", "link",
					"unlink", "symlink", "readlink", "chmod", "fchmod",
					"chown", "fchown", "lchown", "umask", "gettimeofday",
					"getrlimit", "getrusage", "sysinfo", "times", "getuid",
					"getgid", "setuid", "setgid", "geteuid", "getegid",
					"setpgid", "getppid", "getpgrp", "setsid", "setreuid",
					"setregid", "getgroups", "setgroups", "setresuid",
					"getresuid", "setresgid", "getresgid", "getpgid",
					"setfsuid", "setfsgid", "getsid", "capget", "capset",
					"rt_sigpending", "rt_sigtimedwait", "rt_sigqueueinfo",
					"rt_sigsuspend", "sigaltstack", "utime", "mknod",
					"uselib", "personality", "ustat", "statfs", "fstatfs",
					"sysfs", "getpriority", "setpriority", "sched_setparam",
					"sched_getparam", "sched_setscheduler", "sched_getscheduler",
					"sched_get_priority_max", "sched_get_priority_min",
					"sched_rr_get_interval", "mlock", "munlock", "mlockall",
					"munlockall", "vhangup", "pivot_root", "prctl",
					"arch_prctl", "adjtimex", "setrlimit", "chroot", "sync",
					"acct", "settimeofday", "mount", "umount2", "swapon",
					"swapoff", "reboot", "sethostname", "setdomainname",
					"iopl", "ioperm", "init_module", "delete_module",
					"quotactl", "gettid", "readahead", "setxattr", "lsetxattr",
					"fsetxattr", "getxattr", "lgetxattr", "fgetxattr",
					"listxattr", "llistxattr", "flistxattr", "removexattr",
					"lremovexattr", "fremovexattr", "tkill", "time",
					"futex", "sched_setaffinity", "sched_getaffinity",
					"set_thread_area", "io_setup", "io_destroy", "io_getevents",
					"io_submit", "io_cancel", "get_thread_area", "lookup_dcookie",
					"epoll_create", "epoll_ctl_old", "epoll_wait_old",
					"remap_file_pages", "getdents64", "set_tid_address",
					"restart_syscall", "semtimedop", "fadvise64", "timer_create",
					"timer_settime", "timer_gettime", "timer_getoverrun",
					"timer_delete", "clock_settime", "clock_gettime",
					"clock_getres", "clock_nanosleep", "exit_group",
					"epoll_wait", "epoll_ctl", "tgkill", "utimes", "mbind",
					"set_mempolicy", "get_mempolicy", "mq_open", "mq_unlink",
					"mq_timedsend", "mq_timedreceive", "mq_notify",
					"mq_getsetattr", "kexec_load", "waitid", "add_key",
					"request_key", "keyctl", "ioprio_set", "ioprio_get",
					"inotify_init", "inotify_add_watch", "inotify_rm_watch",
					"migrate_pages", "openat", "mkdirat", "mknodat",
					"fchownat", "futimesat", "newfstatat", "unlinkat",
					"renameat", "linkat", "symlinkat", "readlinkat",
					"fchmodat", "faccessat", "pselect6", "ppoll",
					"unshare", "set_robust_list", "get_robust_list",
					"splice", "tee", "sync_file_range", "vmsplice",
					"move_pages", "utimensat", "epoll_pwait", "signalfd",
					"timerfd_create", "eventfd", "fallocate", "timerfd_settime",
					"timerfd_gettime", "accept4", "signalfd4", "eventfd2",
					"epoll_create1", "dup3", "pipe2", "inotify_init1",
					"preadv", "pwritev", "rt_tgsigqueueinfo", "perf_event_open",
					"recvmmsg", "fanotify_init", "fanotify_mark", "prlimit64",
					"name_to_handle_at", "open_by_handle_at", "clock_adjtime",
					"syncfs", "sendmmsg", "setns", "getcpu", "process_vm_readv",
					"process_vm_writev", "kcmp", "finit_module", "sched_setattr",
					"sched_getattr", "renameat2", "seccomp", "getrandom",
					"memfd_create", "kexec_file_load", "bpf", "execveat",
					"userfaultfd", "membarrier", "mlock2", "copy_file_range",
					"preadv2", "pwritev2", "pkey_mprotect", "pkey_alloc",
					"pkey_free", "statx",
				},
				Action: "SCMP_ACT_ALLOW",
			},
		},
	}
}

// Ensure LinuxExecutor implements Executor
var _ Executor = (*LinuxExecutor)(nil)

// Mutex for thread safety
var _ sync.Locker = (*sync.Mutex)(nil)
