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
