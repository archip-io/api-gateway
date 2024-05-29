package proxy

import (
	"github.com/stretchr/testify/require"
	"net/http"
	"testing"
)

func getTestBacks() []*Backend {
	urls := []string{
		"http://localhost:8001",
		"http://localhost:8002",
		"http://localhost:8003",
	}

	testBacks := make([]*Backend, 0, len(urls))

	for _, url := range urls {
		b, err := FormBackend(url, func(writer http.ResponseWriter, request *http.Request, e error) {})
		if err != nil {
			panic(err)
		}

		testBacks = append(testBacks, b)
	}

	return testBacks
}

func TestBalancer(t *testing.T) {
	backs := getTestBacks()
	balancer := NewBalancer()

	for _, b := range backs {
		balancer.AddBackend(b)
	}

	t.Run("balancer returns all backends uniformly", func(t *testing.T) {
		urlsGet := make(map[string]int)
		toEachBackReq := 10
		for i := 0; i < toEachBackReq*len(backs); i++ {
			b, err := balancer.GetBack()
			require.NoError(t, err)

			urlsGet[b.URL.String()]++
		}

		for url, count := range urlsGet {
			require.Equal(t, toEachBackReq, count, "requests to %s should be %d", url, toEachBackReq)
		}

	})

	balancer.RemoveBackend(backs[0])

	t.Run("removed backend doesn't return", func(t *testing.T) {
		urlsGet := make(map[string]int)

		toEachBackReq := 10
		for i := 0; i < toEachBackReq*len(backs); i++ {
			b, err := balancer.GetBack()
			require.NoError(t, err)

			urlsGet[b.URL.String()]++
		}

		require.Equal(t, 0, urlsGet[backs[0].URL.String()])

	})

	t.Run("uniformly after removed", func(t *testing.T) {
		urlsGet := make(map[string]int)

		toEachBackReq := 10
		for i := 0; i < toEachBackReq*(len(backs)-1); i++ {
			b, err := balancer.GetBack()
			require.NoError(t, err)

			urlsGet[b.URL.String()]++
		}

		for url, count := range urlsGet {
			require.Equal(t, toEachBackReq, count, "requests to %s should be %d", url, toEachBackReq)
		}
	})

	t.Run("service unavailable after all removed", func(t *testing.T) {
		for _, b := range backs[1:] {
			balancer.RemoveBackend(b)
		}

		_, err := balancer.GetBack()
		require.ErrorIs(t, err, ServiceUnavailable)
	})

}
