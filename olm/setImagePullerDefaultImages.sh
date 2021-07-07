#!/bin/bash
#
# Copyright (c) 2021 Red Hat, Inc.
# This program and the accompanying materials are made
# available under the terms of the Eclipse Public License 2.0
# which is available at https://www.eclipse.org/legal/epl-2.0/
#
# SPDX-License-Identifier: EPL-2.0
#
# Contributors:
#   Red Hat, Inc. - initial API and implementation


CSV="$1" # Path to CSV file
CR="$2" # Path to custom resource

# Bash array of related images patterns to collect
patterns=(
    ^RELATED_IMAGE_.*_plugin_java8$
    ^RELATED_IMAGE_.*_plugin_java11$
    ^RELATED_IMAGE_.*_plugin_kubernetes$
    ^RELATED_IMAGE_.*_plugin_openshift$
    ^RELATED_IMAGE_.*_plugin_broker.*
    ^RELATED_IMAGE_.*_theia.*
    ^RELATED_IMAGE_.*_stacks_cpp$
    ^RELATED_IMAGE_.*_stacks_dotnet$
    ^RELATED_IMAGE_.*_stacks_golang$
    ^RELATED_IMAGE_.*_stacks_php$
    ^RELATED_IMAGE_.*_cpp_.*_devfile_registry_image.*
    ^RELATED_IMAGE_.*_dotnet_.*_devfile_registry_image.*
    ^RELATED_IMAGE_.*_golang_.*_devfile_registry_image.*
    ^RELATED_IMAGE_.*_php_.*_devfile_registry_image.*
    ^RELATED_IMAGE_.*_java.*_maven_devfile_registry_image.*
)

# Accumulate each pattern into one large pattern
pattern=${patterns[0]}
patterns=("${patterns[@]:1}")
for i in ${patterns[@]}; do
    pattern=$pattern"|"$i
done


RELATED_IMAGES=$(yq -r '.spec.install.spec.deployments[0].spec.template.spec.containers[0].env' $CSV | jq -rc ".[] | select(.name | test(\"$pattern\"))")
PREFIX="RELATED_IMAGE_"
PREFIX_LENGTH=$(echo ${#PREFIX})
MAX_NAME_LENGTH=63
RFC1123_PATTERN="[a-z0-9]([-a-z0-9]*[a-z0-9])?"

# Gather a semi-colon-seperated list of images in this format:
# image_name=image_url;image_name=image_url;image_name=image_url ...
for image in ${RELATED_IMAGES}; do
    value=$(echo $image | jq -r '.value')

    # names must be in RFC 1123 format
    name=$(echo $image | jq -r '.name')

    # remove prefix
    name=$(echo ${name:$PREFIX_LENGTH:MAX_NAME_LENGTH})

    # convert to lowercase, remove dashes
    name=$(echo $name | tr '[:upper:]' '[:lower:]')
    name=$(echo $name | sed 's/_/-/g')

    # capture the start of the string that matches the RFC 1123 format
    name=$([[ $name =~ (${RFC1123_PATTERN})(.*) ]] && echo ${BASH_REMATCH[1]})

    images=$images$name"="$value";"
done

# Write images to custom resource
sed -ri 's|(images: ).*|\1"'"$images"'"|g' $CR
