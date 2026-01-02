# Agent Runtime Metrics

The agent supports comprehensive metrics collection through a flexible, pluggable architecture that can export to either **Prometheus** or **OpenTelemetry** backends.

## Features

✅ **Dual Backend Support**: Both Prometheus and OpenTelemetry  
✅ **Zero Vendor Lock-in**: Switch backends via configuration  
✅ **Production Ready**: Low overhead, high cardinality protection  
✅ **Comprehensive Coverage**: Runtime, tools, LLM, HTTP metrics  
✅ **Configuration Driven**: Enable/disable via YAML config  

## Architecture

```
Agent Runtime
    ↓
AgentMetrics (high-level wrapper)
    ↓  
Provider Interface (prometheus | otel)
    ↓
Metrics Destination (Prometheus Server | OTEL Collector | etc)
```

## Configuration

### Basic Configuration

```yaml
# In configs/agents/default.yaml
metrics_enabled: true
metrics_config:
  provider: "prometheus"  # or "otel"
  namespace: "agent"
  endpoint: "/metrics"
```

### Prometheus Configuration

```yaml
metrics_config:
  provider: "prometheus"
  namespace: "agent"
  prometheus:
    path: "/metrics"
    registry: "default" 
    labels:
      service: "agent-runtime"
      environment: "production"
      version: "v1.0.0"
```

### OpenTelemetry Configuration

```yaml
metrics_config:
  provider: "otel"
  namespace: "agent"
  otel:
    endpoint: "http://localhost:4317"
    protocol: "grpc"
    export_timeout: 30s
    headers:
      authorization: "Bearer token123"
    resources:
      service.name: "agent-runtime"
      service.version: "1.0.0"
      deployment.environment: "production"
```

## Metrics Catalog

### Run Metrics
| Name | Type | Description | Labels |
|------|------|-------------|---------|
| `agent_runs_created_total` | Counter | Total runs created | `tenant_id`, `mode` |
| `agent_runs_completed_total` | Counter | Total runs completed | `tenant_id`, `mode`, `status` |
| `agent_run_duration_seconds` | Histogram | Run execution duration | `mode`, `status` |
| `agent_runs_active` | Gauge | Currently active runs | `tenant_id` |

### Tool Metrics
| Name | Type | Description | Labels |
|------|------|-------------|---------|
| `agent_tool_invocations_total` | Counter | Total tool invocations | `tool_name`, `server_name`, `status` |
| `agent_tool_duration_seconds` | Histogram | Tool execution duration | `tool_name`, `server_name` |

### LLM Metrics
| Name | Type | Description | Labels |
|------|------|-------------|---------|
| `agent_llm_requests_total` | Counter | Total LLM requests | `provider`, `model`, `status` |
| `agent_llm_request_duration_seconds` | Histogram | LLM request duration | `provider`, `model` |
| `agent_llm_tokens_used_total` | Counter | Total tokens consumed | `provider`, `model`, `token_type` |

### HTTP Metrics
| Name | Type | Description | Labels |
|------|------|-------------|---------|
| `agent_http_requests_total` | Counter | Total HTTP requests | `method`, `path`, `status_code`, `status_class` |
| `agent_http_request_duration_seconds` | Histogram | HTTP request duration | `method`, `path` |

## Deployment Examples

### Docker Compose with Prometheus

```yaml
version: '3.8'
services:
  agentd:
    build: .
    ports:
      - "8080:8080"
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    labels:
      - "prometheus.io/scrape=true" 
      - "prometheus.io/port=8080"
      - "prometheus.io/path=/metrics"
      
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.enable-lifecycle'
```

**prometheus.yml:**
```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'agent'
    static_configs:
      - targets: ['agentd:8080']
    metrics_path: '/metrics'
```

### Kubernetes with OpenTelemetry

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agentd
spec:
  replicas: 3
  selector:
    matchLabels:
      app: agentd
  template:
    metadata:
      labels:
        app: agentd
    spec:
      containers:
      - name: agentd
        image: agentd:latest
        ports:
        - containerPort: 8080
        env:
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: agent-secrets
              key: anthropic-api-key
        volumeMounts:
        - name: config
          mountPath: /app/configs
      volumes:
      - name: config
        configMap:
          name: agent-config
---
apiVersion: v1  
kind: ConfigMap
metadata:
  name: agent-config
data:
  default.yaml: |
    metrics_enabled: true
    metrics_config:
      provider: "otel"
      otel:
        endpoint: "http://otel-collector:4317"
        protocol: "grpc"
        resources:
          k8s.cluster.name: "production"
          k8s.namespace.name: "default"
```

## Monitoring Dashboards

### Prometheus/Grafana Queries

**Run Success Rate:**
```promql
rate(agent_runs_completed_total{status="completed"}[5m]) / 
rate(agent_runs_created_total[5m]) * 100
```

**Tool Error Rate:**
```promql
rate(agent_tool_invocations_total{status="error"}[5m]) /
rate(agent_tool_invocations_total[5m]) * 100
```

**LLM Cost per Hour (estimated):**
```promql
rate(agent_llm_tokens_used_total[1h]) * 0.000001 * 24
```

**95th Percentile Response Time:**
```promql
histogram_quantile(0.95, rate(agent_http_request_duration_seconds_bucket[5m]))
```

## Alerting Examples

```yaml
groups:
- name: agent.rules
  rules:
  - alert: HighRunFailureRate
    expr: rate(agent_runs_completed_total{status="failed"}[5m]) > 0.1
    for: 2m
    labels:
      severity: warning
    annotations:
      summary: "High run failure rate detected"
      
  - alert: LLMProviderDown
    expr: rate(agent_llm_requests_total{status="error"}[5m]) > 0.5
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "LLM provider experiencing high error rate"
```

## Performance Considerations

- **Label Cardinality**: UUIDs and dynamic values are templated to prevent metric explosion
- **Sampling**: Histograms use exponential buckets optimized for typical agent workloads  
- **Buffering**: OpenTelemetry uses periodic export to reduce overhead
- **Memory**: No-op provider available when metrics are disabled

## Troubleshooting

### Metrics Not Appearing

1. Check configuration: `metrics_enabled: true`
2. Verify endpoint accessibility: `curl http://localhost:8080/metrics`
3. Check logs for provider initialization errors
4. For OTEL: Ensure collector is reachable

### High Cardinality Warning

If you see metric explosion:
- Check for dynamic labels (UUIDs, timestamps)
- Review path templating in HTTP metrics
- Consider increasing Prometheus storage retention

### Performance Impact

Metrics add ~1-2% CPU overhead and ~10MB memory baseline. For high-throughput deployments:
- Use sampling for histograms
- Disable detailed metrics via config
- Consider push-based OTEL export
