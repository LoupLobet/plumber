package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

type Attribute struct {
	Key   string
	Value string
}

type PlumbMsg struct {
	Attr   []Attribute
	Data   string
	Dst    string
	Src    string
	Type   string
	Wdir   string
}

const PlumbFile = "/mnt/plumb/send"

var attrFlag  = flag.String("a", "", "attributes")
var dstFlag   = flag.String("d", "", "destination")
var plumbFlag = flag.String("p", PlumbFile, "plumb file")
var srcFlag   = flag.String("s", "plumb", "source")
var stdinFlag = flag.Bool("i", false, "read data from standard input")
var typeFlag  = flag.String("t", "text", "type")
var wdirFlag  = flag.String("w", "", "working directory")

func main() {
	var msg PlumbMsg

	flag.Parse()
	if err := CookPlumbMsg(&msg); err != nil {
		log.Fatal(err)
	}

	plumbWr, err := os.OpenFile(*plumbFlag, os.O_WRONLY, os.ModeNamedPipe)
	if err != nil {
		log.Fatal(err)
	}
	defer plumbWr.Close()

	// encode PlumbMsg in json and write it to the send pipe
	jsonPackage, err := json.Marshal(msg)
	if err != nil {
		log.Fatal(err)
	}
	_, err = plumbWr.Write(jsonPackage)
	if err != nil {
		log.Fatal(err)
	}
}

func CookPlumbMsg(msg *PlumbMsg) error {
	var buf bytes.Buffer

	(*msg).Dst = *dstFlag
	(*msg).Src = *srcFlag
	(*msg).Type = *typeFlag
	if len(*attrFlag) > 0 {
		err := ParseAttributes(*attrFlag, msg)
		if err != nil {
			return err
		}
	}
	if len(*wdirFlag) > 0 {
		(*msg).Wdir = *wdirFlag
	} else {
		wdir, err := os.Getwd()
		if err != nil {
			fmt.Println("could not get working directory")
		} else {
			(*msg).Wdir = wdir
		}
	}
	if *stdinFlag {
		if _, err := io.Copy(&buf, os.Stdin); err != nil {
			return err
		}
		if buf.Len() > 0 {
			(*msg).Data = buf.String() + "\n"
		}
	} else if len(flag.Args()) > 0 {
		(*msg).Data = strings.Join(flag.Args(), " ")
	} else {
		log.Fatalln("no data to send to the plumber")
	}
	return nil
}

func ParseAttributes(str string, msg *PlumbMsg) error {
	tokens := strings.Split(str, " ")
	(*msg).Attr = make([]Attribute, len(tokens))
	for i, buf := range tokens {
		if len(buf) < 3 {
			// need at least 3 byte (e.g. a=0)
			return errors.New("unconsistent attribute: " + buf)
		}
		// The attribute key must be alpha, so sep should
		// be the index of the "=" separator.
		sep := strings.Index(buf, "=")
		if sep < 1 || sep == len(buf) - 1 {
			return errors.New("inconsistent attribute: " + buf)
		}
		(*msg).Attr[i].Key = buf[:sep]
		(*msg).Attr[i].Value = buf[sep + 1:]
	}
	return nil
}
