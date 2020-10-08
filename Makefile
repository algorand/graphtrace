all:	cmd/traceclient/traceclient cmd/tracecollector/tracecollector

cmd/traceclient/traceclient:	.PHONY
	cd cmd/traceclient && CGO_ENABLED=0 go build
cmd/tracecollector/tracecollector:	.PHONY
	cd cmd/tracecollector && CGO_ENABLED=0 go build
.PHONY:
