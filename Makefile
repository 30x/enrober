build-to-docker: main.go
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w' -o enrober .
	docker build -t enrober .

push-to-local:
	docker tag -f enrober localhost:5000/enrober
	docker push localhost:5000/enrober
	
push-to-hub:
	docker tag -f enrober jbowen/enrober:v0
	docker push jbowen/enrober:v0

deploy-to-kube:
	kubectl run enrober --image=localhost:5000/enrober:latest
	
deploy-to-aws:
	kubectl run enrober --image=jbowen/enrober:v0
