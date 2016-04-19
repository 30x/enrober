#enrober

>###Warning: There is currently minimal input validation on the json.

This project consists of a an API server that functions as a wrapper around the kubernetes client library. The server can be deployed both locally and as a docker container.

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

###Modifying the Server

To build a static binary:

```sh
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' -o enrober .
```

To build a docker image:

```sh
docker build -t enrober .
```

##API Design


**Inputs:** 

`{namespace}` is a `string`

`{application}` is a `string`

`{revision}` is a `string`

A `json` body with a corresponding header of `Content-Type: application/json` is required for `PUT/POST` requests. The body has the following contents:

```
{
	"PodCount": 			Int,  				//Required 
	"Image":				string, 			//Optional
	"ImagePullSecret":		string				//Optional
	"TrafficHosts": 		[string],			//Required
	"PublicPaths":  		[string],			//Required 
	"PathPort": 			Int	,				//Required
	"EnvVars": { 			string: string,		//Optional
	    					...,
				 			...
	}
}
```

###Usage

> Assuming you are running the server locally

Get all deployments that match a given namespace:

```sh
curl localhost:9000/beeswax/deploy/api/v1/{namespace}
```

Get all deployments that match a given namespace and application name:

```sh
curl localhost:9000/beeswax/deploy/api/v1/{namespace}/{application}
```

Check if a deployment exists that matches a given namespace, application name, and revision tag:

```sh
curl localhost:9000/beeswax/deploy/api/v1/{namespace}/{application}/{revision}
```

Create a deployment in the given namespace, with the given application name and revision tag. If a namespace matching the passed in value doesn't exist it will be created. 

>Doesn't cover all options, only required ones.

```sh
curl -X PUT -H "Content-Type: application/json" -d '{"PodCount": 1, "TrafficHosts": ["test.k8s.local"], "PublicPaths": ["/app"], "PathPort": 9000}' "http://localhost:9000/beeswax/deploy/api/v1/{namespace}/{application}/{revision}"
```

If an Image is not passed in through the JSON body then the Image to be deployed is determined by concatenating the path parameters as follows:

```
{namespace} + "/" + {application} + ":" + {revision}
```

If an image matching this pattern isn't found then the deployed pods will fail with a `PullImageError` 
