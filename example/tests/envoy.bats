#!/usr/bin/env bats

@test "/config_dump contains dynamicly configured foo cluster" {
  result="$(
      curl -f -s "${ENVOY_ADMIN_URL}/config_dump" | jq \
        '.configs[]
          | select(.dynamic_active_clusters != null)
          | .dynamic_active_clusters[0].cluster.name'
  )"
  [ "$result" == '"foo"' ]
}

@test "foo cluster response is from foo server" {
  curl -f -s "${ENVOY_FOO_URL}" | grep "Server name: foo-"
}
