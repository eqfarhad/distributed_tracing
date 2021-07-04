# Welcome to Distributed Tracing PoC!
![alt text](https://raw.githubusercontent.com/eqfarhad/distributed_tracing/main/docs/trace_header.jpg)

This repository contains a sample HelloWorld app scenario that show how we can implement tracing. Here, we are using OpenTelemetry sdk to instrument our code and are exporting our traces to OpenTelemetry agent and collector. beside we are using Jaeger as the backend of our traces and we are able to check the results in the Jaeger dashboard.



## Setup
This demo uses docker-compose and by default runs against the iqfarhad/medium-poc_tracing:latest image which is the image built based on the available code. To run the demo, you need to clone this repo and then run:

```shell
docker-compose up -d
```
if everything goes fine, you should be able to see the Jaeger dashboard by visiting this endpoint: `http://localhost:16686/`.
At this point still you cannot see any traces instead of the default jaeger service. 

To actually see the traces from this app, you need to run the command below or simply visit this page in your browser `http://localhost:8080/sayHello/trace`:
```shell
curl http://localhost:8080/sayHello/trace
```

This will produce more traces and by refreshing the jaeger dashboard, nnow you should be able to see those as well.


## Optional
You can also build the image locally and test the program. to do so, you need to edit the docker-compose file and uncomment the `build: ./`. then you can run `docker-compose build` and use that image to run this application.

More over, if you check the docker-compose file, I'm passing a env variable `TRACING_OPTION` which by default, I set it as `otel-collector`. This means that our traces are gonna get exported to the otel agent. you can set this variable to, `jaeger-collector` and then the application will export traces straightly to the Jaeger agent. (you can set it to export to the Jaeger collector as well, the code is available in `lib/tracing/init.go` )
## Structure 
![alt text](https://raw.githubusercontent.com/eqfarhad/distributed_tracing/main/docs/example_scenario.jpg)
In this scenario we have 3 main module, Main server, Formatter, Queryyer*;

 1. Main Server is our first endpoint which listens to `http://localhost:8080/sayHello/` and we can pass any name as a parameter to this api like for example: 
   `http://localhost:8080/sayHello/trace`.
   
 2. Queryyer is the second end point which our main server will call after receiving a request. The main task of Queryyer is to query the database and return the information related to the person name, if exist. The name of Queryyer is based on Formatter :).

3. The third service is Formatter which again will be called by the main server, and basically it put the retrieved information from the queryyer into a specific format; title-name-description.

We have also two other folders, Lib and Client:
Lib: contains general initializer for OpenTelemetry with Jaeger endpoint
Client: by running the client main.go we are simulating one single request to the server, this is equivalent of running the command, `curl http://localhost:8080/sayHello/trace`. Moreover I put the equivalent of the current setup K8s file in the k8s folder. In there you can find out to set up agent and collector in case of kubernetes.

## Result in Jaeger Dashboard
![alt text](https://raw.githubusercontent.com/eqfarhad/distributed_tracing/main/docs/example.PNG)
