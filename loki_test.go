package lokiclient

import (
	"github.com/prometheus/client_golang/prometheus"
	"gitlab.forceup.in/xlfs/loki-client/client"
	"testing"
	"time"
)

func TestNewLokiClient(t *testing.T) {
	cfg := NewConfig()
	cfg.Set("URL", "http://192.168.1.65:3100/loki/api/v1/push")
	cfg.Set("LabelKeys", "foo,success")
	m := client.NewMetrics(prometheus.DefaultRegisterer)
	cli, err := NewLokiClient(cfg, newLogger("info"), m)
	if err != nil {
		t.Fatal(err)
	}
	cli.SendRecord(map[interface{}]interface{}{"foo": "bar", "success": "true", "msg": "log line"}, time.Now())
	time.Sleep(3 * time.Second)
}
