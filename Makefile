all: clean plumber plumb

plumber: plumber.go expand.go
	go build plumber.go expand.go

plumb: plumb.go
	go build plumb.go

install: all
	mkdir -p /mnt/plumb
	[ -e /mnt/plumb/send ] || mkfifo /mnt/plumb/send
	[ -e /mnt/plumb/log ] || touch /mnt/plumb/log
	[ -e /mnt/plumb/rules ] || cp rules /mnt/plumb/rules
	cp plumber /usr/local/bin/plumber
	cp plumb /usr/local/bin/plumb
	mkdir -p /usr/local/man/man1/
	cp plumb.1 /usr/local/man/man1/plumb.1
	cp plumber.1 /usr/local/man/man1/plumber.1

uninstall:
	rm -rf /mnt/plumb
	rm -f  /usr/local/bin/plumber
	rm -f  /usr/local/bin/plumb

clean:
	rm -f plumber
	rm -f plumb