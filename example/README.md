# xDS Example

This example can be used for local development on the xDS project. Contains
minimal configuration for deployment to k8s and envoy configuration that uses
the xDS service.

## Requirements

- access to a (local) k8s cluster
- docker for building the xDS service container
- envoy binary or container

For running tests you will require besides above mentioned also:

- curl, jq
- bats - the bash tests runner

## Build the xds docker image

From root of the repository:

```bash
docker build -t xds -f Dockerfile .
```

## Deploy xDS to k8s

The `XDS_CONFIGMAP` environment variable points to the used config map. It is
in form `{namespace}/{configmap name}`. In this example it is part of the
`deployment.yaml` and defaults to `default/xds`.

Deploy configmap and the xds deployment, envoy and testing service from the
`example/k8s` sub directory:

```bash
kubectl apply -f k8s/configmap.yaml -f k8s/xds.yaml
```

## Access xds service

Assuming local k8s cluster is used which expose `NodePort` services on
localhost.

Retrieve information about the port used by the xds service:

```bash
kubectl get service xds
NAME   TYPE       CLUSTER-IP       EXTERNAL-IP   PORT(S)        AGE
xds    NodePort   10.104.220.250   <none>        80:30311/TCP   2s
```

Curl the xDS service using the node port `30311`:

```bash
curl localhost:30311/config
{"version":"17287","last_error":"","last_update":"2020-05-13T11:38:45.3721989Z"}
```

```bash
curl localhost:30311/healthz
ok
```


## Access xds pod

Get name of the running xds pod:

```bash
kubectl get pods
NAME                  READY   STATUS             RESTARTS   AGE
xds-f68c64b47-95p9s   0/1     CrashLoopBackOff   15         3h8m
```

Forward its port to the localhost:

```bash
kubectl port-forward xds-f68c64b47-95p9s 8080:80
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


# Whole example environment 

In the `example/k8s` is configuration to spin whole, most basic, integration
testing environment consisting of:

- xDS
- envoy - configured to use xDS
- foo, bar - services discoverable by envoy through xDS

## Build images

From root of this repository:

``` 
docker build -t xds .
docker build -t xdsenvoy example/envoy
```

Envoy's bootstrap configuration is in `example/envoy/envoy.yaml`.

### Deploy & Delete 

```
kubectl apply -f example/k8s/
```

This will deploy set of deplyments and their respective services.

```
kubectl get services
NAME         TYPE        CLUSTER-IP       EXTERNAL-IP   PORT(S)                          AGE
bar          NodePort    10.106.164.147   <none>        80:30798/TCP                     5s
envoy        NodePort    10.107.243.120   <none>        8001:32075/TCP,10001:31763/TCP   5s
foo          NodePort    10.98.250.222    <none>        80:32335/TCP                     5s
kubernetes   ClusterIP   10.96.0.1        <none>        443/TCP                          2d3h
xds          NodePort    10.106.127.94    <none>        80:32344/TCP                     8m37s
```

To delete:

```
kubectl delete -f example/k8s/
```


# Running test suite

The test suite will deploy the whole ensemble, check basic functionality and
delete all the k8s resources afterwards. Please install `bats` (`brew install bats`)
  tests runner.

```
./runtests.sh
```
