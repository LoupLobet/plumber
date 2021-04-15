all: clean plumber plumb

plumber: plumber.go expand.go
	go build plumber.go expand.go

plumb: plumb.go
	go build plumb.go

install: all
	cp plumber /usr/local/bin/plumber
	cp plumb /usr/local/bin/plumb

clean:
	rm -f plumber
	rm -f plumb