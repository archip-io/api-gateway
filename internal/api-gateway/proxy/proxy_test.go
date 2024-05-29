package proxy

import (
	"encoding/json"
	"github.com/archip-io/deployment/api-gateway/internal/api-gateway/cfg"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
)

var iter = atomic.Int32{}

func iterHandler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		_ = r.Body.Close()
	}()

	old := iter.Add(1) - 1
	_, _ = w.Write([]byte(strconv.Itoa(int(old))))
}

type Token struct {
	Token string `json:"token"`
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	defer func() {
		_ = r.Body.Close()
	}()

	t := Token{}
	err := decoder.Decode(&t)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if t.Token == "111" {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusUnauthorized)
}

func TestProxyWithoutAuth(t *testing.T) {

	echoServer := httptest.NewServer(http.HandlerFunc(iterHandler))
	defer echoServer.Close()
	flag := true
	defer func() {
		if flag {
			echoServer.Close()
		}
	}()

	service, err := ProcessService(cfg.ServiceCfg{
		Name: "",
		URLs: []string{echoServer.URL},
		CS:   nil,
	})
	if err != nil {
		panic(err)
	}

	handler := &Gateway{service: service, authService: nil}

	recorder := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", echoServer.URL, nil)
	handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "0", recorder.Body.String())

	recorder = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", echoServer.URL, nil)
	handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "1", recorder.Body.String())

	echoServer.Close()
	flag = false

	recorder = httptest.NewRecorder()
	req, _ = http.NewRequest("GET", echoServer.URL, nil)
	handler.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
}

func TestProxyWitAuth(t *testing.T) {
	iter.Store(0)

	echoServer := httptest.NewServer(http.HandlerFunc(iterHandler))
	defer echoServer.Close()
	authServer1 := httptest.NewServer(http.HandlerFunc(authHandler))
	flag1 := true
	defer func() {
		if flag1 {
			authServer1.Close()
		}
	}()

	authServer2 := httptest.NewServer(http.HandlerFunc(authHandler))
	flag2 := true
	defer func() {
		if flag2 {
			authServer1.Close()
		}
	}()

	service, err := ProcessService(cfg.ServiceCfg{
		Name: "",
		URLs: []string{echoServer.URL},
		CS: &cfg.CheckService{
			Name: "auth",
			Path: "",
		},
	})
	if err != nil {
		panic(err)
	}

	serviceAuth, err := ProcessService(cfg.ServiceCfg{
		Name: "auth",
		URLs: []string{authServer1.URL, authServer2.URL},
		CS:   nil,
	})
	if err != nil {
		panic(err)
	}

	handler := &Gateway{service: service, authService: serviceAuth}

	t.Run("has right token", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", echoServer.URL, nil)
		req.Header.Add("Authorization", "Bearer 111")
		handler.ServeHTTP(recorder, req)

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, "0", recorder.Body.String())
	})

	iter.Store(1)

	t.Run("no token", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", echoServer.URL, nil)
		handler.ServeHTTP(recorder, req)

		require.Equal(t, http.StatusUnauthorized, recorder.Code)

		require.Equal(t, int32(1), iter.Load())
	})

	t.Run("wrong token", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", echoServer.URL, nil)
		req.Header.Add("Authorization", "Bearer 222")
		handler.ServeHTTP(recorder, req)

		require.Equal(t, http.StatusUnauthorized, recorder.Code)
		require.Equal(t, int32(1), iter.Load())
	})

	t.Run("wrong token format", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", echoServer.URL, nil)
		req.Header.Add("Authorization", "asklmgpakwm")
		handler.ServeHTTP(recorder, req)

		require.Equal(t, http.StatusUnauthorized, recorder.Code)
		require.Equal(t, int32(1), iter.Load())

		recorder = httptest.NewRecorder()
		req, _ = http.NewRequest("GET", echoServer.URL, nil)
		req.Header.Add("Authorization", "")
		handler.ServeHTTP(recorder, req)

		require.Equal(t, http.StatusUnauthorized, recorder.Code)
		require.Equal(t, int32(1), iter.Load())
	})

	authServer1.Close()
	flag1 = false

	t.Run("auth1 server is down, service still available", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", echoServer.URL, nil)
		req.Header.Add("Authorization", "Bearer 111")
		handler.ServeHTTP(recorder, req)

		require.Equal(t, http.StatusOK, recorder.Code)
		require.Equal(t, "1", recorder.Body.String())
	})

	iter.Store(2)
	authServer2.Close()
	flag2 = false

	t.Run("all auth down", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", echoServer.URL, nil)
		req.Header.Add("Authorization", "Bearer 111")
		handler.ServeHTTP(recorder, req)

		require.Equal(t, http.StatusUnauthorized, recorder.Code)
		require.Equal(t, int32(2), iter.Load())

	})
}
