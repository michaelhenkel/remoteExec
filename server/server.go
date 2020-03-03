package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"

	"github.com/michaelhenkel/remoteExec/protos"
	"google.golang.org/grpc"
)

//go:generate protoc -I ../protos --go_out=plugins=grpc:../protos ../protos/remoteExec.proto

var (
	buf    bytes.Buffer
	logger = log.New(&buf, "logger: ", log.Lshortfile)
)

type server struct {
	protos.UnimplementedRemoteExecServer
}

func (s *server) GetIP(ctx context.Context, in *protos.Dummy) (*protos.CmdResult, error) {
	cmdResult := &protos.CmdResult{}
	localIP := getOutboundIP()
	cmdResult.Result = localIP.String()
	return cmdResult, nil
}

func (s *server) GetFileContent(ctx context.Context, in *protos.FilePath) (*protos.CmdResult, error) {
	cmdResult := &protos.CmdResult{}
	if _, err := os.Stat(in.Path); os.IsNotExist(err) {
		cmdResult.Result = "file doesn't exists"
		return cmdResult, nil
	}
	content, err := ioutil.ReadFile(in.Path)
	if err != nil {
		return cmdResult, err
	}
	cmdResult.Result = strings.TrimSuffix(string(content), "\n")
	return cmdResult, nil
}

func (s *server) ExecuteCommand(ctx context.Context, in *protos.Command) (*protos.CmdResult, error) {
	cmdResult := &protos.CmdResult{}
	result, err := execCmd(in.Cmd)

	if err != nil {
		return cmdResult, err
	}
	cmdResult.Result = result
	return cmdResult, nil
}

func main() {

	fmt.Println("Serving...")

	logger.Println("Started serving")
	socket := getFlag()
	if _, err := os.Stat(*socket); err == nil {
		logger.Println("socket exists, removing it")
		err = os.Remove(*socket)
	}
	lis, err := net.Listen("unix", *socket)
	if err != nil {
		logger.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	protos.RegisterRemoteExecServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		logger.Fatalf("failed to serve: %v", err)
	}
}

func getFlag() (socket *string) {
	socket = flag.String("socketpath", "/tmp/remotexec.socket", "absolute path to unix socket")
	flag.Parse()
	fmt.Printf("flags: socket: %s", *socket)
	return socket
}

func getOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		logger.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}

func execCmd(cmd string) (string, error) {
	cmdSlice := strings.Fields(cmd)
	execCmd := exec.Command(cmdSlice[0], cmdSlice[1:]...)
	out, err := execCmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
