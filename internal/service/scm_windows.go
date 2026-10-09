//go:build windows

package service

import (
	"context"
	"golang.org/x/sys/windows/svc"
)

type handler struct{ run func(context.Context) error }

func (h handler) Execute(args []string, r <-chan svc.ChangeRequest, s chan<- svc.Status) (bool, uint32) {
	s <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.run(ctx) }()
	s <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
	for {
		select {
		case err := <-done:
			if err != nil {
				return false, 1
			}
			return false, 0
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				s <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				s <- svc.Status{State: svc.StopPending}
				cancel()
				<-done
				return false, 0
			}
		}
	}
}
func Dispatch(run func(context.Context) error) (bool, error) {
	in, e := svc.IsWindowsService()
	if e != nil || !in {
		return false, e
	}
	return true, svc.Run("Yandu", handler{run})
}
