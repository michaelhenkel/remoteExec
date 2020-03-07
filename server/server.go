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

	"github.com/michaelhenkel/remoteExec/genkey"
	"github.com/michaelhenkel/remoteExec/protos"
	"github.com/michaelhenkel/remoteExec/sshtunnel"
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
	fmt.Println("Getting IP")
	cmdResult := &protos.CmdResult{}
	localIP := getOutboundIP()
	fmt.Printf("Got IP: %s\n", localIP.String())
	cmdResult.Result = localIP.String()
	return cmdResult, nil
}

func (s *server) GetFileContent(ctx context.Context, in *protos.FilePath) (*protos.CmdResult, error) {
	fmt.Printf("Getting content for file %s\n", in.Path)
	cmdResult := &protos.CmdResult{}
	if _, err := os.Stat(in.Path); os.IsNotExist(err) {
		cmdResult.Result = "file doesn't exists"
		return cmdResult, nil
	}
	content, err := ioutil.ReadFile(in.Path)
	fmt.Printf("Trying to read file: %s\n", in.Path)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		cmdResult.Result = err.Error()
		return cmdResult, err
	}
	fmt.Printf("Content of file: %s\n", string(content))
	cmdResult.Result = strings.TrimSuffix(string(content), "\n")
	return cmdResult, nil
}

func (s *server) ExecuteCommand(ctx context.Context, in *protos.Command) (*protos.CmdResult, error) {
	fmt.Println("Executing ...")
	cmdResult := &protos.CmdResult{}
	fmt.Printf("Executing cmd: %s\n", in.Cmd)
	result, err := execCmd(in.Cmd)

	if err != nil {
		fmt.Printf("Error: %s\n", err)
		cmdResult.Result = err.Error()
		return cmdResult, err
	}
	fmt.Printf("result of cmd: %s\n", result)
	cmdResult.Result = result
	return cmdResult, nil
}

func (s *server) AddTunnel(ctx context.Context, in *protos.Tunnel) (*protos.CmdResult, error) {
	fmt.Println("Setting up tunnel ...")
	cmdResult := &protos.CmdResult{}
	fmt.Printf("Tunnel srcPort %d, hostPort %d, user %s\n", in.GetVMPort(), in.GetHostPort(), in.GetUsername())
	err := setupTunnel(in)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		cmdResult.Result = err.Error()
		return cmdResult, err
	}
	cmdResult.Result = "tunnel created"
	return cmdResult, nil
}

func main() {

	fmt.Println("Serving...")

	privateKeyPath := "/id_rsa"
	publicKeyPath := "/id_rsa.pub"
	privKeyExists := false
	pubKeyExists := false
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		privKeyExists = true
	}
	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		privKeyExists = true
	}
	if !privKeyExists || !pubKeyExists {
		genkey.GenKey(privateKeyPath, publicKeyPath)
	}

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

func setupTunnel(tunnel *protos.Tunnel) error {
	log.Println("setup tunnel called")
	gatewayIP := getOutboundIP()
	gatewayIP = gatewayIP.To4()
	gatewayIP[3]++
	config := &sshtunnel.Configuration{
		SshServer: sshtunnel.SshServer{
			Address:            tunnel.GetAddress(),
			Username:           tunnel.GetUsername(),
			PrivateKeyFilePath: "/id_rsa",
		},
		Forwards: []sshtunnel.Forward{{
			Local: sshtunnel.Endpoint{
				Host: "127.0.0.1",
				Port: int(tunnel.GetVMPort()),
			},
			Remote: sshtunnel.Endpoint{
				Host: "127.0.0.1",
				Port: int(tunnel.GetHostPort()),
			},
		}},
	}
	err := sshtunnel.SetupTunnel(config)
	if err != nil {
		return err
	}
	return nil
}
