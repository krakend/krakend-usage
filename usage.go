package usage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/catalinc/hashcash"
	"github.com/devopsfaith/krakend/core"
)

const (
	defaultURL       = "https://usage.krakend.io"
	HashBits         = 20
	SaltChars        = 40
	DefaultExtension = ""

	sessionEndpoint = "/session"
	reportEndpoint  = "/report"
)

var (
	hasher      = hashcash.New(HashBits, SaltChars, DefaultExtension)
	reportLapse = 12 * time.Hour
)

type SessionRequest struct {
	ClusterID string `json:"cluster_id"`
	ServerID  string `json:"server_id"`
}

type SessionReply struct {
	Token string `json:"token"`
}

type ReportRequest struct {
	Token string    `json:"token"`
	Pow   string    `json:"pow"`
	Data  UsageData `json:"data"`
}

type UsageData struct {
	Version   string         `json:"version"`
	Arch      string         `json:"arch"`
	OS        string         `json:"os"`
	ClusterID string         `json:"cluster_id"`
	ServerID  string         `json:"server_id"`
	Values    map[string]int `json:"values"`
	Time      int64          `json:"time"`
}

func (u *UsageData) Hash() (string, error) {
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(u)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(buf.Bytes())
	return base64.URLEncoding.EncodeToString(sum[:]), nil
}

func (u *UsageData) Expired() bool {
	return time.Since(time.Unix(u.Time, 0)) > time.Minute
}

type ReportReply struct {
	Status  int    `json:"status"`
	Message string `json:"message,omitempty"`
}

type Minter interface {
	Mint(string) (string, error)
}

type UsageClient interface {
	NewSession(context.Context, *SessionRequest) (*SessionReply, error)
	SendReport(context.Context, *ReportRequest) (*ReportReply, error)
}

type HTTPClient interface {
	Send(context.Context, string, interface{}, interface{}) error
}

type Reporter struct {
	client    UsageClient
	clusterID string
	serverID  string
	start     time.Time
	minter    Minter
	token     string
}

func New(url, clusterID, serverID string) (*Reporter, error) {
	if url == "" {
		url = defaultURL
	}

	usageClient := &client{
		HTTPClient: &httpClient{
			c:   &http.Client{},
			URL: url,
		},
		sessionEndpoint: sessionEndpoint,
		reportEndpoint:  reportEndpoint,
	}
	ses, err := usageClient.NewSession(context.Background(), &SessionRequest{
		ClusterID: clusterID,
		ServerID:  serverID,
	})
	if err != nil {
		return nil, err
	}

	r := &Reporter{
		client:    usageClient,
		start:     time.Now(),
		minter:    hasher,
		token:     ses.Token,
		clusterID: clusterID,
		serverID:  serverID,
	}

	return r, nil
}

func (r *Reporter) Report(ctx context.Context) {
	for {
		r.SingleReport()
		select {
		case <-ctx.Done():
			return
		case <-time.After(reportLapse):
		}
	}
}

func (r *Reporter) SingleReport() error {
	ud := UsageData{
		Version:   core.KrakendVersion,
		Arch:      runtime.GOARCH,
		OS:        runtime.GOOS,
		ClusterID: r.clusterID,
		ServerID:  r.serverID,
		Values: map[string]int{
			"num_databases":       3,
			"num_measurements":    2342,
			"num_series":          87232,
			"uptime":              int(time.Since(r.start).Truncate(time.Second).Seconds()),
			"blks_write":          39,
			"blks_write_bytes":    2421,
			"blks_write_bytes_c":  2202,
			"points_write":        39,
			"points_write_dedupe": 39,
		},
		Time: time.Now().Unix(),
	}

	base, err := ud.Hash()
	if err != nil {
		return err
	}
	fmt.Println("client base:", base, r.token)
	pow, err := r.minter.Mint(r.token + base)
	if err != nil {
		return err
	}

	rep, err := r.client.SendReport(context.Background(), &ReportRequest{
		Token: r.token,
		Pow:   pow,
		Data:  ud,
	})
	if err != nil {
		return err
	}
	fmt.Println(rep)
	return nil
}

type client struct {
	HTTPClient
	sessionEndpoint string
	reportEndpoint  string
}

func (c *client) NewSession(ctx context.Context, in *SessionRequest) (*SessionReply, error) {
	reply := &SessionReply{}
	if err := c.Send(ctx, c.sessionEndpoint, in, reply); err != nil {
		return nil, err
	}

	return reply, nil
}

func (c *client) SendReport(ctx context.Context, in *ReportRequest) (*ReportReply, error) {
	reply := &ReportReply{}
	if err := c.Send(ctx, c.reportEndpoint, in, reply); err != nil {
		return nil, err
	}

	return reply, nil
}

type httpClient struct {
	c   *http.Client
	URL string
}

func (c *httpClient) Send(ctx context.Context, path string, in, out interface{}) error {
	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(in); err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.URL+path, buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.c.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
