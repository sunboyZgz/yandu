package storage

import (
	"context"
	"net/http"
	"time"
)

func (s *Service) guardObject(w http.ResponseWriter, r *http.Request, project, object string) (*http.Request, func()) {
	ctx, cancel := context.WithCancel(r.Context())
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		tick := time.NewTicker(time.Second)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if _, e := s.entry(project, object, false); e != nil {
					controller := http.NewResponseController(w)
					controller.SetReadDeadline(time.Now())
					controller.SetWriteDeadline(time.Now())
					cancel()
					return
				}
			}
		}
	}()
	return r.WithContext(ctx), func() { cancel(); <-finished }
}
