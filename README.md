# xDS

Implementation of [Envoy's](https://www.envoyproxy.io/) dynamic resources discovery [xDS REST](https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol).

xDS is fundamentally an HTTP service that is hit by every Envoy process to get its state of listeners (LDS), clusters (CDS) and subsequently each cluster's endpoints through (EDS).

It's tightly coupled to Kubernetes:
- Uses config map for configuration.
- Cluster endpoints are Kubernetes service endpoints.

Limitations:
- Services only from the *default* namespace.
- Supports only services exposing one port. Services exposing multiple ports will are ignored.


## Configuration

xDS uses environment variables for configuration:

- **XDS_CONFIGMAP** - Path to the configuration configmap in form `{namespace}/{configmap.name}`. Defaults to `default/xds`.
- **XDS_LISTEN** - Socket address for the http server. Defaults to `127.0.0.1:5000`.


## Running 

Build a docker image:

```
go build xds
```

For xds to run, you need access to Kubernetes cluster. During startup xDS will try to infer by reading the `~/.kube/config` file with fallback to a in cluster config.

Assume you have local cluster running, accessible and the configuration loaded:

```
./xds
2020/05/22 15:24:52 configstore.go:146: ConfigStore.Init:  default xds default/xds
2020/05/22 15:24:52 main.go:106: ready.
```

For testing out use the example configmap at `example/k8s/configmap.yaml`.


## Configuration validation

Validate configmap using `--validate` cli argument:

```
./xds --validate path/to/configmap.yaml

# or from stdin

render_my_configmap | ./xds --validate -
```

Or by `POST`ing to the `/validate` endpoint:

```
curl localhost:5000/validate --data-binary @example/k8s/configmap.yaml
ok

# or 

render_my_configmap | curl localhost:5000/validate --data-binary @- 
ok
```


## Inspecting

These can easily be introspected through the HTTP API with `curl`.

LDS - http://xds.service.sentry.internal/v2/discovery:listeners<br>
CDS - http://xds.service.sentry.internal/v2/discovery:clusters<br>
EDS - http://xds.service.sentry.internal/v2/discovery:endpoints


Both LDS and CDS only need information about the host it's querying about, whereas EDS needs to know what service it's asking about.

An example LDS request looks like:

```shell
% curl -s -XPOST -d '{"node": {"id": "xxx", "cluster":"snuba"}}'  xds.service.sentry.internal/v2/discovery:listeners | jq .
{
  "version_info": "0",
  "resources": [
    {
      "@type": "type.googleapis.com/envoy.api.v2.Listener",
      "name": "snuba-query-tcp",
      "address": {
        "socket_address": {
          "address": "127.0.0.1",
          "port_value": 9000
        }
      },
      "filter_chains": [
        {
          "filters": [
            {
              "name": "envoy.tcp_proxy",
              "typed_config": {
                "@type": "type.googleapis.com/envoy.config.filter.network.tcp_proxy.v2.TcpProxy",
                "stat_prefix": "snuba-query-tcp",
                "cluster": "snuba-query-tcp"
              }
            }
          ]
        }
      ]
    }
  ]
}
```

The request is sending along a node id, and a node cluster assignment. This relates to the `assignments` dataset in our `ConfigMap` if we want to make sure that the correct listeners are being served for `snuba`.

This exact query can be made against the CDS endpoint to get the cluster assignments.

From there, the only weird one is EDS, which is probably the most important one. Since EDS returns back the list of endpoints for a specific backend.

Example:

```shell
% curl -s -XPOST -d '{"node": {"id": "xxx", "cluster":"snuba"},"resource_names":["default/snuba-query-tcp"]}'  xds.service.sentry.internal/v2/discovery:endpoints | jq .
{
  "version_info": "0",
  "resources": [
    {
      "@type": "type.googleapis.com/envoy.api.v2.ClusterLoadAssignment",
      "cluster_name": "default/snuba-query-tcp",
      "endpoints": [
        {
          "lb_endpoints": [
            {
              "endpoint": {
                "address": {
                  "socket_address": {
                    "address": "192.168.208.109",
                    "port_value": 9000
                  }
                }
              }
            },
            {
              "endpoint": {
                "address": {
                  "socket_address": {
                    "address": "192.168.208.139",
                    "port_value": 9000
                  }
                }
              }
            }
          ]
        }
      ]
    }
  ]
}
```

For EDS, it takes an extra "resource_names" key to match the cluster_name inside of the cluster definition.