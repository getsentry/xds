eds.yaml: eds.template.yaml
	jinja2 eds.template.yaml > eds.yaml

docker: clean
	docker build --pull --rm -t us.gcr.io/sentryio/xds:latest .

push: docker
	docker push us.gcr.io/sentryio/xds:latest

deploy: push
	kubectl -n sentry-system scale deployment xds --replicas=0
	kubectl -n sentry-system scale deployment xds --replicas=1

clean:
	rm -f ./xds

.PHONY: docker push deploy clean
