UNIX_TIME := $(shell date +%s)

nnn:
	@echo ${UNIX_TIME}

.PHONY: run1
run1:
	HTTP_PORT=50001 GRPC_PORT=50101 DISCOVERY_PORT=50201 RAFT_PORT=50301 RAFT_DIR=".db/s1/${UNIX_TIME}" go run ./example server

.PHONY: run2
run2:
	HTTP_PORT=50002 GRPC_PORT=50102 DISCOVERY_PORT=50202 RAFT_PORT=50302 RAFT_DIR=".db/s2/${UNIX_TIME}" go run ./example server

.PHONY: run3
run3:
	HTTP_PORT=50003 GRPC_PORT=50103 DISCOVERY_PORT=50203 RAFT_PORT=50303 RAFT_DIR=".db/s3/${UNIX_TIME}" go run ./example server

.PHONY: curl1
curl1:
	curl -v http://localhost:50101/message?partition=9&message=hello
