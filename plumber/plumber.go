package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

type Attribute struct {
	Key   string
	Value string
}

type RulesPattern struct {
	Obj  string
	Verb string
	Arg  string
}

type PlumbMsg struct {
	Attr   []Attribute
	Data   string
	Dst    string
	Src    string
	Type   string
	Wdir   string
}


const Send = "/mnt/plumb/send"

func main() {
	HandleMsg(Send)
}

func HandleMsg(sendFile string) {
	var msg bytes.Buffer

	sendFd, err := os.OpenFile(sendFile, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		log.Fatal(err)
	}
	defer sendFd.Close()
	for {
		_, err := io.Copy(&msg, sendFd)
		if err != nil {
			log.Fatal(err)
		}
		if msg.Len() > 0 {
			go UnpackPlumbMsg(msg.Bytes())
			msg.Reset()
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func UnpackPlumbMsg(jsonMsg []byte) {
	var msg PlumbMsg

	err := json.Unmarshal(jsonMsg, &msg)
	if err != nil {
		log.Println(err)
	}
}

func ParseRules(rulesFile string, msg *PlumbMsg) error {
	var pattern RulesPattern

	rulesFd, err := os.OpenFile(rulesFile, os.O_RDONLY)
	if err != nil {
		log.Println(err)
	}
	scanner := bufio.NewScanner(rulesFd)
	while scanner.Scan() {
		if scanner.Err() != nil {
			return errors.New("could not read rules")
		}
		// Parse line
		obj :=
		line := scanner.Text()
		object := line[:strings.Index(line, " ")]
		switch line[:sep] {
		case "type":

		case "data":
		case "arg":
		case "plumb"
		case "dst":
		case "src":
		case "wdir":
		case "attr":
		}
	}
}