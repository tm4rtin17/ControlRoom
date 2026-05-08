# ControlRoom — in-cluster K8s deployment (Phase A)

Runs the ControlRoom admin UI as a Pod inside the K3s cluster it manages,
using in-cluster RBAC (read-only, Phase A) instead of a mounted kubeconfig.

## Image availability

**Option A — import local build into k3s containerd (default):**
```
docker save controlroom:dev | sudo k3s ctr images import -
```
The manifests default to `image: controlroom:dev` with `imagePullPolicy: IfNotPresent`.

**Option B — pull from GHCR:**
Edit `deployment.yaml` and set:
```yaml
image: ghcr.io/tm4rtin17/controlroom:v0.2
imagePullPolicy: IfNotPresent
```

## Apply order

```
kubectl apply -f deploy/k8s/namespace.yaml \
              -f deploy/k8s/rbac.yaml \
              -f deploy/k8s/pvc.yaml \
              -f deploy/k8s/deployment.yaml \
              -f deploy/k8s/service.yaml \
              -f deploy/k8s/ingress.yaml
```

Namespace and RBAC must exist before the Deployment so the SA binding resolves
on first pod schedule.

## Verify

```
kubectl -n controlroom get all
kubectl -n controlroom logs deployment/controlroom -f
kubectl -n controlroom describe pod -l app.kubernetes.io/name=controlroom
```

## RBAC scope

Phase A grants `get/list/watch` on nodes, namespaces, pods, services,
configmaps, events, deployments, statefulsets, daemonsets, and replicasets.
No write verbs. No access to secrets or persistent volumes.
Phase B/C will widen the ClusterRole to cover metrics-server, CRDs, and
targeted write operations (scale, restart). Phase D adds Helm/Kustomize.

## Notes

- Replicas is hard-coded to 1. ControlRoom stores its JWT key, SQLite DB, and
  TLS material on the PVC. Scaling to >1 requires an external DB (Phase C).
- TLS is currently self-signed (CR_TLS_MODE=selfsigned). To switch to ACME,
  set CR_TLS_MODE=acme + CR_ACME_HOST + CR_ACME_EMAIL in deployment.yaml and
  add a cert-manager ClusterIssuer annotation in ingress.yaml.
- The pod does not mount docker.sock or the host kubeconfig. All cluster access
  flows through the projected ServiceAccount token at the standard in-cluster
  path (/var/run/secrets/kubernetes.io/serviceaccount/).
