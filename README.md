#enrober

This project consists of a an API server that functions as a wrapper around the kubernetes client library. The server can be deployed both locally and as a docker container within a kubernetes cluster.

###Local Deployment

```sh
go build
./enrober
```

The server will be accesible at `localhost:9000/beeswax/deploy/api/v1`

###Kubernetes Deployment

A prebuilt docker image is available with:
 
```sh
docker pull jbowen/enrober:v0.0.1
```

To deploy the server as a docker container on a kubernetes cluster you should use the provided `deploy.yaml` file. Running `kubectl create -f deploy.yaml` will pull the image from dockerhub and deploy it to the default namespace.

The server will be accesible at `<pod-ip>/beeswax/deploy/api/v1`

You may also choose to build the files into your own docker image using the following steps:  

```sh
#Build a static binary
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' -o enrober .

#Build a docker image
docker build -t enrober .

#Tag the image
docker tag -f enrober <desired name>

#Push to dockerhub
docker push <desired-name>
```

The server can now be deployed using:

```sh
kubectl run enrober --image=<desired-name>
```

##API Design

A swagger.yaml file is provided that documents the API per the OpenAPI specification.

###Usage

> Assuming you are running the server locally

Create a new namespace group1-env1 with a secret for use with [ingress](https://github.com/30x/k8s-pods-ingress#security)

```sh
curl -X POST -d '{
	"environmentName": "env1",
	"secret": "12345"
	}' \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments"
```

Create a new deployment dep1 

```sh
curl -X POST -d '{
	"deploymentName": "dep1",
	"trafficHosts": "test.k8s.local",
	"replicas": 1,
	"ptsUrl": ""
	}' \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments/env1/deployments"
```

Update deployment dep1 with new Pod Template Spec
	
```sh
curl -X PATCH -d '{
	"deploymentName": "dep1",
	"trafficHosts": "test.k8s.local",
	"replicas": 1,
	"ptsUrl": ""
	}' \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments/env1/deployments/dep1"
```

Delete deployment dep1

```sh
curl -X DELETE \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments/env1/deployments/dep1"
```

Delete namespace group1-env1

```sh
curl -X DELETE \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments/env1"
```
