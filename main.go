package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	reg := prometheus.NewRegistry()
	foo, err := NewCommitTimeCollector()
	if err != nil {
		fmt.Printf("can't find openshift cluster: %s", err)
		return
	}
	reg.MustRegister(foo)

	http.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	log.Fatal(http.ListenAndServe(":9101", nil))
}
