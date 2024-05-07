package proxy

import (
	"context"
	"fmt"
	"github.com/archip-io/deployment/api-gateway/internal/cfg"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
	"time"
)

type Backend struct {
	URL   *url.URL
	Alive atomic.Bool
	Proxy *httputil.ReverseProxy
}

type Service struct {
	RequireCheck *cfg.CheckService
	Backends     *Balancer
}

type Services struct {
	services map[string]*Service
}

type retryT struct{}

var retryKey = retryT{}

type backT struct{}

var backKey = backT{}

func GetRetryFromContext(r *http.Request) int {
	if val, ok := r.Context().Value(retryKey).(int); ok {
		return val
	}
	return 0
}

func GetBackFromContext(r *http.Request) *Backend {
	if val, ok := r.Context().Value(backKey).(*Backend); ok {
		return val
	}
	return nil
}

func ConsiderDelete(writer http.ResponseWriter, request *http.Request, e error, balancer *Balancer) {
	retries := GetRetryFromContext(request)
	back := GetBackFromContext(request)

	if retries < 10 {
		<-time.After(10 * time.Millisecond)
		ctx := context.WithValue(request.Context(), retryKey, retries+1)
		back.Proxy.ServeHTTP(writer, request.WithContext(ctx))
		return
	}

	if back.Alive.Swap(false) {
		balancer.RemoveBackend(back)
	}

	newBackend, err := balancer.GetBack()
	if err != nil {
		http.Error(writer, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}

	ctx := context.WithValue(request.Context(), backKey, newBackend)

	newBackend.Proxy.ServeHTTP(writer, request.WithContext(ctx))
}

func FormBackend(urlRaw string, errHandler func(writer http.ResponseWriter, request *http.Request, e error)) (*Backend, error) {
	urlP, err := url.Parse(urlRaw)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(urlP)
	proxy.ErrorHandler = errHandler

	back := &Backend{URL: urlP, Proxy: proxy}
	back.Alive.Store(true)

	return back, nil
}

func GetBackends(cfgs cfg.ServicesConfigs) (Services, error) {

	backs := Services{services: make(map[string]*Service, len(cfgs.Services))}

	for _, service := range cfgs.Services {
		curService := &Service{RequireCheck: service.CS, Backends: NewBalancer()}

		_, ok := backs.services[service.Name]
		if ok {
			return Services{}, fmt.Errorf("duplicate service name: %s", service.Name)
		}

		if service.URLs == nil || len(service.URLs) == 0 {
			return Services{}, fmt.Errorf("service %s has no URLs", service.Name)
		}

		for _, u := range service.URLs {

			backend, err := FormBackend(u, func(writer http.ResponseWriter, request *http.Request, e error) {
				ConsiderDelete(writer, request, e, curService.Backends)
			})

			if err != nil {
				return Services{}, err
			}

			curService.Backends.AddBackend(backend)
		}

		backs.services[service.Name] = curService
	}

	for _, service := range backs.services {
		if service.RequireCheck != nil {
			_, ok := backs.services[service.RequireCheck.Name]
			if !ok {
				return Services{}, fmt.Errorf("no service with name %s, which require for check", service.RequireCheck.Name)
			}
		}
	}

	return backs, nil
}
