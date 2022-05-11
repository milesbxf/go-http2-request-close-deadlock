package go_http2_request_close_deadlock

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// We force HTTP/1.1 by using the http scheme
// We expect this test to hang (since the body hangs forever)
// request 1 should never succeed, but we expect request 2 to succeed
func TestHttp1Succeeds(t *testing.T) {
	runTest(t, "http://httpbin.org/")
}

// We force HTTP/2 by using the https scheme
// We expect this test to hang (since the body hangs forever)
// request 1 should never succeed, and request 2 should also get stuck
// if you get a runtime trace or stick this in a debugger, you'll see that request 2 is stuck on the http2 mutex
func TestHttp2Deadlocks(t *testing.T) {
	runTest(t, "https://httpbin.org/")
}

func runTest(t *testing.T, baseURL string) {
	wg := sync.WaitGroup{}

	rt := &http.Transport{}

	wg.Add(1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		t.Log("Started context deadline")

		body := blockingReadCloser{
			R: strings.NewReader("Hello World"),
		}

		req1, err := http.NewRequest("GET", baseURL+"/delay/10", body)
		require.NoError(t, err)

		req1.Header.Set("Content-Length", "226400")
		req1.ContentLength = 226400

		t.Log("Making request 1 to /delay/10")
		_, err = rt.RoundTrip(req1.WithContext(ctx))
		if assert.Error(t, err) {
			assert.Equal(t, context.DeadlineExceeded.Error(), err.Error())
		}
		t.Log("Request 1 done")
		wg.Done()
	}()

	wg.Add(1)
	go func() {
		time.Sleep(time.Second * 2)
		req2, err := http.NewRequest("GET", baseURL, nil)
		require.NoError(t, err)

		t.Log("Making request 2")
		_, err = rt.RoundTrip(req2)
		require.NoError(t, err)
		t.Log("Request 2 done")
		wg.Done()
	}()

	wg.Wait()
}

type blockingReadCloser struct {
	R io.Reader
}

func (b blockingReadCloser) Read(p []byte) (n int, err error) {
	time.Sleep(time.Second)
	return b.R.Read(p)
}

func (b blockingReadCloser) Close() error {
	// block forever
	for {
	}
}
