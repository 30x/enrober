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
docker pull thirtyx/enrober:v0.1.2
```

To deploy the server as a docker container on a kubernetes cluster you should use the provided `deploy-base.yaml` file. Running `kubectl create -f deploy-base.yaml` will pull the image from dockerhub and deploy it to the default namespace.

The server will be accesible at `<pod-ip>/beeswax/deploy/api/v1`

You can choose to expose the pod using the [k8s-pods-ingress](https://github.com/30x/k8s-pods-ingress). Make sure to modify the `deploy.yaml` file to match your ingress configuration. 

Alternatively you can expose the server using a kubernetes service. Refer to the docs [here](http://kubernetes.io/docs/user-guide/services/).

###Privileged Containers

By default enrober doesn't allow privileged containers to be deployed and will modify the containers security context at deploy time so that `Priveleged = false`. If you have a need for privileged containers set the `ALLOW_PRIV_CONTAINERS` environment variable to `"true"` in enrobers deployment yaml file.

##API Design

A swagger.yaml file is provided that documents the API per the OpenAPI specification.

##Key Components

####Environments

An environment consists of a kubernetes namespace and our specific secrets associated with it. Each environment comes with a `routing` secret that contains two key-value pairs, a `public-api-key` and a `private-api-key`. These are for use with the [k8s-pods-ingress](https://github.com/30x/k8s-pods-ingress) to allow for secure communication with pods from inside and outside of the kubernetes cluster.  


##Apigee Specific Annotations

####Environments

When created environments can accept an array of valid host names to accept traffic from. This array is represented on the namespace object as a space delimited annotation. The individual values must be either a valid IP address or valid host name. 

####Deployments

When created deployments can accept a `publicHosts` value, a `privateHosts` value or both. These values are for use with the [k8s-pods-ingress](https://github.com/30x/k8s-pods-ingress) and are the host name where the deployment can be reached. These values are stored as annotations on the deployed pods. 

####Pod Template Specs

When they are provided to the deployments endpoint pod template specs must have several Apigee specific labels and annotations.  

**Labels:**
For pods to be recognized by the [k8s-pods-ingress](https://github.com/30x/k8s-pods-ingress) they must have a label named `"routable"` with a value of `"true"`.

**Annotations:**
For pods to be properly routed by the [k8s-pods-ingress](https://github.com/30x/k8s-pods-ingress) they must have a `"publicPaths"` and/or `"privatePaths"` annotation where the value is of the form `{PORT}:{PATH}`. You may have multiple space delimited `{PORT}:{PATH}` combinations on each annotation. 
 

##Usage

> This assumes you are running the server locally, it is accessible at localhost:9000, and your kubernetes cluster is exposed with `kubectl proxy --port=8080`

####Create a new environment:

**Note:** Creating and modifying an environment require a valid JWT to be passed into an authorization header. For this example we are using an empty JSON object that has been base64 encoded.

```sh
curl -X POST -H "Authorization: Bearer e30.e30.e30" -d '{
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

**Note:** Creating and modifying an environment require a valid JWT to be passed into an authorization header. For this example we are using an empty JSON object that has been base64 encoded.

```sh
curl -X PATCH -H "Authorization: Bearer e30.e30.e30" -d '{
	"hostNames": ["host1", "host2"]
	}' \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments/env1"
```

This will modify the previously created environment's hostNames array to equal:

`["host1", "host2"]`

### Create a new deployment from an inline Pod Template Spec

```sh
curl -X POST -H "Authorization: Bearer e30.e30.e30" -d '{
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
curl -X PATCH -H "Authorization: Bearer e30.e30.e30" -d '{
	"deploymentName": "dep1",
	"replicas": 3,
}' \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments/env1/deployments/dep1"
```

This will modify the previous deployment to now guarantee 3 replicas of the pod.


###Delete deployment

```sh
curl -X DELETE -H "Authorization: Bearer e30.e30.e30" \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments/env1/deployments/dep1"
```

This will delete the previously created deployment and all related resources such as replica sets and pods. 

###Delete environment

```sh
curl -X DELETE -H "Authorization: Bearer e30.e30.e30" \
"localhost:9000/beeswax/deploy/api/v1/environmentGroups/group1/environments/env1"
```

This will delete the previously created environment. 