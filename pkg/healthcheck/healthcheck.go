package healthcheck

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type CheckFunc func(ctx context.Context) error

type Checker struct {
	mu       sync.RWMutex
	liveness map[string]CheckFunc
	readiness map[string]CheckFunc
}

func New() *Checker {
	return &Checker{
		liveness:  make(map[string]CheckFunc),
		readiness: make(map[string]CheckFunc),
	}
}

func (c *Checker) AddLiveness(name string, fn CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.liveness[name] = fn
}

func (c *Checker) AddReadiness(name string, fn CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readiness[name] = fn
}

type result struct {
	Status string `json:"status"`
	Checks map[string]string `json:"checks,omitempty"`
}

func (c *Checker) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", c.handleChecks(c.getLiveness))
	mux.HandleFunc("/readyz", c.handleChecks(c.getReadiness))
	return mux
}

func (c *Checker) getLiveness() map[string]CheckFunc {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cp := make(map[string]CheckFunc, len(c.liveness))
	for k, v := range c.liveness {
		cp[k] = v
	}
	return cp
}

func (c *Checker) getReadiness() map[string]CheckFunc {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cp := make(map[string]CheckFunc, len(c.readiness))
	for k, v := range c.readiness {
		cp[k] = v
	}
	return cp
}

func (c *Checker) handleChecks(getChecks func() map[string]CheckFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		checks := getChecks()
		results := make(map[string]string, len(checks))
		allOK := true

		for name, fn := range checks {
			if err := fn(ctx); err != nil {
				results[name] = err.Error()
				allOK = false
			} else {
				results[name] = "ok"
			}
		}

		res := result{Checks: results}
		status := http.StatusOK
		if allOK {
			res.Status = "ok"
		} else {
			res.Status = "unavailable"
			status = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(res)
	}
}
