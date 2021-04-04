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
	"regexp"
	"strings"
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
	err = ParseRules("/mnt/plumb/rules", &msg)
	if err != nil {
		log.Println(err)
	}
}

func ParseRules(rulesFile string, msg *PlumbMsg) error {
	var pattern RulesPattern
	var matchRules = true

	rulesFd, err := os.Open(rulesFile)
	if err != nil {
		log.Println(err)
	}
	scanner := bufio.NewScanner(rulesFd)
	for i := 0; scanner.Scan(); i++ {
		if scanner.Err() != nil {
			return errors.New("could not read rules")
		}
		line := string(scanner.Text())
		
		// Skip blank lines
		if line == "\n" {
			continue
		} else {
			// Ignore commented lines and lines full of spaces and tabs		
			ignore := true
			for j := 0; j < len(line); j++ {
				if line[j] == ' ' {
					continue
				}
				if line[j] == '#' {
					break
				} else if line[j] != ' ' && line[j] != '\t' {
					ignore = false
					line = line[j:]
					break
				}
			}
			if ignore {
				// finaly skip the line
				continue
			}

		}
		
		// Parse line
		sep := strings.Index(line, " ")
		
		
		
		
		
		
		if sep == -1 {
			errmsg := fmt.Sprintf("inconsistent rule pattern: line %d", i)
			return errors.New(errmsg)
		}
		pattern.Obj = line[:sep]
		line = line[sep + 1:]
		sep = strings.Index(line, " ")
		if sep == -1 {
			errmsg := fmt.Sprintf("inconsistent rule pattern: line %d", i)
			return errors.New(errmsg)
		}
		pattern.Verb = line[:sep]
		pattern.Arg = line[sep + 1:]
		switch pattern.Obj {
		case "type":
			if pattern.Verb == "is" {
				if pattern.Arg != msg.Type {
					matchRules = false
				}
			} else if pattern.Verb == "isn't" {
				if pattern.Arg == msg.Type {
					matchRules = false
				}
			} else {
				errmsg := fmt.Sprintf("unknow verb, line %d: %s", i, pattern.Verb)
				return errors.New(errmsg)
			}
		case "data":
			if pattern.Verb == "set" {
				// check for valid variable name (i.e. shell var syntax)
				if pattern.Arg[0]
			}
		case "arg":
		case "plumb"
		case "dst":
		case "src":
		case "wdir":
		case "attr":
		}
	}
	return nil
}
