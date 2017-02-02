#enrober

This project consists of a an API server that functions as a wrapper around the kubernetes client library. The server can be deployed both locally and as a docker container within a kubernetes cluster.

###Local Deployment

```sh
go build
./enrober
```

The server will be accesible at `localhost:9000/`

> In a local configuration the server will use information in your `.kube/config` file to communicate with your current cluster. 

###Kubernetes Deployment

A prebuilt docker image is available with:
 
```sh
docker pull thirtyx/enrober:v0.8.0
```

To deploy the server as a docker container on a kubernetes cluster you should use the provided `deploy.yaml` file. Running `kubectl create -f deploy-base.yaml` will pull the image from dockerhub and deploy it to the default namespace.

The server will be accesible at it's `<pod-ip>`

Additionally you can expose the server using a kubernetes service. Refer to the docs [here](http://kubernetes.io/docs/user-guide/services/).

###Privileged Containers

By default enrober doesn't allow privileged containers to be deployed and will modify the containers security context at deploy time so that `Priveleged = false`. If you have a need for privileged containers set the `ALLOW_PRIV_CONTAINERS` environment variable to `"true"` in enrobers deployment yaml file.

##API Design

An OpenAPI.yaml file is provided that documents the API per the OpenAPI specification.

##Key Components

####Environments

An environment consists of a kubernetes namespace and our specific secrets associated with it. Each environment comes with a `routing` secret that contains two key-value pairs, a `public-api-key` and a `private-api-key`. These are for use with the [k8s-router](https://github.com/30x/k8s-router) to allow for secure communication with pods from inside and outside of the kubernetes cluster. 

When created environments can accept an array of valid host names to accept traffic from. This array is represented on the namespace object as a space delimited annotation. The individual values must be either a valid IP address or valid host name. 

####Deployments

When created deployments can accept a `publicHosts` value, a `privateHosts` value or both. These values are for use with the [k8s-router](https://github.com/30x/k8s-router) and are the host name where the deployment can be reached. These values are stored as annotations on the deployed pods. 

####Pod Template Specs

Enrober only accepts Pod Template Specs(PTS) through a URL. For testing it is easiest to host your PTS as a JSON object on a site like [myjson.com](myjson.com).

Please note that Enrober only supports single container pods at this time!

An example Pod Template Spec might look like:

```json
{
  "metadata": {
    "name": "nginx",
    "labels": {
      "component": "nginx"
    }
  },
  "spec": {
    "containers": [
      {
        "name": "nginx",
        "image": "nginx",
        "ports": [
          {
            "containerPort": 80
          }
        ]
      }
    ]
  }
}

```


##Usage

> This assumes you are running the `Enrober` server locally so that it is accessible at `localhost:9000`

**Note:** When running in production mode all API calls require a valid Apigee JWT to be passed into an authorization header. 

This will create a `org1-env1` namespace and a secret named `routing` with two key-value pairs:

- public-api-key
- private-api-key

The value of each of these keys-value pairs will a 256-bit base64 encoded randomized string. 

###Update the environment

```sh
curl -X PATCH -d '{}' \
"localhost:9000/environments/org1:env1"
```

This will modify the previously created environment's hostNames array to equal:

###Create deployment

```sh
curl -X POST -d '{
	"deploymentName": "dep1",
    "edgePaths": [{
	    "basePath": "base",
    	"containerPort": 9000,
    	"targetPath": "target"
    },
	"replicas": 1,
	"ptsURL": "http://myjson.com/a41kb",
	"envVars": [
	{
		"name": "test1",
		"value": "value1",
	}]
}' \
"localhost:9000/environments/org1:env1/deployments"
```

If you do not already have an `Environment` created this call with create the underlying `Environment` object to place the `Deployment` into as well. 

There is an added bit of `Apigee` specific syntax for environment variables.

```json
{
	"name": "test2",
	"valueFrom": {
		"edgeConfigRef": {
			"name": "GoTest1",
			"key": "key1"
		}
	}
}
```
This will attempt to retrieve the value stored in the named KVM under the given key. The KVM must be under the organization and environment that your deployment is under. This call is only supported by setting the `"APIGEE_KVM"` environment variable to `"true"`. 

### Update deployment
	
```sh
curl -X PATCH  -d '{
	"replicas": 3,
}' \
"localhost:9000/environments/org1:env1/deployments/dep1"
```

This will modify the previous deployment to now maintain 3 replicas of the pod.

###Delete deployment

```sh
curl -X DELETE  \
"localhost:9000/environments/org1:env1/deployments/dep1"
```

This will delete the previously created deployment and all related resources such as replica sets and pods. 
