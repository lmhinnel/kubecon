# Safer deploy in k8s 

## flow
```mermaid
flowchart TB
 subgraph s1["k8s cluster"]
    direction TD
        n2["kyverno"]
        n3["ClusterPolicy<br>check signed image?"]
        n4["pod"]
  end
    A["image"] -- "1. push" --> B["harbor"]
    B -- "2. trigger trivy" --> B
    C{"severity &lt; critical"} -- yes --> D["sign image"]
    B -- "3. webhook on scan_completed" ----> n1["cosign-server"]
    n1 -- "4. check image" --> C
    D -- save --> B
    n2 --> n3
    n2 -- get image --> n4
    n2 -.- B
```

## solutions

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
kind create cluster --config manifests/kind-config.yaml

cloud-provider-kind # 172.18.0.5

# /etc/hosts
# 172.18.0.5 core.harbor.domain
# 172.18.0.5 webhook.harbor.domain

helm repo add containeroo https://charts.containeroo.ch
helm upgrade --install local-path-provisioner containeroo/local-path-provisioner --version 0.0.36 --set storageClass.defaultClass=true

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
helm upgrade --install kyverno kyverno/kyverno --version 3.7.1 -n kyverno --create-namespace --set features.registryClient.allowInsecure=true
k apply -f manifests/07-kyverno-policy.yaml
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
```