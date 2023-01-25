package usage

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/catalinc/hashcash"
)

type mockUsageServer struct {
	newSession func(context.Context, *SessionRequest) (*SessionReply, error)
	sendReport func(context.Context, *ReportRequest) (*ReportReply, error)
}

func (t *mockUsageServer) NewSession(c context.Context, s *SessionRequest) (*SessionReply, error) {
	return t.newSession(c, s)
}

func (t *mockUsageServer) SendReport(c context.Context, r *ReportRequest) (*ReportReply, error) {
	return t.sendReport(c, r)
}

func TestReport(t *testing.T) {
	done := make(chan struct{}, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	hasher := hashcash.New(hashBits, saltChars, defaultExtension)

	totReport := int32(5)
	var reports int32
	reportEndpoint := "/r"
	sessionEndpoint := "/s"
	expectedExtraPayload := make([]byte, 1024*1024)
	rand.Read(expectedExtraPayload)

	s := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.URL.String() == sessionEndpoint {
			rw.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(rw, `{"token":"some_token_value"}`)
			return
		}

		if req.URL.String() != reportEndpoint {
			t.Errorf("unknown endpoint %s", req.URL.String())
			http.Error(rw, "not found", 404)
			done <- struct{}{}
			return
		}

		tot := atomic.AddInt32(&reports, 1)

		if tot >= totReport+1 {
			done <- struct{}{}
			return
		}

		var msg ReportRequest
		if err := json.NewDecoder(req.Body).Decode(&msg); err != nil {
			t.Error(err)
		}
		req.Body.Close()

		if !hasher.Check(msg.Pow) {
			t.Errorf("wrong pow: %s", msg.Pow)
		}
		d, err := msg.Data.Hash()
		if err != nil {
			t.Error(err)
		}
		if !strings.Contains(msg.Pow, d) {
			t.Errorf("pow with unexpected hash. have: %s want: %s", msg.Pow, d)
		}
		if msg.Data.Expired() {
			t.Errorf("expired pow. have: %s", time.Unix(msg.Data.Time, 0))
		}

		if tot == 1 {
			if msg.Data.Extra == nil || len(msg.Data.Extra) == 0 {
				t.Error("nil extra")
				http.Error(rw, "crash", 500)
				return
			}

			if !bytes.Equal(msg.Data.Extra, expectedExtraPayload) {
				t.Errorf("unexpected extra payload. have %s", string(msg.Data.Extra))
				http.Error(rw, "crash", 500)
				return
			}
		} else {
			if msg.Data.Extra != nil && len(msg.Data.Extra) != 0 {
				t.Error("unexpected extra payload")
				http.Error(rw, "crash", 500)
				return
			}
		}

		rw.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(rw, `{"status":200}`)
	}))

	defer s.Close()

	<-time.After(100 * time.Millisecond)

	if err := Report(
		ctx,
		Options{
			ClusterID:       "clusterId",
			ServerID:        "serverId",
			URL:             s.URL,
			ExtraPayload:    expectedExtraPayload,
			ReportLapse:     2 * time.Second,
			UserAgent:       "foo bar",
			ReportEndpoint:  reportEndpoint,
			SessionEndpoint: sessionEndpoint,
		},
		nil,
	); err != nil {
		t.Error(err)
		return
	}

	select {
	case <-ctx.Done():
	case <-done:
	}

	if reports != totReport+1 {
		t.Errorf("unexpected number of reports: %d", reports)
	}
}
