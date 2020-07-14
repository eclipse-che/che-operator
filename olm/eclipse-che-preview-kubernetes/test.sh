#!/bin/bash

# opm alpha bundle generate -d eclipse-che-preview-openshift --channels latest --package /home/user/GoWorkSpace/src/github.com/eclipse/che-operator/olm/eclipse-che-preview-kubernetes/deploy/olm-catalog/eclipse-che-preview-kubernetes
# $ opm alpha bundle generate -d deploy/olm-catalog/eclipse-che-preview-kubernetes --channels stable,nightly --package eclipse-che-preview-kubernetes -e stable

opm alpha bundle build -d deploy/olm-catalog/eclipse-che-preview-kubernetes --tag docker.io/aandrienko/che-operator-bundle:latest --package eclipse-che-preview-kubernetes --channels stable,nightly --default stable --image-builder podman
podman push docker.io/aandrienko/che-operator-bundle:latest
opm index add --bundles docker.io/aandrienko/che-operator-bundle:latest --tag docker.io/aandrienko/che-operator-catalog:latest
podman push docker.io/aandrienko/che-operator-catalog:latest



# opm alpha bundle validate --tag docker.io/aandrienko/che-operator-bundle:latest --image-builder podman

# podman push docker.io/aandrienko/che-operator-bundle:latest

# opm index add --bundles docker.io/aandrienko/che-operator-bundle:latest --tag docker.io/aandrienko/che-operator-catalog:latest
