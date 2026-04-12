#!/bin/bash

# List of images to pull and push to Harbor
IMAGES=(
    "lmhinnel/flappy:latest"
    "golang:1.22-alpine"
    "nginx:latest"
)

# Harbor registry details
HARBOR_REGISTRY="core.harbor.domain"
HARBOR_PROJECT="library"
HARBOR_USERNAME="admin"
HARBOR_PASSWORD="Harbor12345"

# Login to Harbor registry
docker login $HARBOR_REGISTRY -u $HARBOR_USERNAME -p $HARBOR_PASSWORD

# Pull, tag, and push images to Harbor
for IMAGE in "${IMAGES[@]}"; do
    # Pull the image from Docker Hub
    docker pull $IMAGE

    # Tag the image for Harbor
    docker tag $IMAGE $HARBOR_REGISTRY/$HARBOR_PROJECT/$IMAGE

    # Push the image to Harbor
    docker push $HARBOR_REGISTRY/$HARBOR_PROJECT/$IMAGE
done