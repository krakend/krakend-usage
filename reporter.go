package usage

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"github.com/catalinc/hashcash"
)

type UsageClient interface {
	NewSession(context.Context, *SessionRequest) (*SessionReply, error)
	SendReport(context.Context, *ReportRequest) (*ReportReply, error)
}

type Reporter struct {
	Options
	Client UsageClient
	Start  time.Time
	Token  string
}

func Report(ctx context.Context, opt Options, c UsageClient) error {
	reporter, err := New(opt, c)
	if err != nil {
		return err
	}

	go reporter.Report(ctx)

	return nil
}

func New(opt Options, c UsageClient) (*Reporter, error) {
	if opt.URL == "" {
		opt.URL = defaultURL
	}
	if opt.HashBits == 0 {
		opt.HashBits = hashBits
	}
	if opt.SaltChars == 0 {
		opt.SaltChars = saltChars
	}
	if opt.Minter == nil {
		opt.Minter = hashcash.New(opt.HashBits, opt.SaltChars, defaultExtension)
	}
	if opt.SessionEndpoint == "" {
		opt.SessionEndpoint = sessionEndpoint
	}
	if opt.ReportEndpoint == "" {
		opt.ReportEndpoint = reportEndpoint
	}
	if opt.Timeout == 0 {
		opt.Timeout = timeout
	}
	if opt.ReportLapse == 0 {
		opt.ReportLapse = reportLapse
	}

	if c == nil {
		httpClient := &http.Client{}
		if opt.Client != nil {
			httpClient = opt.Client
		}
		c = &Client{
			HTTPClient: &BasicClient{
				Client:    httpClient,
				URL:       opt.URL,
				UserAgent: opt.UserAgent,
			},
			SessionEndpoint: opt.SessionEndpoint,
			ReportEndpoint:  opt.ReportEndpoint,
		}
	}

	localCtx, cancel := context.WithTimeout(context.Background(), opt.Timeout)
	defer cancel()

	ses, err := c.NewSession(
		localCtx,
		&SessionRequest{
			ClusterID: opt.ClusterID,
			ServerID:  opt.ServerID,
		},
	)

	return &Reporter{
		Client:  c,
		Start:   time.Now(),
		Options: opt,
		Token:   ses.Token,
	}, err
}

func (r *Reporter) Report(ctx context.Context) {
	tc := time.NewTicker(r.ReportLapse)
	defer tc.Stop()

	for {
		localCtx, cancel := context.WithTimeout(ctx, r.Timeout)
		r.SingleReport(localCtx)
		cancel()

		select {
		case <-ctx.Done():
			return
		case <-tc.C:
		}
	}
}

func (r *Reporter) SingleReport(ctx context.Context) error {
	ud := UsageData{
		Version:   r.Version,
		Arch:      runtime.GOARCH,
		OS:        runtime.GOOS,
		ClusterID: r.ClusterID,
		ServerID:  r.ServerID,
		Uptime:    int64(time.Since(r.Start) / time.Second),
		Time:      time.Now().Unix(),
	}

	if v := int64(r.ReportLapse / time.Second); ud.Uptime < v {
		ud.Extra = r.ExtraPayload
	}

	base, err := ud.Hash()
	if err != nil {
		return err
	}

	pow, err := r.Minter.Mint(r.Token + base)
	if err != nil {
		return err
	}

	_, err = r.Client.SendReport(ctx, &ReportRequest{
		Token: r.Token,
		Pow:   pow,
		Data:  ud,
	})
	return err
}
