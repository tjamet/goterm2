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
	sessions, err := i.ListSessions(&api.ListSessionsRequest{})
	if err != nil {
		panic(err)
	}
	i.SubscribeNewSessionNotifications(func(n *api.NewSessionNotification) error {
		fmt.Println(n.String())
		return nil
	})
	fmt.Println(i.RegisterNotifier(iterm2.TerminateSessionNotifier(func(n *api.TerminateSessionNotification) error {
		fmt.Println(n.String())
		return nil
	})))

	for _, w := range sessions.GetWindows() {
		for _, t := range w.GetTabs() {
			for _, s := range t.GetRoot().GetLinks() {
				i.SubscribePromptMonitorNotifications(s.GetSession(), &api.PromptMonitorRequest{
					Modes: []api.PromptMonitorMode{api.PromptMonitorMode_COMMAND_START, api.PromptMonitorMode_COMMAND_END, api.PromptMonitorMode_PROMPT},
				}, func(n *api.PromptNotification) error {
					fmt.Println(n.String())
					return nil
				})
				// i.SubscribeScreenUpdateNotifications(s.GetSession(), func(n *api.ScreenUpdateNotification) error {
				// 	fmt.Println(n.String())
				// 	return nil
				// })
			}
		}
	}
	<-make(chan interface{})
	//i.ServerOriginatedRpcResult(&api.RPCRegistrationRequest{})
}
