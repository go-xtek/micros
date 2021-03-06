package healthcheck

import (
	"context"
	"fmt"
	"github.com/gidyon/micros"
	"github.com/gidyon/micros/pkg/conn"
	"net/http"
	"sync"
	"time"
)

const (
	// ProbeLiveNess indicates the health check probe is a liveness check i.e service is running correctly
	ProbeLiveNess = "liveness"
	// ProbeReadiness indicates the health check probe is a readiness check i.e service has started and can service requests
	ProbeReadiness = "readiness"
	// ProbeStartup indicates the health check probe is a startup check i.e service has started correctly
	ProbeStartup = "startup"
)

// ProbeOptions contains data and options required for doing healthcheck
type ProbeOptions struct {
	successMsg   string
	Service      *micros.Service
	AutoMigrator func() error
	Type         string
}

// RegisterProbe ...
func RegisterProbe(opt *ProbeOptions) http.HandlerFunc {
	var (
		service = opt.Service
		cfg     = opt.Service.Config()
	)

	serviceNil := service == nil
	cfgNil := cfg == nil

	// apply defaults
	if !serviceNil && !cfgNil {
		switch opt.Type {
		case ProbeLiveNess:
			opt.successMsg = fmt.Sprintf("service %q is running correctly :)", cfg.ServiceName())
		case ProbeReadiness:
			opt.successMsg = fmt.Sprintf("service %q is ready :)", cfg.ServiceName())
		case ProbeStartup:
			opt.successMsg = fmt.Sprintf("service %q has started :)", cfg.ServiceName())
		default:
			opt.successMsg = fmt.Sprintf("service %q is ready and running :)", cfg.ServiceName())
		}
	}

	// Check only service or app internals and not external components
	return func(w http.ResponseWriter, r *http.Request) {
		// Handle any panic
		defer func() {
			if err := recover(); err != nil {
				errMsg := fmt.Sprintf("unexpected error: %v", err)
				fmt.Fprintln(w, errMsg)
			}
		}()

		var (
			mu     = &sync.Mutex{}
			errMsg string
			errs   = make([]string, 0)
		)

		if serviceNil {
			w.WriteHeader(http.StatusExpectationFailed)
			w.Write([]byte("service is uninitialized"))
			return
		}

		if cfgNil {
			w.WriteHeader(http.StatusExpectationFailed)
			w.Write([]byte("service has no configuration options"))
			return
		}

		ctx := r.Context()

		nCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		if cfg.UseSQLDatabase() {
			// Test DB connection
			err := opt.AutoMigrator()
			if err != nil {
				errMsg = fmt.Sprintf("failed to check database health: %v", err)
				errs = append(errs, errMsg)
			}
		}

		if cfg.UseRedis() {
			// Ping redis connection
			statusCMD := service.RedisClient().Ping()
			if err := statusCMD.Err(); err != nil {
				errMsg = fmt.Sprintf("failed to check redis health: %v", err)
				errs = append(errs, errMsg)
			}
		}

		wg := &sync.WaitGroup{}

		// check external services
		for _, extSrv := range cfg.ExternalServices() {
			if !extSrv.Available() {
				continue
			}
			wg.Add(1)
			extSrv := extSrv
			// dials concurrently
			go func() {
				defer wg.Done()

				cc, err := conn.DialService(nCtx, &conn.GRPCDialOptions{
					Address:     extSrv.Address(),
					TLSCertFile: extSrv.TLSCertFile(),
					ServerName:  extSrv.ServerName(),
					WithBlock:   true,
					K8Service:   extSrv.K8Service(),
				})
				if err != nil {
					mu.Lock()
					errMsg = fmt.Sprintf("failed to connect to %s service: %v", extSrv.Name(), err)
					errs = append(errs, errMsg)
					mu.Unlock()
				} else {
					defer cc.Close()
				}
			}()
		}

		// wait until all dials complete
		wg.Wait()

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)

		// Check errors from external components
		if len(errs) != 0 {
			for _, err := range errs {
				fmt.Fprintln(w, err)
			}
			return
		}

		fmt.Fprintln(w, opt.successMsg)
	}
}
