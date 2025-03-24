package main

import (
	"fmt"
	"log"
	"net/http"
	// "os"
	// "path"
	"strconv"
	"strings"

	klog "github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/go-redis/redis"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/promlog"
	"github.com/prometheus/common/promlog/flag"
	"github.com/prometheus/common/version"
	"github.com/spf13/viper"

	// webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
	"github.com/alecthomas/kingpin/v2"
)

var (
	listenAddress = kingpin.Flag(
		"listen-address",
		"Address to listen on for web interface and telemetry.",
	).Default(":9104").String()
	metricPath = kingpin.Flag(
		"metrics-path",
		"Path under which to expose metrics.",
	).Default("/metrics").String()
	configMycnf = kingpin.Flag(
		"config",
		"Path to my.yaml file to read MySQL credentials from.",
	).Default("redis.yaml").String()
	cfgFile string
)

type Config struct {
	Name string
}

// 读取配置
func (c *Config) InitConfig() error {
	if c.Name != "" {
		viper.SetConfigFile(c.Name)
	} else {
		viper.AddConfigPath("./")
		viper.SetConfigName("redis")
	}
	viper.SetConfigType("yaml")

	// 从环境变量中读取
	viper.AutomaticEnv()
	// viper.SetEnvPrefix("web")
	viper.SetEnvKeyReplacer(strings.NewReplacer("_", "."))

	return viper.ReadInConfig()
}

// 初始化配置
func initConfig() {
	c := Config{
		Name: cfgFile,
	}

	if err := c.InitConfig(); err != nil {
		panic(err)
	}
	fmt.Println("载入配置成功")
}

// TextToMap 将文本内容转换为map
func TextToMap(text string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		// 跳过空行
		if line == "" {
			continue
		}
		// 分割键值对
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		result[key] = value
	}
	return result
}

// type metrics struct {
// 	cpuTemp    prometheus.Gauge
// 	hdFailures *prometheus.CounterVec
// }

type clusterMetrics struct {
	cluster_state          prometheus.Gauge
	cluster_slots_assigned prometheus.Gauge
	cluster_slots_ok       prometheus.Gauge
	cluster_slots_pfail    prometheus.Gauge
	cluster_slots_fail     prometheus.Gauge
	cluster_known_nodes    prometheus.Gauge
	cluster_size           prometheus.Gauge
	// cluster_size           *prometheus.CounterVec
}

// func NewMetrics(reg prometheus.Registerer) *metrics {
// 	m := &metrics{
// 		cpuTemp: prometheus.NewGauge(prometheus.GaugeOpts{
// 			Name: "cpu_temperature_celsius",
// 			Help: "Current temperature of the CPU.",
// 		}),
// 		hdFailures: prometheus.NewCounterVec(
// 			prometheus.CounterOpts{
// 				Name: "hd_errors_total",
// 				Help: "Number of hard-disk errors.",
// 			},
// 			[]string{"device"},
// 		),
// 	}
// 	reg.MustRegister(m.cpuTemp)
// 	reg.MustRegister(m.hdFailures)
// 	return m
// }

func NewClusterMetrics(reg prometheus.Registerer) *clusterMetrics {
	m := &clusterMetrics{
		cluster_state: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "redis_cluster_state",
			Help: "redis cluster current state ok or not",
		}),
		cluster_slots_assigned: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "redis_cluster_slots_assigned",
			Help: "分配的槽位数",
		}),
		cluster_slots_ok: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "redis_cluster_slots_ok",
			Help: "不在FAIL或PFAIL状态槽位数",
		}),
		cluster_slots_pfail: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "redis_cluster_slots_pfail",
			Help: "在PFAIL状态槽位数",
		}),
		cluster_slots_fail: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "redis_cluster_slots_fail",
			Help: "在FAIL状态槽位数",
		}),
		cluster_known_nodes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "redis_cluster_known_nodes",
			Help: "the number of node",
		}),
		cluster_size: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "redis_cluster_size",
			Help: "主节点数",
		}),
	}
	reg.MustRegister(m.cluster_state)
	reg.MustRegister(m.cluster_slots_assigned)
	reg.MustRegister(m.cluster_slots_ok)
	reg.MustRegister(m.cluster_slots_pfail)
	reg.MustRegister(m.cluster_slots_fail)
	reg.MustRegister(m.cluster_known_nodes)
	reg.MustRegister(m.cluster_size)
	return m
}

func GetRedisClusterStateMetrics(addr, pwd string, logger klog.Logger) *prometheus.Registry {

	reg := prometheus.NewRegistry()
	m := NewClusterMetrics(reg)

	// 创建 Redis 客户端
	conn := redis.NewClient(&redis.Options{
		Addr:     addr, // Redis 地址，可以是本地或者远程
		Password: pwd,  // 如果没有设置密码，可以为空
		// DB:         0,    // 使用默认的 DB
		MaxRetries: 3,
	})

	ok, err := conn.Ping().Result()
	level.Info(logger).Log("Ping 返回值: %s", ok)
	if err != nil || ok != "PONG" {
		level.Error(logger).Log("redis连接失败. redis地址: %s. 错误信息: %s %s", addr, ok, err)
	}
	level.Info(logger).Log("redis连接成功, redis地址: %s", addr)

	val, err := conn.Do("CLUSTER", "INFO").Result()
	if err == redis.Nil || err != nil || val.(string) == "" {
		level.Error(logger).Log("获取redis state 地址: %s. 错误信息: %s", addr, err)
		return reg
	}

	infoMap := TextToMap(val.(string))
	m.cluster_state.Set(0)
	if val, ok := infoMap["cluster_state"]; ok && val == "ok" {
		m.cluster_state.Set(1)
	}
	if val, ok := infoMap["cluster_slots_assigned"]; ok {
		floatValue, _ := strconv.ParseFloat(val, 64)
		m.cluster_slots_assigned.Set(floatValue)
	}
	if val, ok := infoMap["cluster_slots_ok"]; ok {
		floatValue, _ := strconv.ParseFloat(val, 64)
		m.cluster_slots_ok.Set(floatValue)
	}
	if val, ok := infoMap["cluster_slots_pfail"]; ok {
		floatValue, _ := strconv.ParseFloat(val, 64)
		m.cluster_slots_pfail.Set(floatValue)
	}
	if val, ok := infoMap["cluster_slots_fail"]; ok {
		floatValue, _ := strconv.ParseFloat(val, 64)
		m.cluster_slots_fail.Set(floatValue)
	}
	if val, ok := infoMap["cluster_known_nodes"]; ok {
		floatValue, _ := strconv.ParseFloat(val, 64)
		m.cluster_known_nodes.Set(floatValue)
	}
	if val, ok := infoMap["cluster_size"]; ok {
		floatValue, _ := strconv.ParseFloat(val, 64)
		m.cluster_size.Set(floatValue)
	}

	return reg
}

func newHandler(logger klog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()["name"]
		level.Info(logger).Log("msg", "collect[] params", "params", strings.Join(params, ","))

		host := viper.GetStringMapString(fmt.Sprintf("host.%s", params[0]))
		addr := host["ip"] + ":" + host["port"]
		reg := GetRedisClusterStateMetrics(addr, host["pwd"], logger)

		// Delegate http serving to Prometheus client library, which will call collector.Collect.
		h := promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg})
		h.ServeHTTP(w, r)
	}
}

func main() {
	promlogConfig := &promlog.Config{}
	flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(version.Print("redis_cluster_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	logger := promlog.New(promlogConfig)

	cfgFile = *configMycnf
	fmt.Println(cfgFile)
	initConfig()
	// Create a non-global registry.
	// reg := prometheus.NewRegistry()
	// reg := prometheus.NewRegistry()

	// Create new metrics and register them using the custom registry.
	// m := NewClusterMetrics(reg)
	// Set values for the new created metrics.
	// m.hdFailures.With(prometheus.Labels{"device": "/dev/sda"}).Inc()
	// m.cluster_state.Set(1)
	// m.cluster_slots_assigned.Set(16384)
	// m.cluster_slots_ok.Set(16384)
	// m.cluster_slots_pfail.Set(0)
	// m.cluster_slots_fail.Set(0)
	// m.cluster_known_nodes.Set(6)
	// m.cluster_size.Set(3)

	// Expose metrics and custom registry via an HTTP server
	// using the HandleFor function. "/metrics" is the usual endpoint for that.
	// http.Handle(*metricPath, promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	http.Handle(*metricPath, newHandler(logger))
	log.Fatal(http.ListenAndServe(*listenAddress, nil))
}
