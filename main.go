package main

import "github.com/0-u-0/dugon-signal-server/libs"

func main() {
	clientGroup := libs.NewClientGroup()
	go clientGroup.Run()
	libs.InitWsServer(clientGroup)
}
