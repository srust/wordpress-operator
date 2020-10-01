IMAGE   ?= quay.io/srust/wordpress-operator
VERSION ?= better

build:
	operator-sdk build $(IMAGE):$(VERSION)

push:
	docker push $(IMAGE):$(VERSION)

.PHONY: build push
