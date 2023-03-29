# Dockerfile Clarification

**Dockerfile** is the Eclipse Che Operator build file, used in this repo to publish to [quay.io/eclipse/che-operator](https://quay.io/repository/eclipse/che-operator?tab=tags).

See also [community-operators/eclipse-che](https://github.com/redhat-openshift-ecosystem/community-operators-prod/tree/main/operators/eclipse-che).

**brew.Dockerfile** is a variation on `Dockerfile` specifically for Red Hat builds, and cannot be run locally as is. It is used to publish the [Red Hat OpenShift Dev Spaces Operator](https://github.com/redhat-developer/devspaces-images/tree/devspaces-3-rhel-8/devspaces-operator) image to [quay.io/devspaces/devspaces-rhel8-operator](https://quay.io/repository/devspaces/devspaces-rhel8-operator?tab=tags).

See also [Red Hat OpenShift Dev Spaces Operator Bundle](https://github.com/redhat-developer/devspaces-images/tree/devspaces-3-rhel-8/devspaces-operator-bundle). 
