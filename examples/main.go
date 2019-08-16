package main

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	iterm2 "github.com/tjamet/goterm2"
	"github.com/tjamet/goterm2/api"
)

func main() {
	logger := log.New()
	logger.SetOutput(os.Stdout)
	logger.SetLevel(log.TraceLevel)
	i, err := iterm2.New()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	i.Logger(logger)
	fmt.Println(i.ListSessions(&api.ListSessionsRequest{}))
}
