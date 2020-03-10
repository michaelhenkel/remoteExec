package sshtunnel

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/function61/gokit/backoff"
	"github.com/function61/gokit/bidipipe"
	"github.com/function61/gokit/logex"
	"golang.org/x/crypto/ssh"
)

func handleClient(client net.Conn, forward Forward, logger *log.Logger) {
	defer client.Close()

	logl := logex.Levels(logger)

	logl.Info.Printf("%s connected", client.RemoteAddr())
	defer logl.Info.Println("closed")

	remote, err := net.Dial("tcp", forward.Local.String())
	if err != nil {
		logl.Error.Printf("dial INTO local service error: %s", err.Error())
		return
	}

	if err := bidipipe.Pipe(client, "client", remote, "remote"); err != nil {
		logl.Error.Println(err.Error())
	}
}

func connectToSshAndServe(
	ctx context.Context,
	conf *Configuration,
	auth ssh.AuthMethod,
	logger *log.Logger,
	makeLogger loggerFactory,
) error {
	logl := logex.Levels(logger)

	logl.Info.Println("connecting")

	sshConfig := &ssh.ClientConfig{
		User:            conf.SshServer.Username,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshClient, errConnect := connectSSH(ctx, conf.SshServer.Address, sshConfig)

	if errConnect != nil {
		return errConnect
	}

	defer sshClient.Close()
	defer logl.Info.Println("disconnecting")

	logl.Info.Println("connected; starting to forward ports")

	listenerStopped := make(chan error, len(conf.Forwards))

	for _, forward := range conf.Forwards {
		if err := forwardOnePort(
			forward,
			sshClient,
			listenerStopped,
			makeLogger("forwardOnePort"),
			makeLogger,
		); err != nil {
			// closes SSH connection if even one forward Listen() fails
			return err
		}
	}

	select {
	case <-ctx.Done():
		return nil
	case listenerFirstErr := <-listenerStopped:
		// assumes all the other listeners failed too so no teardown necessary
		return listenerFirstErr
	}
}

func forwardOnePort(
	forward Forward,
	sshClient *ssh.Client,
	listenerStopped chan<- error,
	logger *log.Logger,
	mkLogger loggerFactory,
) error {
	logl := logex.Levels(logger)

	// Listen on remote server port
	listener, err := sshClient.Listen("tcp", forward.Remote.String())
	if err != nil {
		return err
	}

	go func() {
		defer listener.Close()

		logl.Info.Printf("listening remote %s", forward.Remote.String())

		// handle incoming connections on reverse forwarded tunnel
		for {
			client, err := listener.Accept()
			if err != nil {
				listenerStopped <- fmt.Errorf("Accept(): %s", err.Error())
				return
			}

			go handleClient(client, forward, mkLogger("handleClient"))
		}
	}()

	return nil
}

func TunnelWatcher(tunnelPath *string) error {

	_, err := os.Stat(*tunnelPath)
	if !os.IsNotExist(err) {
		_, err := os.Create(*tunnelPath)
		if err != nil {
			return err
		}
	}
	/*
		watchFile := strings.Split(*tunnelPath, "/")
		watchPath := strings.TrimSuffix(*tunnelPath, watchFile[len(watchFile)-1])
		tunnelWatcher, err := WatchFile(watchPath, time.Second, func() {
			_, err := os.Stat(*tunnelPath)
			if !os.IsNotExist(err) {
				nodeManager(controlNodesPtr, "control", contrailClient)
			} else if os.IsNotExist(err) {
				controlNodes(contrailClient, []*types.ControlNode{})
			}
		})
	*/
	return nil

}

func AddTunnel(ctx context.Context, conf *Configuration) error {
	log.Println("sshtunnel: setup tunnel called")
	logger := logex.StandardLogger()
	//ctx, cancel := context.WithCancel(context.Background())
	//ctx := ossignal.InterruptOrTerminateBackgroundCtx(logger)
	privateKey, err := signerFromPrivateKeyFile(conf.SshServer.PrivateKeyFilePath)
	if err != nil {
		return err
	}
	sshAuth := ssh.PublicKeys(privateKey)
	// 0ms, 100 ms, 200 ms, 400 ms, ...
	backoffTime := backoff.ExponentialWithCappedMax(100*time.Millisecond, 5*time.Second)
	go func() error {
		for {
			log.Println("server: trying to setup tunnel")
			err := connectToSshAndServe(
				ctx,
				conf,
				sshAuth,
				logex.Prefix("connectToSshAndServe", logger),
				mkLoggerFactory(logger))
			if err != nil {
				log.Println("server: failed to setup tunnel")
				return err
			}
			select {
			case <-ctx.Done():
				return nil
			default:
			}

			logex.Levels(logger).Error.Println(err.Error())

			time.Sleep(backoffTime())
		}
	}()

	return nil
}

func connectSSH(ctx context.Context, addr string, sshConfig *ssh.ClientConfig) (*ssh.Client, error) {
	dialer := net.Dialer{
		Timeout: 10 * time.Second,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	return sshClientForConn(conn, addr, sshConfig)
}

func sshClientForConn(conn net.Conn, addr string, sshConfig *ssh.ClientConfig) (*ssh.Client, error) {
	sconn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
	if err != nil {
		return nil, err
	}

	return ssh.NewClient(sconn, chans, reqs), nil
}

type loggerFactory func(prefix string) *log.Logger

func mkLoggerFactory(rootLogger *log.Logger) loggerFactory {
	return func(prefix string) *log.Logger {
		return logex.Prefix(prefix, rootLogger)
	}
}

func exitIfError(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
