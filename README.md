# ssh-gateway-docker

Runs a SSH server that exposes a docker shell (i.e. a shell that can be used to run
only docker commands)


```
> go run main.go 3333
2014/10/15 03:54:53 [INFO] listening to "0.0.0.0:3333"
2014/10/15 03:54:54 [INFO] got request "pty-req"
2014/10/15 03:54:54 [INFO] got request "env"
2014/10/15 03:54:54 [INFO] got request "env"
2014/10/15 03:54:54 [INFO] got request "env"
2014/10/15 03:54:54 [INFO] got request "shell"

> ssh -p 3333 0.0.0.0
docker > run -it busybox
/ # whoami
root
/ #
```
