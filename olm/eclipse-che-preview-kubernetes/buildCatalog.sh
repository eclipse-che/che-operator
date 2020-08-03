#!/bin/bash

podman build --no-cache -t docker.io/aandrienko/che-operator-catalog:latest .
podman push docker.io/aandrienko/che-operator-catalog:latest 