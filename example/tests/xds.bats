#!/usr/bin/env bats

@test "/heatlhz endpoint is ok" {
  result="$(curl -f -s "${XDS_URL}/healthz")"
  [ "$result" == "ok" ]
}

@test "/config endpoint reports no error" {
  result="$(curl -f -s "${XDS_URL}/config" | jq '.last_error')"
  [ "$result" == '""' ]
}

@test "/v2/discovery:clusters select foo" {
  run curl -s -X POST \
    -d '{"node": {"id": "", "cluster":"foo"}}' \
    "${XDS_URL}/v2/discovery:clusters"

  [ "${status}" -eq 0 ]
  [ "$(echo "${output}" | jq '.resources | length')" -eq 1 ]
  [ "$(echo "${output}" | jq '.resources[0].name')" == '"foo"' ]

}

@test "/v2/discovery:clusters select bar" {
  run curl -s -X POST \
    -d '{"node": {"id": "", "cluster":"bar"}}' \
    "${XDS_URL}/v2/discovery:clusters"

  [ "${status}" -eq 0 ]
  [ "$(echo "${output}" | jq '.resources | length')" -eq 1 ]
  [ "$(echo "${output}" | jq '.resources[0].name')" == '"bar"' ]

}

@test "/v2/discovery:clusters select baz" {
  run curl -s -X POST \
    -d '{"node": {"id": "", "cluster":"baz"}}' \
    "${XDS_URL}/v2/discovery:clusters"


  [ "${status}" -eq 0 ]
  [ "$(echo "${output}" | jq '.resources | length')" -eq 2 ]
  [ "$(echo "${output}" | jq '.resources[0].name')" == '"bar"' ]
  [ "$(echo "${output}" | jq '.resources[1].name')" == '"baz"' ]
  [ "$(echo "${output}" | jq '.resources[1].type')" == '"STATIC"' ]
  [ "$(echo "${output}" | jq '.resources[1].load_assignment.endpoints[0].lb_endpoints[0].endpoint.address.socket_address.port_value')" -eq 8888 ]

}

@test "/v2/discovery:clusters select qux" {
  run curl -s -X POST \
    -d '{"node": {"id": "", "cluster":"qux"}}' \
    "${XDS_URL}/v2/discovery:clusters"

  [ "${status}" -eq 0 ]
  [ "$(echo "${output}" | jq '.resources | length')" -eq 1 ]
  [ "$(echo "${output}" | jq '.resources[0].name')" == '"qux"' ]

}

@test "/v2/discovery:listeners select foo" {
  run curl -s -X POST \
    -d '{"node": {"id": "", "cluster":"foo"}}' \
    "${XDS_URL}/v2/discovery:listeners"

  [ "${status}" -eq 0 ]
  [ "$(echo "${output}" | jq '.resources | length')" -eq 1 ]
  [ "$(echo "${output}" | jq '.resources[0].name')" == '"foo"' ]

}

@test "/v2/discovery:listeners select bar" {
  run curl -s -X POST \
    -d '{"node": {"id": "", "cluster":"bar"}}' \
    "${XDS_URL}/v2/discovery:listeners"

  [ "${status}" -eq 0 ]
  [ "$(echo "${output}" | jq '.resources | length')" -eq 1 ]
  [ "$(echo "${output}" | jq '.resources[0].name')" == '"bar"' ]

}

@test "/v2/discovery:listeners select qux" {
  run curl -s -X POST \
    -d '{"node": {"id": "", "cluster":"qux"}}' \
    "${XDS_URL}/v2/discovery:listeners"

  [ "${status}" -eq 0 ]
  [ "$(echo "${output}" | jq '.resources | length')" -eq 1 ]
  [ "$(echo "${output}" | jq '.resources[0].name')" == '"qux"' ]

}

@test "/v2/discovery:endpoints select foo" {
  run curl -s -X POST \
    -d '{"node": {"id": "", "cluster":"foo"}, "resource_names": ["default/foo"]}' \
    "${XDS_URL}/v2/discovery:endpoints"

  [ "${status}" -eq 0 ]
  [ "$(echo "${output}" | jq '.resources | length')" -eq 1 ]
  [ "$(echo "${output}" | jq '.resources[0].cluster_name')" == '"default/foo"' ]

}

@test "/v2/discovery:endpoints select bar" {
  run curl -s -X POST \
    -d '{"node": {"id": "", "cluster":"bar"}, "resource_names": ["default/bar"]}' \
    "${XDS_URL}/v2/discovery:endpoints"

  [ "${status}" -eq 0 ]
  [ "$(echo "${output}" | jq '.resources | length')" -eq 1 ]
  [ "$(echo "${output}" | jq '.resources[0].cluster_name')" == '"default/bar"' ]

}

@test "/v2/discovery:endpoints select qux" {
  run curl -s -X POST \
    -d '{"node": {"id": "", "cluster":"qux"}, "resource_names": ["quxns/qux"]}' \
    "${XDS_URL}/v2/discovery:endpoints"

  echo "${output}" >> /tmp/quxout

  [ "${status}" -eq 0 ]
  [ "$(echo "${output}" | jq '.resources | length')" -eq 1 ]
  [ "$(echo "${output}" | jq '.resources[0].cluster_name')" == '"quxns/qux"' ]

}

@test "/validate endpoint invalid configmap" {
  run curl -s -f -d "invalid yaml" "${XDS_URL}/validate"
  [ "${status}" -ne 0 ]
}

@test "/validate endpoint valid configmap" {
  run curl -s -f --data-binary @"${SCRIPT_DIR}/k8s/configmap.yaml" "${XDS_URL}/validate"
  [ "${status}" -eq 0 ]
  [ "${output}" == "ok" ]
}


@test "--validate from stdin invalid" {
  run docker run --rm -i xds --validate - <<<"invalid yaml"
  [ "${status}" -eq 1 ]
}

@test "--validate from stdin valid" {
  docker run --rm -i xds --validate - < "${SCRIPT_DIR}/k8s/configmap.yaml"
}

@test "--validate file invalid" {
  run docker run --rm -i xds --validate <(echo "invalid")
  [ "${status}" -eq 1 ]
}

@test "--validate file valid" {
  docker run --volume "${SCRIPT_DIR}/k8s/configmap.yaml:/configmap.yaml" --rm -i xds --validate /configmap.yaml
}
