#enrober

This project consists of a an API server that functions as a wrapper around the kubernetes client library. The server can be deployed both locally and as a docker container within a kubernetes cluster.

This project is closely related to other 30x projects:

- [dev-setup](https://github.com/30x/Dev_Setup)
- [k8s-pods-ingress](https://github.com/30x/k8s-pods-ingress)
- [shipyard](https://github.com/30x/shipyard)

###Local Deployment

```sh
go build
./enrober
```

The server will be accesible at `localhost:9000/beeswax/deploy/api/v1`

For the server to be able to communicate with your kubernetes cluster you must run:

```
kubectl proxy --port=8080 &
```

Please note that this allows for insecure communication with your kubernetes cluster and shuold only be used for testing.

###Kubernetes Deployment

A prebuilt docker image is available with:
 
```sh
docker pull jbowen/enrober:v0.1.0
```

To deploy the server as a docker container on a kubernetes cluster you should use the provided `deploy-base.yaml` file. Running `kubectl create -f deploy-base.yaml` will pull the image from dockerhub and deploy it to the default namespace.

The server will be accesible at `<pod-ip>/beeswax/deploy/api/v1`

You can choose to expose the pod using the [k8s-pods-ingress](https://github.com/30x/k8s-pods-ingress). Make sure to modify the `deploy.yaml` file to match your ingress configuration. 

Alternatively you can expose the server using a kubernetes service. Refer to the docs [here](http://kubernetes.io/docs/user-guide/services/).

##API Design

A swagger.yaml file is provided that documents the API per the OpenAPI specification.

##Key Components

TODO: Explain stuff

##Usage

> This assumes you are running the server locally, it is accessible at localhost:9000, and your kubernetes cluster is exposed with `kubectl proxy --port=8080`

####Create a new environment:

```sh
curl -X POST -d '{
	"environmentName": "env1",
	"hostNames": ["host1"]
	}' \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments"
```

This will create a `group1-env1` namespace and a secret named `routing` with two key-value pairs:

- public-api-key
- private-api-key

The value of each of these keys-value pairs will a 256-bit base64 encoded randomized string. These secrets are for use with [30x/k8s-pods-ingress](https://github.com/30x/k8s-pods-ingress)

###Update the environment

```sh
curl -X PATCH -d '{
	"hostNames": ["host1", "host2"]
	}' \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments/env1"
```

This will modify the previously created environment's hostNames array to equal:

`["host1", "host2"]`

### Create a new deployment from an inline Pod Template Spec

```sh
curl -X POST -d '{
	"deploymentName": "dep1",
    "publicHosts": "deploy.k8s.public",
    "privateHosts": "deploy.k8s.private",
	"replicas": 1,
	"pts": 
	{
		"apiVersion": "v1",
		"kind": "Pod",
		"metadata": {
			"name": "nginx-and-helloworld",
			"labels": {
				"app": "web",
			},
			"annotations": {
		       	"publicPaths": "80:/ 90:/2",  
		        "privatePaths": "80:/ 90:/2"
	        }
		},
		"spec": {
			"containers": [{
				"name": "nginx",
				"image": "nginx",
				"env": [{
					"name": "PORT",
					"value": "80"
				}],
				"ports": [{
					"containerPort": 80
				}]
			}, {
				"name": "test",
				"image": "jbowen/testapp:v0",
				"env": [{
					"name": "PORT",
					"value": "90"
				}],
				"ports": [{
					"containerPort": 90
				}]
			}]
		}
	}
}' \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments/env1/deployments"
```

This will create a deployment that will guarantee a single replica of a pod consisting of two containers: 

- An nginx container serving on port 80
- A hello world container serving on port 90


### Update deployment
	
```sh
curl -X PATCH -d '{
	"deploymentName": "dep1",
	"replicas": 3,
}' \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments/env1/deployments/dep1"
```

This will modify the previous deployment to now guarantee 3 replicas of the pod.


###Delete deployment

```sh
curl -X DELETE \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments/env1/deployments/dep1"
```

This will delete the previously created deployment and all related resources such as replica sets and pods. 

###Delete environment

```sh
curl -X DELETE \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments/env1"
```

This will delete the previously created environment. 