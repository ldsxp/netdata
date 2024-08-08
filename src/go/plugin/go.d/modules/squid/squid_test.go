// SPDX-License-Identifier: GPL-3.0-or-later

package squid

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/netdata/netdata/go/plugins/plugin/go.d/agent/module"
	"github.com/netdata/netdata/go/plugins/plugin/go.d/pkg/web"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	dataConfigJSON, _ = os.ReadFile("testdata/config.json")
	dataConfigYAML, _ = os.ReadFile("testdata/config.yaml")
	dataCounters, _   = os.ReadFile("testdata/counters.txt")
)

func Test_testDataIsValid(t *testing.T) {
	for name, data := range map[string][]byte{
		"dataConfigJSON": dataConfigJSON,
		"dataConfigYAML": dataConfigYAML,
		"dataCounters":   dataCounters,
	} {
		require.NotNil(t, data, name)
	}
}

func TestSquid_ConfigurationSerialize(t *testing.T) {
	module.TestConfigurationSerialize(t, &Squid{}, dataConfigJSON, dataConfigYAML)
}

func TestSquid_Init(t *testing.T) {
	tests := map[string]struct {
		wantFail bool
		config   Config
	}{
		"success with default": {
			wantFail: false,
			config:   New().Config,
		},
		"fail when URL not set": {
			wantFail: true,
			config: Config{
				HTTP: web.HTTP{
					Request: web.Request{URL: ""},
				},
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			squid := New()
			squid.Config = test.config

			if test.wantFail {
				assert.Error(t, squid.Init())
			} else {
				assert.NoError(t, squid.Init())
			}
		})
	}
}

func TestSquid_Charts(t *testing.T) {
	assert.NotNil(t, New().Charts())
}

func TestSquid_Check(t *testing.T) {
	tests := map[string]struct {
		wantFail bool
		prepare  func(t *testing.T) (*Squid, func())
	}{
		"success case": {
			wantFail: false,
			prepare:  prepareCaseSuccess,
		},
		"fails on unexpected response": {
			wantFail: true,
			prepare:  prepareCaseUnexpectedResponse,
		},
		"fails on empty response": {
			wantFail: true,
			prepare:  prepareCaseEmptyResponse,
		},
		"fails on connection refused": {
			wantFail: true,
			prepare:  prepareCaseConnectionRefused,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			squid, cleanup := test.prepare(t)
			defer cleanup()

			if test.wantFail {
				assert.Error(t, squid.Check())
			} else {
				assert.NoError(t, squid.Check())
			}
		})
	}
}

func TestSquid_Collect(t *testing.T) {
	tests := map[string]struct {
		prepare     func(t *testing.T) (*Squid, func())
		wantMetrics map[string]int64
		wantCharts  int
	}{
		"success case": {
			prepare:    prepareCaseSuccess,
			wantCharts: len(charts),
			wantMetrics: map[string]int64{
				"client_http.errors":         5,
				"client_http.hit_kbytes_out": 11,
				"client_http.hits":           1,
				"client_http.kbytes_in":      566,
				"client_http.kbytes_out":     16081,
				"client_http.requests":       9019,
				"server.all.errors":          0,
				"server.all.kbytes_in":       0,
				"server.all.kbytes_out":      0,
				"server.all.requests":        0,
			},
		},
		"fails on unexpected response": {
			prepare: prepareCaseUnexpectedResponse,
		},
		"fails on empty response": {
			prepare: prepareCaseEmptyResponse,
		},
		"fails on connection refused": {
			prepare: prepareCaseConnectionRefused,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			squid, cleanup := test.prepare(t)
			defer cleanup()

			mx := squid.Collect()

			require.Equal(t, test.wantMetrics, mx)

			if len(test.wantMetrics) > 0 {
				assert.Equal(t, test.wantCharts, len(*squid.Charts()))
				module.TestMetricsHasAllChartsDims(t, squid.Charts(), mx)
			}
		})
	}
}

func prepareCaseSuccess(t *testing.T) (*Squid, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case urlPathServerStats:
				_, _ = w.Write(dataCounters)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))

	squid := New()
	squid.URL = srv.URL
	require.NoError(t, squid.Init())

	return squid, srv.Close
}

func prepareCaseUnexpectedResponse(t *testing.T) (*Squid, func()) {
	t.Helper()
	resp := []byte(`
Lorem ipsum dolor sit amet, consectetur adipiscing elit.
Nulla malesuada erat id magna mattis, eu viverra tellus rhoncus.
Fusce et felis pulvinar, posuere sem non, porttitor eros.`)

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(resp))
		}))

	squid := New()
	squid.URL = srv.URL
	require.NoError(t, squid.Init())

	return squid, srv.Close
}

func prepareCaseEmptyResponse(t *testing.T) (*Squid, func()) {
	t.Helper()
	resp := []byte(``)

	srv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(resp))
		}))

	squid := New()
	squid.URL = srv.URL
	require.NoError(t, squid.Init())

	return squid, srv.Close
}

func prepareCaseConnectionRefused(t *testing.T) (*Squid, func()) {
	t.Helper()
	squid := New()
	squid.URL = "http://127.0.0.1:65001"
	require.NoError(t, squid.Init())

	return squid, func() {}
}
