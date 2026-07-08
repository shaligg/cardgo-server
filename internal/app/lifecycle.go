package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	ilog "github.com/bigfish/go_orm_1/internal/infra/log"
)

func (a *Application) Start(ctx context.Context) error {
	if a.flushWorker != nil {
		a.flushWorker.Start()
	}

	if err := a.wsServer.Start(ctx); err != nil {
		return err
	}

	go func() {
		ilog.Infof("api server listening on %s", a.apiServer.Addr)
		if err := a.apiServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			ilog.Errorf("api server stopped with error: %v", err)
		}
	}()
	return nil
}

func (a *Application) Stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var firstErr error
	if err := a.wsServer.Stop(shutdownCtx); err != nil {
		firstErr = err
	}
	if a.flushWorker != nil {
		if err := a.flushWorker.Stop(shutdownCtx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if a.flushQueue != nil {
		if err := a.flushQueue.Drain(shutdownCtx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := a.apiServer.Shutdown(shutdownCtx); err != nil && firstErr == nil {
		firstErr = err
	}
	ilog.Infof("application stopped node=%s", a.cfg.Server.NodeID)
	return firstErr
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
