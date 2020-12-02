set -x

setCustomOperatorImage() {
  updateYaml "'.spec.template.spec.containers[0].imagePullPolicy = "IfNotPresent"'" ${1}
  updateYaml "'.spec.template.spec.containers[0].image = "'${2}'"'" ${1}
}

updateYaml() {
  yq -rSY ${1} > /tmp/tmp.yaml && mv /tmp/tmp.yaml ${2}
}

setCustomOperatorImage operator.yaml ttttttt

