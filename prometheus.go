package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	PidLabel = "exe"
)

type MetricsCollector struct {
	TcpSourceEndpointsCount      *prometheus.GaugeVec
	TcpDestinationEndpointsCount *prometheus.GaugeVec
	ReadFromFDCount              *prometheus.GaugeVec
	WrittenToFDCount             *prometheus.GaugeVec
	ReadFromFDBytes              *prometheus.GaugeVec
	WritenToFDBytes              *prometheus.GaugeVec
	BlockIOSectors               *prometheus.GaugeVec
	BlockIOTimeNanos             *prometheus.GaugeVec
}

func NewMetricsCollector() *MetricsCollector {
	labels := []string{PidLabel}
	ret := &MetricsCollector{
		TcpSourceEndpointsCount:      prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_tcp_src_endpoint_count"}, labels),
		TcpDestinationEndpointsCount: prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_tcp_dest_endpoint_count"}, labels),
		ReadFromFDCount:              prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_fd_in_read_count"}, labels),
		WrittenToFDCount:             prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_fd_in_write_count"}, labels),
		ReadFromFDBytes:              prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_fd_read_bytes"}, labels),
		WritenToFDBytes:              prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_fd_written_bytes"}, labels),
		BlockIOSectors:               prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_block_io_sector_count"}, labels),
		BlockIOTimeNanos:             prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "procshave_block_io_duration_nanos"}, labels),
	}
	for _, metric := range []prometheus.Collector{
		ret.TcpSourceEndpointsCount,
		ret.TcpDestinationEndpointsCount,
		ret.ReadFromFDCount,
		ret.WrittenToFDCount,
		ret.ReadFromFDBytes,
		ret.WritenToFDBytes,
		ret.BlockIOSectors,
		ret.BlockIOTimeNanos,
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
