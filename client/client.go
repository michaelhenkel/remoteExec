package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/michaelhenkel/remoteExec/client/executor"
)

//go:generate protoc -I ../protos --go_out=plugins=grpc:../protos ../protos/remoteExec.proto

var (
	buf    bytes.Buffer
	logger = log.New(&buf, "logger: ", log.Lshortfile)
)

func main() {
	socket, getIP, file, cmd, tunnel := getFlags()

	e := &executor.Executor{
		Socket: *socket,
	}

	if *getIP {
		ipResult, err := e.GetIP()
		if err != nil {
			logger.Fatal(err)
		}
		fmt.Println(*ipResult)
	}

	if *file != "" {
		fileResult, err := e.GetFileContent(*file)
		if err != nil {
			logger.Fatal(err)
		}
		fmt.Println(*fileResult)
	}

	if *cmd != "" {
		cmdResult, err := e.ExecuteCommand(*cmd)
		if err != nil {
			logger.Fatal(err)
		}
		fmt.Println(*cmdResult)
	}

	if *tunnel != "" {
		tunnelSlice := strings.Split(*tunnel, ",")
		vmPort, _ := strconv.Atoi(tunnelSlice[0])
		hostPort, _ := strconv.Atoi(tunnelSlice[1])
		cmdResult, err := e.SetupTunnel(vmPort, hostPort, tunnelSlice[2], tunnelSlice[3])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println(*cmdResult)
	}

}

func getFlags() (*string, *bool, *string, *string, *string) {
	socket := flag.String("socketpath", "/tmp/remotexec.socket", "absolute path to unix socket")
	ip := flag.Bool("ip", false, "Get ip")
	file := flag.String("file", "", "file to read")
	cmd := flag.String("cmd", "", "command to execute")
	tunnel := flag.String("tunnel", "", "tunnel to be added")
	flag.Parse()

	return socket, ip, file, cmd, tunnel
}
