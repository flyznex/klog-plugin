#!/bin/sh
docker run --rm --name krakend-gw -v $(pwd)/out:/app/plugin -v $(pwd)/krakend.json:/app/krakend.json -p 8080:8080 devopsfaith/krakend:latest run -dc /app/krakend.json 