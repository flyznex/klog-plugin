#! /bin/sh
rm -rf out
DOCKER_BUILDKIT=1 docker build . -f Dockerfile --output out 