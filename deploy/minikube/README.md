# Minikube Deployment

The manifests deploy PostgreSQL and the store service into the `chaosbox`
namespace. PostgreSQL data is retained in a persistent volume claim.

## GitHub Image

GitHub Actions publishes main-branch images to:

```text
ghcr.io/clint-mathews/chaosbox-store:latest
```

The GHCR package is public. Docker and Minikube can pull it without registry
credentials or an image pull secret.

Start Minikube and deploy the published image:

```bash
make minikube-start
make minikube-deploy
make minikube-status
make minikube-url
```

## Local Image

Build the Dockerfile directly into Minikube before deploying:

```bash
make minikube-start
make minikube-build
make minikube-deploy
```

The local build uses the same image name as the GitHub image. The
`IfNotPresent` pull policy makes Minikube use the image already loaded into the
cluster.

## Cleanup

Delete the namespace, workloads, and PostgreSQL data:

```bash
make minikube-clean
```

Stop Minikube without deleting the cluster:

```bash
make minikube-stop
```

The credentials in `secret.yaml` are development-only values for the local
Minikube environment.

## Diagnostics

Metrics Server remains enabled separately from Prometheus and powers
`kubectl top`. Use the following targets while running load or failure tests:

```bash
make minikube-top
make minikube-logs
make minikube-db-logs
make minikube-previous-logs
make minikube-events
```

Open an interactive PostgreSQL session inside the Minikube pod:

```bash
make minikube-db-shell
```

Remove generated order and order-item data after a load test:

```bash
make minikube-db-clear
```

The clear target resets the order identity while preserving seeded products,
schema migrations, and the PostgreSQL persistent volume.

Install and open k9s with:

```bash
brew install k9s
make minikube-k9s
```

## Prometheus And Grafana

Install Helm, deploy the application, and then install the monitoring stack:

```bash
brew install helm
make minikube-deploy
make monitoring-install
make monitoring-status
```

The order matters: Helm installs the Prometheus Operator CRDs before the
separate monitoring Kustomization creates the store `ServiceMonitor`.
Application deployment does not depend on those CRDs.

Open Grafana at `http://localhost:3000` using the `admin` user:

```bash
make monitoring-grafana-password
make monitoring-grafana-forward
```

Open Prometheus at `http://localhost:9090`:

```bash
make monitoring-prometheus-forward
```

The query `up{namespace="chaosbox",service="store"}` should return `1`. The
store dashboard is loaded automatically and refreshes every five seconds.

Loki and Grafana Alloy are installed with the monitoring stack. Alloy collects
Kubernetes pod logs and sends them to Loki with a 24-hour retention period.
Open Grafana Explore and select the Loki datasource, or use queries such as:

```logql
{namespace="chaosbox", app="store"} | json
```

```logql
{namespace="chaosbox", app="store"} | json | level="ERROR"
```

Search for one correlated request with:

```logql
{namespace="chaosbox", app="store"} | json | request_id="REQUEST_ID"
```

Loki can also be inspected directly:

```bash
make monitoring-loki-forward
curl --fail http://localhost:3100/ready
```

## Load-Test Dashboard

The `ChaosBox Load Test` dashboard combines request throughput, errors,
latency, resource use, PostgreSQL pool activity, and application logs. Print
its URL with:

```bash
make monitoring-load-dashboard
```

Each `make load-smoke` or `make load-test` run prints a Grafana URL scoped to
the exact test time range. Every generated request carries an `X-Request-ID`,
and failed request IDs are retained in `summary.json` for direct Loki searches.
Keep both the store and Grafana port-forwards running while executing a load
test.

The monitoring stack has explicit resource constraints, but its overhead can
still affect small Minikube experiments. The store's `32Mi` memory limit is
intentionally unchanged so OOM behavior remains visible.

Remove custom monitoring resources before uninstalling the chart:

```bash
make monitoring-clean
```

Helm normally retains Prometheus Operator CRDs. This target leaves those CRDs
in place to avoid deleting shared custom-resource definitions unexpectedly.
It deletes Loki's local persistent volume claim, including retained logs.
