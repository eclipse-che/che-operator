#!/bin/bash

# SCRIPT=$(readlink -f "$0")
# export SCRIPT

# BASE_DIR=$(dirname "$SCRIPT");
# export BASE_DIR

# if [ ! $(oc get configs.imageregistry.operator.openshift.io/cluster -o yaml | yq -r ".spec.defaultRoute") == true ];then
#     oc patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
# fi

HOST=$(oc get route default-route -n openshift-image-registry -o yaml | yq -r ".spec.host")
certBundle=$(echo "Q" | openssl s_client -showcerts -connect "${HOST}":443)
# echo $certBundle
CA_CRT="/home/user/crt/test.crt"
rm -rf "${CA_CRT}"
touch "${CA_CRT}"

echo "${certBundle}" |
while IFS= read -r line
do
  if [ "${line}" == "-----BEGIN CERTIFICATE-----" ]; then
    IS_CERT_STARTED=true
  fi
  
  if [ "${IS_CERT_STARTED}" == true ]; then
    CERT="${CERT}${line}\n"
  fi

  if [ "${line}" == "-----END CERTIFICATE-----" ]; then
    if echo -e "${CERT}" | openssl x509 -text | grep -q "CA:TRUE"; then
        echo "CA sertificate found! And store by path ${CA_CRT}"
        echo -e "${CERT}" > "${CA_CRT}"
        exit 0
    fi
    CERT=""
    IS_CERT_STARTED="false"
  fi
done
echo "=========================="
