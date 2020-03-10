package executor

import (
	"bytes"
	"context"
	"log"
	"net"
	"os"
	"time"

	"github.com/michaelhenkel/remoteExec/protos"
	"google.golang.org/grpc"
)

//go:generate protoc -I ../protos --go_out=plugins=grpc:../protos ../protos/remoteExec.proto

var (
	buf    bytes.Buffer
	logger = log.New(&buf, "logger: ", log.Lshortfile)
)

type Executor struct {
	Socket string
}

func (e *Executor) GetIP() (*string, error) {
	socket := e.Socket
	c, ctx, conn, cancel := newClient(&socket)
	defer conn.Close()
	defer cancel()
	ipGetResult, err := c.GetIP(ctx, &protos.Dummy{})
	if err != nil {
		return nil, err
	}
	return &ipGetResult.Result, nil
}

func (e *Executor) GetFileContent(filePath string) (*string, error) {
	socket := e.Socket
	c, ctx, conn, cancel := newClient(&socket)
	defer conn.Close()
	defer cancel()
	fileResult, err := c.GetFileContent(ctx, &protos.FilePath{Path: filePath})
	if err != nil {
		return nil, err
	}
	return &fileResult.Result, nil
}

func (e *Executor) ExecuteCommand(cmd string) (*string, error) {
	socket := e.Socket
	c, ctx, conn, cancel := newClient(&socket)
	defer conn.Close()
	defer cancel()
	cmdResult, err := c.ExecuteCommand(ctx, &protos.Command{Cmd: cmd})
	if err != nil {
		return nil, err
	}
	return &cmdResult.Result, nil
}

func (e *Executor) ServiceRunning(address, protocol string, port int) (*bool, error) {
	socket := e.Socket
	c, ctx, conn, cancel := newClient(&socket)
	defer conn.Close()
	defer cancel()
	cmdResult, err := c.ServiceRunning(ctx, &protos.Service{
		Address:  address,
		Port:     int32(port),
		Protocol: protocol,
	})
	if err != nil {
		return nil, err
	}

	return &cmdResult.Result, nil
}

func (e *Executor) SetupTunnel(vmPort, hostPort int, username, address string) (*string, error) {
	socket := e.Socket
	c, ctx, conn, cancel := newClient(&socket)
	defer conn.Close()
	defer cancel()
	cmdResult, err := c.AddTunnel(ctx, &protos.Tunnel{
		HostPort: int32(hostPort),
		VMPort:   int32(vmPort),
		Username: username,
		Address:  address,
	})
	if err != nil {
		return nil, err
	}
	return &cmdResult.Result, nil
}

func (e *Executor) DeleteTunnel(vmPort, hostPort int, username, address string) (*string, error) {
	socket := e.Socket
	c, ctx, conn, cancel := newClient(&socket)
	defer conn.Close()
	defer cancel()
	cmdResult, err := c.DeleteTunnel(ctx, &protos.Tunnel{
		HostPort: int32(hostPort),
		VMPort:   int32(vmPort),
		Username: username,
		Address:  address,
	})
	if err != nil {
		return nil, err
	}
	return &cmdResult.Result, nil
}

func unixConnect(addr string, t time.Duration) (net.Conn, error) {
	unixAddr, err := net.ResolveUnixAddr("unix", addr)
	conn, err := net.DialUnix("unix", nil, unixAddr)
	return conn, err
}

func newClient(socket *string) (protos.RemoteExecClient, context.Context, *grpc.ClientConn, context.CancelFunc) {
	if _, err := os.Stat(*socket); os.IsNotExist(err) {
		logger.Fatalf("socket %s doesn't exist, server running?", *socket)
	}
	logger.Printf("Dialing socket %s", *socket)
	conn, err := grpc.Dial(*socket, grpc.WithInsecure(), grpc.WithDialer(unixConnect))
	if err != nil {
		logger.Fatalf("did not connect: %v", err)
	}
	c := protos.NewRemoteExecClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	return c, ctx, conn, cancel
}
