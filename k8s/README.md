# Kubernetes

This folder contains the all-in-one kuberentes yaml file which show how to set up the OpenTelemetry agent or collector, plus the Jaeger. Take into account that, in here I used deployments for the agent but, in production you need to consider having it as DaemonSet or Sidecar.


## Setup
To test the sample application and set up quickly the otel collector and agent, you just need to to use the command below:

```shell
kubectl apply -f all-in-one.yaml
```

this will create and set up required components. after the deployments and pods are running, you can do:

```shell
kubectl port-forward -n monitoring service/jaeger 16686:16686
```
this will allow you to check the Jaeger dashboard from your localhost (http://localhost:16686/)

and then in another terminal tab do:

```shell
kubectl port-forward -n monitoring service/tracing-poc 8080:8080
```
this will allow you to test and run the sample HelloWorld application by doing:

```shell
curl http://localhost:8080/sayHello/trace
```
Voila!