package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

type Gateway struct {
	service     *Service
	authService *Service
}

type ToAuth struct {
	Token string `json:"token"`
}

var InvalidToken = errors.New("invalid token")

var bearerTokenPrefix = "Bearer "

func getBearerToken(r *http.Request) (string, error) {
	authVal := r.Header.Get("Authorization")

	if len(authVal) < len(bearerTokenPrefix) || authVal[:len(bearerTokenPrefix)] != bearerTokenPrefix {
		return "", InvalidToken
	}

	return authVal[len(bearerTokenPrefix):], nil
}

var maxAuthIter = 1000

func doAuthRequest(autService *Service, path string, tokenJson []byte) (int, error) {

	for range maxAuthIter {
		back, err := autService.Backends.GetBack()
		if err != nil {
			break
		}

		body := bytes.NewBuffer(bytes.Clone(tokenJson))

		req, err := http.NewRequest(http.MethodGet, back.URL.String()+path, body)
		if err != nil {
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			if back.Alive.Swap(false) {
				autService.Backends.RemoveBackend(back)
			}
			continue
		}

		return resp.StatusCode, nil

	}

	return http.StatusServiceUnavailable, nil

}

func checkAuth(r *http.Request, g *Gateway) (bool, error) {

	token, err := getBearerToken(r)
	if err != nil {
		return false, err
	}

	ta := ToAuth{token}

	taSer, err := json.Marshal(ta)
	if err != nil {
		return false, err
	}
	code, err := doAuthRequest(g.authService, g.service.RequireCheck.Path, taSer)

	return code == http.StatusOK, nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if g.service.RequireCheck != nil {
		ok, err := checkAuth(r, g)
		if err != nil && errors.Is(err, InvalidToken) || !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	back, err := g.service.Backends.GetBack()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	ctx := context.WithValue(r.Context(), backKey, back)

	back.Proxy.ServeHTTP(w, r.WithContext(ctx))
}

func isBackendAlive(u *url.URL) bool {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		log.Println("Site unreachable, error: ", err)
		return false
	}
	_ = conn.Close() // close it, we dont need to maintain this connection
	return true
}

func checkBackends(ss Services) {
	for {
		for _, s := range ss.services {
			s.Backends.rw.RLock()
			for _, b := range s.Backends.backends {
				ok := isBackendAlive(b.URL)
				if !ok {
					if b.Alive.Swap(false) {
						s.Backends.rw.RUnlock()
						s.Backends.RemoveBackend(b)
						s.Backends.rw.RLock()
					}
				}
			}
			s.Backends.rw.RUnlock()
		}

		time.Sleep(2 * time.Second)
	}

}

func ListenConnections(addr string, backs Services) error {

	mux := http.NewServeMux()

	for n, service := range backs.services {
		g := &Gateway{service: service}
		if service.RequireCheck != nil {
			g.authService = backs.services[service.RequireCheck.Name]
		}
		mux.Handle(n, g)
		log.Println(n)
	}

	go checkBackends(backs)

	err := http.ListenAndServe(addr, mux)
	return err
}
