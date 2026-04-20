# Safer deploy in k8s

## flow

```mermaid
flowchart TB
 subgraph s1["k8s cluster"]
        n2["kyverno"]
        n3["ClusterPolicy<br>check signed image?"]
        n4["apiserver"]
  end
    n5["deploy.yaml"] -- "1. apply" --> n4
    n4 <-- "2. admission<br>=&gt; allow/deny req" --> n2
    n2 -- "3. signed?" --> B
    n2 --> n3
    A["image"] -- "1. store" --> B["harbor"]
    B -- "2. trivy<br/>scan on push" --> B
    C{"severity &lt; critical"} -- yes --> D["sign image"]
    B -- "3. scan_completed<br/>webhook" ----> n1["cosign-server"]
    n1 -- "4. severity?" --> C
    D -- save --> B

    linkStyle 0 stroke:#326CE5,fill:none
    linkStyle 1 stroke:#326CE5,fill:none
    linkStyle 2 stroke:#326CE5,fill:none
    linkStyle 3 stroke:#326CE5,fill:none
    linkStyle 4 stroke:#5A9858,fill:none
    linkStyle 5 stroke:#5A9858,fill:none
    linkStyle 6 stroke:#5A9858,fill:none
    linkStyle 7 stroke:#5A9858,fill:none
    linkStyle 8 stroke:#5A9858,fill:none
    linkStyle 9 stroke:#5A9858,fill:none
```

## solutions

> **[survey.md](survey.md)**

### config webhook in harbor and auto scan with trivy

- webhook
  ![alt text](<assets/Screenshot from 2026-04-12 18-12-17.png>)

- auto scan
  ![alt text](<assets/Screenshot from 2026-04-12 18-14-06.png>)

### pollute harbor with vulnerable images and trivy scan results

- pollute harbor
  ![alt text](<assets/Screenshot from 2026-04-12 17-55-54.png>)

- webhook results
  ![alt text](<assets/Screenshot from 2026-04-12 18-05-05.png>)

- log in webhook
  ![alt text](<assets/Screenshot from 2026-04-12 18-06-06.png>)

- cve details: [csv_file_20260412180800.csv](assets/csv_file_20260412180800.csv)

### opt. 1: Use built-int harbor cosign prevention

- config in harbor UI
  ![alt text](<assets/Screenshot from 2026-04-12 17-17-13.png>)

- result in k8s
  ![alt text](<assets/Screenshot from 2026-04-12 17-01-07.png>)

### opt. 2: Use Kyverno Admission Controller

- more flexible, can customize policy
- e.g. only allow images from specific registry, or only allow signed images with specific key, etc.

![alt text](<assets/Screenshot from 2026-04-12 17-43-48.png>)

## walkthrough

```bash
kind create cluster --config manifests/A/kind-config.yaml
kind create cluster --config manifests/B/kind-config.yaml

cloud-provider-kind # 172.18.0.5

# /etc/hosts
# 172.18.0.5 core.harbor.domain
# 172.18.0.5 webhook.harbor.domain

# helm repo add containeroo https://charts.containeroo.ch
# helm upgrade --install local-path-provisioner containeroo/local-path-provisioner --version 0.0.36 --set storageClass.defaultClass=true

helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx --version 4.15.1

helm repo add harbor https://helm.goharbor.io
helm upgrade --install harbor harbor/harbor --version 1.18.3 --set expose.ingress.className=nginx

k apply -f manifests/deploy
```

### in case opt. 2

```bash
helm repo add kyverno https://kyverno.github.io/kyverno/
helm repo update
k create ns app-ns
helm upgrade --install kyverno kyverno/kyverno --version 3.7.1 -n app-ns -f manifests/values/kyverno.yaml
k apply -f manifests/A/07-kyverno-policy.yaml
```

### test

```bash
chmod +x pollute-harbor.sh
./pollute-harbor.sh

k apply -f manifests/samples
```

### clean up

```bash
kind delete cluster -n meo
kind delete cluster -n app
```

### multi-tenant (bonus)

```bash
helm install cert-manager oci://quay.io/jetstack/charts/cert-manager --version v1.19.2 --namespace cert-manager --create-namespace --set crds.enabled=true

k apply -f https://raw.githubusercontent.com/metallb/metallb/v0.15.3/config/manifests/metallb-native.yaml

sed -E "s|172.19|$(docker network inspect -f '{{range .IPAM.Config}}{{.Gateway}}{{end}}' kind | sed -E 's|^([0-9]+\.[0-9]+)\..*$|\1|g')|g" manifests/A/metalb.yaml | k apply -f -

helm upgrade --install kamaji clastix/kamaji --namespace kamaji-system --create-namespace --set 'resources=null' --version 0.0.0+latest

```
