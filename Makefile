


buildDockerExec:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ./docker/main .

makeDockerDir:
	mkdir docker