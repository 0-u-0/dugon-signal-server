


buildDockerExec:
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.lastCompile=`date +%Y-%m-%d-%X`" -a -installsuffix cgo -o ./docker/main .

makeDockerDir:
	mkdir docker