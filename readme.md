
```bash
# kubectl apply -f https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
kubectl apply -f tekton.yaml

# kubectl apply -f https://raw.githubusercontent.com/tektoncd/catalog/refs/heads/main/task/buildpacks-phases/0.3/buildpacks-phases.yaml
k apply -f buildpacks-phases.yaml

# kubectl apply -f https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.35/deploy/local-path-storage.yaml
k apply -f local-path-storage.yaml

k create secret docker-registry registry-user-pass --docker-username=lmhinnel --docker-password=dckr_pat_123 --docker-server=https://index.docker.io/v1/ --namespace default
k apply -f pvc.yaml -f sa.yaml -f pipeline.yaml
k apply -f run.yaml

helm repo add knative-operator https://knative.github.io/operator
helm install knative-operator --create-namespace --namespace knative-operator knative-operator/knative-operator

kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.21.2/serving-default-domain.yaml
```