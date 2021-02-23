package client

import (
	"context"

	"github.com/devopsfaith/krakend-usage"
)

type Options struct {
	ClusterID string
	ServerID  string
	URL       string
	Version   string
}

func StartReporter(ctx context.Context, opt Options) error {
	reporter, err := usage.New(opt.URL, opt.ClusterID, opt.ServerID, opt.Version)
	if err != nil {
		return err
	}

	go reporter.Report(ctx)

	return nil
}
