install:
	go build -o $(HOME)/.config/kustomize/plugin/imranismail.dev/v2/externalsecret/ExternalSecret
test:
	rm -rf ./plugin
	go build -o ./plugin/imranismail.dev/v2/externalsecret/ExternalSecret
	cp ./main_test.go ./plugin/imranismail.dev/v2/externalsecret
	(cd ./plugin/imranismail.dev/v2/externalsecret && go test)
	rm -rf ./plugin