package lokiclient

import (
	"github.com/go-kit/log"
	"gitlab.forceup.in/xlfs/loki-client/client"
)

// newClient creates a new client based on the fluentbit configuration.
func newClient(cfg *config, logger log.Logger, metrics *client.Metrics) (client.Client, error) {
	if cfg.bufferConfig.buffer {
		return NewBuffer(cfg, logger, metrics)
	}
	return client.New(metrics, cfg.clientConfig, 0, 0, false, logger)
}
