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

	"github.com/michaelhenkel/remoteExec/executor"

)

//go:generate protoc -I ../protos --go_out=plugins=grpc:../protos ../protos/remoteExec.proto

var (
	buf    bytes.Buffer
	logger = log.New(&buf, "logger: ", log.Lshortfile)
)

func main() {
	socket, getIP, file, cmd := getFlags()
	
	e := &executor.Executor{
		Socket = socket
	}

	if *getIP {
		ipResult, err := e.GetIP()
		if err != nil {
			logger.Fatal(err)
		}
		fmt.Println(*ipResult)
	}

	var fileResult *string
	if *file != "" {
		fileResult, err := e.GetFileContent(*file)
		if err != nil {
			logger.Fatal(err)
		}
		fmt.Println(*fileResult)
	}

	var cmdResult *string
	if *cmd != "" {
		cmdResult, err := e.ExecuteCommand(*cmd)
		if err != nil {
			logger.Fatal(err)
		}
		fmt.Println(*cmdResult)
	}
}
