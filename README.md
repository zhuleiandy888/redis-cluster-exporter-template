# redis-cluster-exporter-template
redis-cluster-exporter-template
### usage help

## run 
redis_cluster_exporter --listen-address=:9104 --metrics-path=/metrics --config=/path/xxx/redis.yaml

## scrape redis data example
curl http://192.168.3.240:9104/metrics?name=test002

## redis config file example
host:
  test001:
    ip: 192.168.3.127
    pwd: ''
    port: 6379
  test002:
    ip: 192.168.2.128
    pwd: xxxxxxx
    port: 6379


## promethues config example
scrape_configs:
  - job_name: 'redis_cluster_exporter'
    scrape_timeout: 10s
    scrape_interval: 30s
    static_configs:
      - targets:
        - '192.168.3.240:9104'
      labels:
        name: test001
        __metrics_path__: /metrics
        __param_name: test001

      - targets: 
        - '192.168.3.240:9104'
      labels:
        name: test002
        __metrics_path__: /metrics
        __param_name: test002
