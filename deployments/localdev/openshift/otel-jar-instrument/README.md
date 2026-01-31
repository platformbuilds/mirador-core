# OpenTelemetry Java Agent - Installation Guide

This guide shows how to add the [OpenTelemetry Java Agent](https://github.com/open-telemetry/opentelemetry-java-instrumentation) to your existing Java application on OpenShift.

The agent provides **automatic instrumentation** - no code changes required.

## Prerequisites

- OpenShift cluster with your Java application deployed
- OTel Collector deployed (see `../otel-collector/`)

## Installation Steps

### Step 1: Add Init Container and Volume

Add the following to your existing Deployment:

```yaml
spec:
  template:
    spec:
      # ADD THIS: Init container to download the agent
      initContainers:
        - name: otel-agent-init
          image: registry.access.redhat.com/ubi9/ubi-minimal:latest
          command:
            - sh
            - -c
            - |
              curl -fsSL -o /otel/opentelemetry-javaagent.jar \
                https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/download/v2.11.0/opentelemetry-javaagent.jar
          volumeMounts:
            - name: otel-agent
              mountPath: /otel
          resources:
            limits:
              cpu: 200m
              memory: 128Mi

      # ADD THIS: Volume for the agent JAR
      volumes:
        - name: otel-agent
          emptyDir: {}
```

### Step 2: Add Volume Mount and Environment Variables

Update your application container:

```yaml
containers:
  - name: your-java-app          # Your existing container
    image: your-image:tag        # Your existing image
    
    # ADD THIS: Mount the agent JAR
    volumeMounts:
      - name: otel-agent
        mountPath: /otel
        readOnly: true
    
    # ADD THIS: Environment variables
    env:
      # Attach the agent to JVM
      - name: JAVA_TOOL_OPTIONS
        value: "-javaagent:/otel/opentelemetry-javaagent.jar"
      
      # Your service name (REQUIRED - change this)
      - name: OTEL_SERVICE_NAME
        value: "your-service-name"
      
      # OTel Collector endpoint
      - name: OTEL_EXPORTER_OTLP_ENDPOINT
        value: "http://otel-collector.mirador-observability.svc.cluster.local:4317"
```

### Step 3: Deploy

```bash
oc apply -f your-deployment.yaml
```

---

## Complete Example

Here's a complete before/after example:

### Before (without instrumentation)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-java-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-java-app
  template:
    metadata:
      labels:
        app: my-java-app
    spec:
      containers:
        - name: app
          image: my-registry/my-java-app:1.0.0
          ports:
            - containerPort: 8080
```

### After (with instrumentation)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-java-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-java-app
  template:
    metadata:
      labels:
        app: my-java-app
    spec:
      # ┌─────────────────────────────────────────────────────────┐
      # │ NEW: Init container to download the agent               │
      # └─────────────────────────────────────────────────────────┘
      initContainers:
        - name: otel-agent-init
          image: registry.access.redhat.com/ubi9/ubi-minimal:latest
          command:
            - sh
            - -c
            - |
              curl -fsSL -o /otel/opentelemetry-javaagent.jar \
                https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/download/v2.11.0/opentelemetry-javaagent.jar
          volumeMounts:
            - name: otel-agent
              mountPath: /otel
          resources:
            limits:
              cpu: 200m
              memory: 128Mi

      containers:
        - name: app
          image: my-registry/my-java-app:1.0.0
          ports:
            - containerPort: 8080
          
          # ┌─────────────────────────────────────────────────────┐
          # │ NEW: Environment variables for the agent            │
          # └─────────────────────────────────────────────────────┘
          env:
            - name: JAVA_TOOL_OPTIONS
              value: "-javaagent:/otel/opentelemetry-javaagent.jar"
            - name: OTEL_SERVICE_NAME
              value: "my-java-app"
            - name: OTEL_EXPORTER_OTLP_ENDPOINT
              value: "http://otel-collector.mirador-observability.svc.cluster.local:4317"
          
          # ┌─────────────────────────────────────────────────────┐
          # │ NEW: Volume mount for the agent JAR                 │
          # └─────────────────────────────────────────────────────┘
          volumeMounts:
            - name: otel-agent
              mountPath: /otel
              readOnly: true

      # ┌─────────────────────────────────────────────────────────┐
      # │ NEW: Volume definition                                  │
      # └─────────────────────────────────────────────────────────┘
      volumes:
        - name: otel-agent
          emptyDir: {}
```

---

## Quick Patch for Existing Deployments

If you prefer using `oc` commands instead of editing YAML:

```bash
# 1. Patch to add init container and volume
oc patch deployment YOUR_DEPLOYMENT -n YOUR_NAMESPACE --type='json' -p='[
  {
    "op": "add",
    "path": "/spec/template/spec/initContainers",
    "value": [{
      "name": "otel-agent-init",
      "image": "registry.access.redhat.com/ubi9/ubi-minimal:latest",
      "command": ["sh", "-c", "curl -fsSL -o /otel/opentelemetry-javaagent.jar https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases/download/v2.11.0/opentelemetry-javaagent.jar"],
      "volumeMounts": [{"name": "otel-agent", "mountPath": "/otel"}]
    }]
  },
  {
    "op": "add",
    "path": "/spec/template/spec/volumes",
    "value": [{"name": "otel-agent", "emptyDir": {}}]
  }
]'

# 2. Add volume mount to your container (replace CONTAINER_INDEX with 0, 1, etc.)
oc patch deployment YOUR_DEPLOYMENT -n YOUR_NAMESPACE --type='json' -p='[
  {
    "op": "add",
    "path": "/spec/template/spec/containers/0/volumeMounts/-",
    "value": {"name": "otel-agent", "mountPath": "/otel", "readOnly": true}
  }
]'

# 3. Add environment variables
oc set env deployment/YOUR_DEPLOYMENT -n YOUR_NAMESPACE \
  JAVA_TOOL_OPTIONS="-javaagent:/otel/opentelemetry-javaagent.jar" \
  OTEL_SERVICE_NAME="your-service-name" \
  OTEL_EXPORTER_OTLP_ENDPOINT="http://otel-collector.mirador-observability.svc.cluster.local:4317"
```

---

## Verify Installation

### 1. Check the agent was downloaded

```bash
oc exec -it $(oc get pod -l app=YOUR_APP -o jsonpath='{.items[0].metadata.name}') \
  -- ls -la /otel/
```

Expected output:
```
-rw-r--r-- 1 root root 21234567 Jan 27 12:00 opentelemetry-javaagent.jar
```

### 2. Check the agent is attached

```bash
oc logs $(oc get pod -l app=YOUR_APP -o jsonpath='{.items[0].metadata.name}') | grep -i opentelemetry
```

Expected output:
```
[otel.javaagent] opentelemetry-javaagent - version: 2.11.0
```

### 3. Check traces are being sent

```bash
oc logs -n mirador-observability -l app.kubernetes.io/name=otel-collector | grep -i "traces"
```

---

## Optional: Additional Configuration

### Add Kubernetes Metadata to Traces

```yaml
env:
  - name: JAVA_TOOL_OPTIONS
    value: "-javaagent:/otel/opentelemetry-javaagent.jar"
  - name: OTEL_SERVICE_NAME
    value: "my-java-app"
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "http://otel-collector.mirador-observability.svc.cluster.local:4317"
  
  # Add these for K8s metadata
  - name: K8S_NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace
  - name: K8S_POD_NAME
    valueFrom:
      fieldRef:
        fieldPath: metadata.name
  - name: OTEL_RESOURCE_ATTRIBUTES
    value: "k8s.namespace.name=$(K8S_NAMESPACE),k8s.pod.name=$(K8S_POD_NAME)"
```

### Reduce Sampling for Production

```yaml
env:
  # ... other env vars ...
  
  # Sample only 10% of traces
  - name: OTEL_TRACES_SAMPLER
    value: "parentbased_traceidratio"
  - name: OTEL_TRACES_SAMPLER_ARG
    value: "0.1"
```

### Enable Debug Logging

```yaml
env:
  # ... other env vars ...
  - name: OTEL_JAVAAGENT_DEBUG
    value: "true"
```

---

## Shipping Spring Boot Logback Logs to VictoriaLogs

The OTel Java Agent **automatically instruments Logback** and can ship logs to VictoriaLogs via the OTel Collector.

### Option A: Using the Java Agent (Recommended)

The Java agent already captures Logback logs. Just ensure `OTEL_LOGS_EXPORTER` is set:

```yaml
env:
  - name: JAVA_TOOL_OPTIONS
    value: "-javaagent:/otel/opentelemetry-javaagent.jar"
  - name: OTEL_SERVICE_NAME
    value: "my-spring-app"
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "http://otel-collector.mirador-observability.svc.cluster.local:4317"
  
  # Enable logs export (this is the key setting)
  - name: OTEL_LOGS_EXPORTER
    value: "otlp"
```

That's it! All Logback logs will be shipped to VictoriaLogs.

### Option B: Using OpenTelemetry Logback Appender (More Control)

If you want more control or aren't using the Java agent, add the OTel Logback appender.

#### Step 1: Add Maven Dependency

```xml
<dependency>
    <groupId>io.opentelemetry.instrumentation</groupId>
    <artifactId>opentelemetry-logback-appender-1.0</artifactId>
    <version>2.11.0-alpha</version>
</dependency>
```

Or Gradle:
```groovy
implementation 'io.opentelemetry.instrumentation:opentelemetry-logback-appender-1.0:2.11.0-alpha'
```

#### Step 2: Configure logback-spring.xml

Create or update `src/main/resources/logback-spring.xml`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<configuration>
    <!-- Console appender for local viewing -->
    <appender name="CONSOLE" class="ch.qos.logback.core.ConsoleAppender">
        <encoder>
            <pattern>%d{yyyy-MM-dd HH:mm:ss.SSS} [%thread] %-5level %logger{36} - %msg%n</pattern>
        </encoder>
    </appender>

    <!-- OpenTelemetry appender - ships logs to OTel Collector -->
    <appender name="OTEL" class="io.opentelemetry.instrumentation.logback.appender.v1_0.OpenTelemetryAppender">
        <captureExperimentalAttributes>true</captureExperimentalAttributes>
        <captureCodeAttributes>true</captureCodeAttributes>
        <captureMarkerAttribute>true</captureMarkerAttribute>
        <captureKeyValuePairAttributes>true</captureKeyValuePairAttributes>
        <captureMdcAttributes>*</captureMdcAttributes>
    </appender>

    <root level="INFO">
        <appender-ref ref="CONSOLE"/>
        <appender-ref ref="OTEL"/>
    </root>
</configuration>
```

#### Step 3: Add application.yaml Configuration

```yaml
# application.yaml
otel:
  exporter:
    otlp:
      endpoint: http://otel-collector.mirador-observability.svc.cluster.local:4317
  service:
    name: my-spring-app
  logs:
    exporter: otlp
```

#### Step 4: Environment Variables in Deployment

```yaml
env:
  - name: OTEL_SERVICE_NAME
    value: "my-spring-app"
  - name: OTEL_EXPORTER_OTLP_ENDPOINT
    value: "http://otel-collector.mirador-observability.svc.cluster.local:4317"
  - name: OTEL_EXPORTER_OTLP_PROTOCOL
    value: "grpc"
  - name: OTEL_LOGS_EXPORTER
    value: "otlp"
```

### Verify Logs are Being Shipped

1. Check application logs show OTel initialization:
   ```bash
   oc logs POD_NAME | grep -i "opentelemetry\|otel"
   ```

2. Check OTel Collector is receiving logs:
   ```bash
   oc logs -n mirador-observability -l app.kubernetes.io/name=otel-collector | grep -i "logs"
   ```

3. Query VictoriaLogs:
   ```bash
   curl "http://192.168.80.173:9429/select/logsql/query?query=service.name:my-spring-app"
   ```

### Adding Context to Logs (MDC)

Add trace context to logs for correlation:

```java
import org.slf4j.MDC;

// In your code
MDC.put("userId", userId);
MDC.put("requestId", requestId);
log.info("Processing request");
MDC.clear();
```

These MDC attributes will be captured and shipped with the logs.

### Log Level Filtering

To reduce log volume, filter by level in logback-spring.xml:

```xml
<appender name="OTEL" class="io.opentelemetry.instrumentation.logback.appender.v1_0.OpenTelemetryAppender">
    <filter class="ch.qos.logback.classic.filter.ThresholdFilter">
        <level>WARN</level>  <!-- Only ship WARN and above -->
    </filter>
</appender>
```

---

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Agent JAR not found | Check init container logs: `oc logs POD -c otel-agent-init` |
| Agent not loading | Verify `JAVA_TOOL_OPTIONS` is set: `oc exec POD -- env \| grep JAVA_TOOL_OPTIONS` |
| No traces appearing | Enable debug: `OTEL_JAVAAGENT_DEBUG=true`, check collector logs |
| No logs appearing | Verify `OTEL_LOGS_EXPORTER=otlp` is set |
| App starts slower | Normal - agent adds 3-10s startup time. Increase `initialDelaySeconds` on probes |
| Higher memory usage | Normal - agent adds ~50-100MB. Increase container memory limits |

---

## Supported Frameworks

The agent automatically instruments 100+ libraries including:

- **Web**: Spring Boot, JAX-RS, Servlet, Netty
- **Database**: JDBC, Hibernate, MongoDB, Redis
- **Messaging**: Kafka, RabbitMQ, JMS
- **HTTP Clients**: Apache HttpClient, OkHttp

Full list: [Supported Libraries](https://github.com/open-telemetry/opentelemetry-java-instrumentation/blob/main/docs/supported-libraries.md)

---

## Links

- [OpenTelemetry Java Agent GitHub](https://github.com/open-telemetry/opentelemetry-java-instrumentation)
- [Configuration Reference](https://opentelemetry.io/docs/languages/java/automatic/configuration/)
- [Releases](https://github.com/open-telemetry/opentelemetry-java-instrumentation/releases)
