# XDS Example

This example can be used for local development on the xds project. Contains
minimal configuration for deployment to k8s and envoy configuration that uses
the xds service.

## Requirements

- access to a (local) k8s cluster
- docker for building the xds service container
- envoy binary or container

## Build the xds docker image

From root of the repository:

```bash
docker build -t xds -f Dockerfile .
```

## Deploy to k8s

The `XDS_CONFIGMAP` environment variable points to the used config map. It is
in form `{namespace}/{configmap name}`. In this example it is part of the
`deployment.yaml` and defaults to `default/xds`.

Deploy configmap and the xds deployment, envoy and testing service from the
`example/k8s` sub directory:

```bash
kubectl --namespace xds apply -f k8s/
```

## Access xds service

Assuming local k8s cluster is used which expose `NodePort` services on
localhost.

Retrieve information about the port used by the xds service:

```bash
kubectl --namespace xds get service xds
NAME   TYPE       CLUSTER-IP       EXTERNAL-IP   PORT(S)        AGE
xds    NodePort   10.104.220.250   <none>        80:30311/TCP   2s
```

Curl the xds service using the node port `30311`:

```bash
curl localhost:30311/healthz
ok
```

```bash
curl localhost:30311/config
{"version":"17287","last_error":"","last_update":"2020-05-13T11:38:45.3721989Z"}
```


## Access xds pod

Get name of the running xds pod:

```bash
kubectl --namespace xds get pods
NAME                  READY   STATUS             RESTARTS   AGE
xds-f68c64b47-95p9s   0/1     CrashLoopBackOff   15         3h8m
```

Forward its port to the localhost:

```bash
kubectl --namespace xds port-forward xds-f68c64b47-95p9s 8080:80
Forwarding from 127.0.0.1:8080 -> 80
Forwarding from [::1]:8080 -> 80
```

Check that it is running and reachable:

```bash
curl localhost:8080/config

{"version":"14120","last_error":"","last_update":"2020-05-13T11:08:34.2465368Z"}
```

```bash
curl localhost:8080/healthz

ok
```

Port forwarding is usefull when one wants to access specific pod but it
becomes annoying when pushing new images and creating new pods which requires
to also recreate the port forwarding.


# NOTES:
- endpoints only in default scope
- only services exposing one port
