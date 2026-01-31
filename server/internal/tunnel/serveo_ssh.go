package tunnel

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

const serveoHost = "serveo.net:22"

// startServeoNativeSSH starts a reverse tunnel to Serveo using Go's SSH client
// (no system ssh binary required). Works on all platforms.
// Returns the public URL, a cleanup function, and an error.
func startServeoNativeSSH(ctx context.Context, port int, subdomain string) (url string, cleanup func(), err error) {
	if port == 0 {
		port = 8080
	}
	// Serveo with subdomain: we know the URL. Without subdomain we cannot get it from protocol.
	if subdomain == "" {
		return "", nil, fmt.Errorf("subdomain required for native SSH tunnel (use echo-xxx)")
	}
	url = "https://" + subdomain + ".serveo.net"

	config := &ssh.ClientConfig{
		User:            "nobody",           // Serveo accepts unauthenticated tunneling
		Auth:            []ssh.AuthMethod{}, // no auth
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         15 * time.Second,
	}

	client, err := ssh.Dial("tcp", serveoHost, config)
	if err != nil {
		return "", nil, fmt.Errorf("ssh dial serveo: %w", err)
	}

	// Request remote listen: for Serveo, bind address can be subdomain to get that subdomain
	listenAddr := subdomain + ":80"
	listener, err := client.Listen("tcp", listenAddr)
	if err != nil {
		client.Close()
		return "", nil, fmt.Errorf("ssh reverse listen: %w", err)
	}

	var once sync.Once
	cleanup = func() {
		once.Do(func() {
			listener.Close()
			client.Close()
		})
	}

	go func() {
		defer cleanup()
		for {
			remote, err := listener.Accept()
			if err != nil {
				return
			}
			go func(remote net.Conn) {
				defer remote.Close()
				local, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
				if err != nil {
					return
				}
				defer local.Close()
				go io.Copy(local, remote)
				io.Copy(remote, local)
			}(remote)
		}
	}()

	// Keep connection alive until ctx is done
	go func() {
		<-ctx.Done()
		cleanup()
	}()

	return url, cleanup, nil
}
