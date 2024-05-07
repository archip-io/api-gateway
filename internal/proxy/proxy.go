package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func CheckAuth(r *http.Request, g *Gateway) (bool, error) {
	authVal := r.Header.Get("Authorization")

	if len(authVal) < 7 || authVal[:7] != "Bearer " {
		return false, InvalidToken
	}

	token := authVal[7:]

	ta := ToAuth{token}

	taSer, err := json.Marshal(ta)
	if err != nil {
		return false, err
	}
	body := bytes.NewBuffer(taSer)
	back, err := g.authService.Backends.GetBack()

	if err != nil {
		return false, err
	}

	req, err := http.NewRequest(http.MethodGet, back.URL.String()+g.service.RequireCheck.Path, body)
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}

	return resp.StatusCode == http.StatusOK, nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if g.service.RequireCheck != nil {
		ok, err := CheckAuth(r, g)
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

	fmt.Println(back, err)

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
	fmt.Println(backs)

	for n, service := range backs.services {
		g := &Gateway{service: service}
		if service.RequireCheck != nil {
			g.authService = backs.services[service.RequireCheck.Name]
		}
		mux.Handle(n, g)
		log.Println(n)
	}

	//mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	//	fmt.Println(r.URL)
	//
	//	http.Error(w, "Not found123123", http.StatusNotFound)
	//	return
	//})

	go checkBackends(backs)

	err := http.ListenAndServe(addr, mux)
	return err
}
