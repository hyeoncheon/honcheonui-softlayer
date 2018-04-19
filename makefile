DEST=../honcheonui

run: test softlayer.so
	./test

softlayer.so: softlayer.go
	go build -buildmode=plugin -o softlayer.so softlayer.go

test: tester/test.go
	go build -o test tester/test.go

install: softlayer.so
	install -s -m 644 softlayer.so $(DEST)/plugins/provider-softlayer.so
