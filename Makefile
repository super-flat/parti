.PHONY: run1
run1:
	HTTP_PORT=50001 RAFT_PORT=50101 DISCOVERY_PORT=50201 go run ./example server

.PHONY: run2
run2:
	HTTP_PORT=50002 RAFT_PORT=50102 DISCOVERY_PORT=50202 go run ./example server

.PHONY: run3
run3:
	HTTP_PORT=50003 RAFT_PORT=50103 DISCOVERY_PORT=50203 go run ./example server

.PHONY: curl1
curl1:
	curl -v http://localhost:50101/message?partition=9&message=hello
