package check

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// sh runs a recipe command (install, build, migrate) to the end.
func (c *checker) sh(ctx context.Context, command string, env []string) error {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Dir, cmd.Env = c.dir, env
	var out tail
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s failed (%v): %s", command, err, lastLine(out.b))
	}
	return nil
}

type process struct {
	cmd  *exec.Cmd
	out  tail
	done chan struct{}
}

// startProcess runs the start command in its own process group, so stop can end
// everything it starts.
func startProcess(dir, command string, env []string) (*process, error) {
	p := &process{cmd: exec.Command("/bin/sh", "-c", command), done: make(chan struct{})}
	p.cmd.Dir, p.cmd.Env = dir, env
	p.cmd.Stdout, p.cmd.Stderr = &p.out, &p.out
	p.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	// A child that left the group could hold the output open and keep Wait from returning.
	p.cmd.WaitDelay = time.Second
	if err := p.cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		p.cmd.Wait()
		close(p.done)
	}()
	return p, nil
}

// stop ends the app and returns once it has exited. It is safe to call twice.
func (p *process) stop() {
	syscall.Kill(-p.cmd.Process.Pid, syscall.SIGTERM)
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
		syscall.Kill(-p.cmd.Process.Pid, syscall.SIGKILL)
		<-p.done
	}
}

// memory is the resident memory of the app and everything it started, or 0 where the
// machine has no ps.
func (p *process) memory() int64 {
	out, _ := exec.Command("ps", "-A", "-o", "pgid=,rss=").Output()
	pgid := strconv.Itoa(p.cmd.Process.Pid)
	var kb int64
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == pgid {
			n, _ := strconv.ParseInt(f[1], 10, 64)
			kb += n
		}
	}
	return kb << 10
}

// outsideIP is an address of this machine other than loopback. An app that listens only on
// 127.0.0.1 does not answer on it, as it would not answer the gateway.
func outsideIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.To4() != nil && !n.IP.IsLoopback() && !n.IP.IsLinkLocalUnicast() {
			return n.IP.String(), nil
		}
	}
	return "", errors.New("this machine has no network address besides 127.0.0.1, so listening on 0.0.0.0 cannot be checked: connect to a network")
}

// tail keeps the last 16 KB written to it.
type tail struct{ b []byte }

func (t *tail) Write(p []byte) (int, error) {
	t.b = append(t.b, p...)
	if over := len(t.b) - 16<<10; over > 0 {
		t.b = t.b[over:]
	}
	return len(p), nil
}
