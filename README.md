# Welcome to Distributed Tracing PoC!

This repository contains a sample app scenario that show how we can implement tracing. Here, we are using OpenTelemetry api/sdk to instrument our code and export our traces to OpenTelemetry agent and collector and using Jaeger as backend of our traces.

## Setup
This demo uses docker-compose and by default runs against the iqfarhad/medium-poc_tracing:latest image which is the image built based on the available code. To run the demo, you need to clone this repo and then run:

```shell
docker-compose up -d
```

Also you can build the image locally and test the program.

## Structure 
In this scenario we have 3 main module, Main server, Formatter, Queryyer*;

 1. Main Server is our first endpoint which listens to `http://localhost:8080/sayHello/` and we can pass any name as a parameter to this api like for example: 
   `http://localhost:8080/sayHello/trace`.
   
 2. Queryyer is the second end point which our main server will call after receiving a request. The main task of Queryyer is to query the database and return the information related to the person name, if exist. The name of Queryyer is based on Formatter :).

3. The third service is Formatter which again will be called by the main server, and basically it put the retrieved information from the queryyer into a specific format; title-name-description.

We have also two other folders, Lib and Client:
Lib: contains general initializer for OpenTelemetry with Jaeger endpoint
Client: by running the client main.go we are simulating one single request to the server, this is equivalent of running the command, `curl http://localhost:8080/sayHello/trace`. Moreover I put the equivalent of the current setup K8s file in the k8s folder. In there you can find out to set up agent and collector in case of kubernetes.
