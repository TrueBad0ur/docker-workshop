#!/bin/bash

docker run -p 5000:5000 --rm --name registry arm64v8/registry:2

curl localhost:5000/v2/_catalog

docker build -t localhost:5000/shisha-server:latest -f Dockerfile-server .

docker push localhost:5000/shisha-server:latest

DOCKER_BUILDKIT=1 docker build --build-arg BUILDKIT_INLINE_CACHE=1 --cache-from localhost:5000/shisha-server:latest -t localhost:5000/shisha-server-cached:latest -f Dockerfile-server .