module redis_cluster_exporter

go 1.16

require (
	github.com/alecthomas/kingpin/v2 v2.4.0
	github.com/go-kit/log v0.2.1
	github.com/prometheus/client_golang v1.20.5
)

require (
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/onsi/gomega v1.34.2 // indirect
	github.com/prometheus/common v0.60.0
	github.com/spf13/viper v1.19.0
)
