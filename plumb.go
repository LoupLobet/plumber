package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

type PlumbMsg struct {
	Data   string
	Dst    string
	Src    string
	Type   string
	Wdir   string
}

const PlumbFile = "/mnt/plumb/send"

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