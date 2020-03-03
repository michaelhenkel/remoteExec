package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
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

func main() {
	socket, getIP, file, cmd := getFlags()
	c, ctx, conn, cancel := newClient(socket)
	defer conn.Close()
	defer cancel()
	if *getIP {
		ipGetResult, err := c.GetIP(ctx, &protos.Dummy{})
		if err != nil {
			logger.Fatal(err)
		}
		fmt.Println(ipGetResult.Result)
	}
	if *file != "" {
		fileResult, err := c.GetFileContent(ctx, &protos.FilePath{Path: *file})
		if err != nil {
			logger.Fatal(err)
		}
		fmt.Println(fileResult.Result)
	}

	if *cmd != "" {
		cmdResult, err := c.ExecuteCommand(ctx, &protos.Command{Cmd: *cmd})
		if err != nil {
			logger.Fatal(err)
		}
		fmt.Println(cmdResult.Result)
	}
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

func getFlags() (*string, *bool, *string, *string) {
	socket := flag.String("socketpath", "/tmp/remotexec.socket", "absolute path to unix socket")
	ip := flag.Bool("ip", false, "Get ip")
	file := flag.String("file", "", "file to read")
	cmd := flag.String("cmd", "", "command to execute")
	flag.Parse()

	return socket, ip, file, cmd
}
