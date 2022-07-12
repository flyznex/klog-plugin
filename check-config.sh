#!/bin/sh
docker run --rm -v $(pwd)/go.sum:/app/go.sum devopsfaith/krakend:latest check-plugin -g 1.17.11 -s /app/go.sum