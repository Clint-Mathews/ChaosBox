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
