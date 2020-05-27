IMAGE   ?= quay.io/srust/wordpress-operator
VERSION ?= 0.0.1

build:
	operator-sdk build $(IMAGE):$(VERSION)

push:
	docker push $(IMAGE):$(VERSION)

.PHONY: build push
