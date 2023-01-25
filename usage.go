package usage

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"
)

const (
	defaultURL       = "https://usage.krakend.io"
	defaultExtension = ""

	hashBits  uint = 20
	saltChars uint = 40

	sessionEndpoint = "/session"
	reportEndpoint  = "/report"

	timeout     = 15 * time.Second
	reportLapse = 12 * time.Hour
)

var ReportExpiration = 60 * time.Second

type Options struct {
	// ClusterID identifies the cluster
	ClusterID string
	// ServerID identifies the instance
	ServerID string
	// URL is the base path of the remote service
	URL string
	// Version of the application reporting
	Version string
	// Minter is the default Minter injected into the Reporter
	Minter Minter
	// ExtraPayload is the extra information to send with the first report
	ExtraPayload []byte
	// HashBits are the number of bits of collision
	HashBits uint
	// SaltChars are the number of bytes of salt chars
	SaltChars uint
	//SessionEndpoint is the path of the session endpoint
	SessionEndpoint string
	// ReportEndpoint is the path of the report endpoint
	ReportEndpoint string
	// Timeout is the max duration of every request
	Timeout time.Duration
	// ReportLapse is the waiting time between reports
	ReportLapse time.Duration
	// UserAgent is the value of the user-agent header to send with the requests
	UserAgent string
	// Client is the http client to use. If nil, a new http client will be created
	Client *http.Client
}

type Minter interface {
	Mint(string) (string, error)
}

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

type ReportReply struct {
	Status  int    `json:"status"`
	Message string `json:"message,omitempty"`
}

type UsageData struct {
	Version   string `json:"version"`
	Arch      string `json:"arch"`
	OS        string `json:"os"`
	ClusterID string `json:"cluster_id"`
	ServerID  string `json:"server_id"`
	Uptime    int64  `json:"uptime"`
	Time      int64  `json:"time"`
	Extra     []byte `json:"extra,omitempty"`
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
	return time.Since(time.Unix(u.Time, 0)) > ReportExpiration
}
