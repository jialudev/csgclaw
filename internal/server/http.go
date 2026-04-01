package server

import (
	"context"
	"net/http"
	"time"

	"csgclaw/internal/agent"
	"csgclaw/internal/api"
	"csgclaw/internal/im"
)

type Options struct {
	ListenAddr string
	Service    *agent.Service
	IM         *im.Service
	IMBus      *im.Bus
	PicoClaw   *im.PicoClawBridge
	Context    context.Context
}

func Run(opts Options) error {
	if opts.Context == nil {
		opts.Context = context.Background()
	}

	handler := api.NewHandler(opts.Service, opts.IM, opts.IMBus, opts.PicoClaw)
	mux := handler.Routes()
	mux.Handle("/", uiHandler())

	httpServer := &http.Server{
		Addr:              opts.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if opts.IMBus != nil && opts.PicoClaw != nil {
		events, cancel := opts.IMBus.Subscribe()
		defer cancel()

		go func() {
			for {
				select {
				case <-opts.Context.Done():
					return
				case evt, ok := <-events:
					if !ok {
						return
					}
					handler.PublishPicoClawEvent(evt)
				}
			}
		}()
	}

	errCh := make(chan error, 1)
	go func() {
		<-opts.Context.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		errCh <- err
	}

	close(errCh)
	if err := <-errCh; err != nil {
		return err
	}
	if opts.Service != nil {
		return opts.Service.Close()
	}
	return nil
}
