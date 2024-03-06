package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	PidLabel      = "pid"
	HostnameLabel = "hostname"
)

type MetricsCollector struct {
	TcpSourceEndpointsCount      *prometheus.GaugeVec
	TcpSourceTrafficBytes        *prometheus.GaugeVec
	TcpDestinationEndpointsCount *prometheus.GaugeVec
	TcpDestinationTrafficBytes   *prometheus.GaugeVec
	ReadFromFDCount              *prometheus.GaugeVec
	WrittenToFDCount             *prometheus.GaugeVec
	ReadFromFDBytes              *prometheus.GaugeVec
	WrittenToFDBytes             *prometheus.GaugeVec
	BlockIOSectors               *prometheus.GaugeVec
	BlockIOTimeMillis            *prometheus.GaugeVec
}

func NewMetricsCollector() *MetricsCollector {
	labels := []string{PidLabel, HostnameLabel}
	ret := &MetricsCollector{
		TcpSourceEndpointsCount:      prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_tcp_src_endpoint_count"}, labels),
		TcpSourceTrafficBytes:        prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_tcp_src_traffic_bytes"}, labels),
		TcpDestinationEndpointsCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_tcp_dest_endpoint_count"}, labels),
		TcpDestinationTrafficBytes:   prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_tcp_dest_traffic_bytes"}, labels),
		ReadFromFDCount:              prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_fd_in_read_count"}, labels),
		WrittenToFDCount:             prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_fd_in_write_count"}, labels),
		ReadFromFDBytes:              prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_fd_read_bytes"}, labels),
		WrittenToFDBytes:             prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_fd_written_bytes"}, labels),
		BlockIOSectors:               prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_block_io_sector_count"}, labels),
		BlockIOTimeMillis:            prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_block_io_duration_millis"}, labels),
	}
	for _, metric := range []prometheus.Collector{
		ret.TcpSourceEndpointsCount,
		ret.TcpSourceTrafficBytes,
		ret.TcpDestinationEndpointsCount,
		ret.TcpDestinationTrafficBytes,
		ret.ReadFromFDCount,
		ret.WrittenToFDCount,
		ret.ReadFromFDBytes,
		ret.WrittenToFDBytes,
		ret.BlockIOSectors,
		ret.BlockIOTimeMillis,
	} {
		if err := prometheus.Register(metric); err != nil {
			panic(err)
		}
	}
	return ret
}

func (metrics *MetricsCollector) Start(address string) error {
	mux := http.NewServeMux()
	handler := promhttp.InstrumentMetricHandler(prometheus.DefaultRegisterer, promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))
	mux.Handle("/procshave-metrics", handler)
	return http.ListenAndServe(address, mux)
}
