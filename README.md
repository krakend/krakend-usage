# KrakenD usage module
An anonymous software usage reporter with proof of work.

The KrakenD usage is designed to collect **anonymous information** from any software and push it to a server, but it was built for KrakenD. [Read the blog post](https://www.krakend.io/blog/building-a-telemetry-service/). You can use it with the following code in any app:

```go
	if err := Report(
		ctx,
		Options{
			ClusterID:       "clusterId",
			ServerID:        "serverId",
			URL:             "https://my.usage.api.tld",
      Version:         "v1.2.3",
			ExtraPayload:    someExtraPayload,
			ReportLapse:     12 * time.Hour,
			UserAgent:       "foo bar",
			ReportEndpoint:  "/report",
			SessionEndpoint: "/session",
		},
		nil,
	); err != nil {
		t.Error(err)
		return
	}
```
From the options above, you must implement at least the `ClusterID`, `ServerID`, and `URL`. We recommend using an `uuid` randomly-generated for the IDs. All the options are [documented here](https://github.com/krakendio/krakend-usage/blob/dev-v2/usage.go#L28-L57).

The processing server for this report is out of the project's scope, but [you can get inspiration from the tests](https://github.com/krakendio/krakend-usage/blob/dev-v2/reporter_test.go#L42-L105).

On KrakenD API gateway, the module is entirely disabled by setting an environment var `USAGE_DISABLE=1`, but you should decide your strategy to disable it in your application.

## Note on v2
The v2 API is not compatible with the previous version, although is pretty similar. Please check the new interface for the required changes.
