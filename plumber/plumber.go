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
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	Blank   = iota
	Comment
	Macro
	Pattern
	Error
)

type Attribute struct {
	Key   string
	Value string
}

type RulePattern struct {
	Obj     string
	Verb    string
	Arg     string
}

type PlumbMsg struct {
	Attr []Attribute
	Data string
	Dst  string
	Src  string
	Type string
	Wdir string
}

type Vars map[string]string
type Macros = Vars

const Send = "/mnt/plumb/send"
const Rules = "/mnt/plumb/rules"
const Shell = "/bin/sh"
const ShellOpts = "-c"

func main() {
	var msg bytes.Buffer

	sendFd, err := os.OpenFile(Send, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		log.Fatal(err)
	}
	defer sendFd.Close()
	for {
		_, err := io.Copy(&msg, sendFd)
		if err != nil {
			log.Println(err)
			continue
		}
		if msg.Len() > 0 {
			go ProcessMsg(msg.Bytes())
			msg.Reset()
		}
		time.Sleep(50 * time.Millisecond)
	}

}

func CookPattern(line string, pattern *RulePattern) error {
	var toknb int
	var tokRecord bool

	for i := 0; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' || toknb == 2 && tokRecord  {
			switch toknb {
			case 0:
				(*pattern).Obj += string(line[i])
			case 1:
				(*pattern).Verb += string(line[i])
			case 2:
				(*pattern).Arg += string(line[i])
			}
			tokRecord = true
		} else if tokRecord && toknb < 2 {
			toknb++
			tokRecord = false
		}
	}
	if (*pattern).Arg == "" || (*pattern).Verb == "" || (*pattern).Obj == "" {
		return errors.New("Inconsistent pattern: " + line)
	}
	return nil
}

func EvalPattern(pattern RulePattern, msg *PlumbMsg, vars *Vars) (bool, error) {
	var patternValue bool
	var err error

	pattern.Arg, err = Expand([]byte(pattern.Arg), vars)
	if err != nil {
		return false, err
	}

	switch pattern.Obj {

	case "arg":  fallthrough
	case "data": fallthrough
	case "dst":  fallthrough
	case "src":  fallthrough
	case "type": fallthrough
	case "wdir":
		if pattern.Verb == "is" {
			patternValue = (pattern.Arg == (*vars)[pattern.Obj])
		} else if pattern.Verb == "isn't" {
			patternValue = (pattern.Arg != (*vars)[pattern.Obj])
		} else if pattern.Verb == "set" {
			(*vars)[pattern.Obj] = pattern.Arg
			patternValue = true
		} else if pattern.Verb == "matches" {
			re := regexp.MustCompile(pattern.Arg)
			patternValue = re.MatchString((*vars)[pattern.Obj])
			if patternValue {
				submatch := re.FindStringSubmatch((*vars)[pattern.Obj])
				// set $0 ... $9
				for i := 0; i < len(submatch) && i < 10; i++ {
					(*vars)[strconv.Itoa(i)] = submatch[i]
				}
			}
		} else if pattern.Verb == "isfile" || pattern.Verb == "isdir" {
		    stat, err := os.Stat(pattern.Arg)
			if err != nil {
				log.Println(err)
				break
			}
			mode := stat.Mode()
			if pattern.Verb == "isfile" && mode.IsRegular() ||
			   pattern.Verb == "isdir" && mode.IsDir() {
				patternValue = true
				(*vars)[pattern.Obj] = pattern.Arg
			}
		} else {
			err = errors.New("Inconsistent verb with object '" +
			                 pattern.Obj + "': " + pattern.Verb)
		}
		return patternValue, err

	case "plumb":
		if pattern.Verb == "start" {
			shell, shellExists := (*vars)["SHELL"]
			shellOpts, shellOptsExists := (*vars)["SHELLOPTS"]
			if !shellExists || !shellOptsExists {
				shell = Shell
				shellOpts = ShellOpts
			}
			cmd := exec.Command(shell, shellOpts, pattern.Arg)
			err := cmd.Start()
			//time.Sleep(2 * time.Second)
			if err != nil {
				log.Println(err)
			}
		} else {
			err = errors.New("Inconsistent verb with object '" +
			                 pattern.Obj + "': " + pattern.Verb)
		}
		return true, err
	}
	return false, nil
}

func LineType(line string) int {

	if line == "\n" || line == "" {
		return Blank
	}
	if len(line) > 0 && line[0] == '#' {
		return Comment
	}
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '\t' {
			return Pattern
		}
		if line[i] == '=' {
			return Macro
		}
	}
	return Error
}

func ProcessMsg(jsonMsg []byte) {
	//var macros = make(Macros)
	var rule []RulePattern
	var capturing bool
	var ruleError bool

	msg, err := UnpackPlumbMsg(jsonMsg)
	if err != nil {
		log.Println(err)
		return
	}
	var macros = Macros {
		"data": msg.Data, "dst": msg.Dst, "src": msg.Src,
		"type": msg.Type, "wdir": msg.Wdir,
	}

	rulesFd, err := os.Open(Rules)
	if err != nil {
		log.Println(err)
	}
	defer rulesFd.Close()

	scanner := bufio.NewScanner(rulesFd)
	for scanner.Scan() {
		err := scanner.Err()
		if err != nil {
			log.Println(err)
		}

		line := strings.Trim(scanner.Text(), "\t ")
		switch LineType(line) {
		case Blank:
			if capturing && !ruleError && len(rule) > 0 {
				// end of capture eval the rule
				// set some the special vars (i.e. $data $dst $src $wdir $type)
				vars := macros

				ruleValue := true
				for i := 0; i < len(rule) && ruleValue; i++ {
					patternValue, err := EvalPattern(rule[i], &msg, &vars)

					if err != nil {
						log.Println(err)
						// err != nil => patternValue == false
					}
					ruleValue = ruleValue && patternValue
				}
				//fmt.Println("vars:", vars)
				fmt.Println("ruleValue:", ruleValue)
				fmt.Println("$arg:", vars["arg"])
				rule = nil
			}
			ruleError = false
		case Comment:
		case Macro:
			err := SetMacro(line, &macros)
			if err != nil {
				log.Println(err)
			}
			// avoid macro definition in a rule
			ruleError = true
		case Pattern:
			if !ruleError {
				var pattern RulePattern
				capturing = true
				err := CookPattern(line, &pattern)
				if err != nil {
					ruleError = true
					log.Println(err)
					break
				}
				rule = append(rule, pattern)
			}
		case Error:
			ruleError = true
		}
	}
	fmt.Println("")
}

func SetMacro(line string, macros *Macros) error {
	var name string
	var value string

	for i := 0; i < len(line); i++ {
		if line[i] == '=' {
			name = line[:i]
			if i == 0 {
				return errors.New("Missing name in macro declaration")
			}
			if i == 1 && 0 <= name[0] && name[0] <= 9 {
				return errors.New("Invalid macro name: " + name)
			}
			if i < len(line) - 1 {
				value = line[i + 1:]
			} else {
				return errors.New("Missing value in macro declaration")
			}
			break
		}
		if !IsAlphaNum(line[i]) {
			return errors.New("Invalid '" + string(line[i]) +
			                  "' character in macro name: " + line)
		}
	}
	value, err := Expand([]byte(value), macros)
	if err != nil {
		return err
	}
	(*macros)[name] = value
	return nil
}

func UnpackPlumbMsg(jsonMsg []byte) (PlumbMsg, error) {
	var msg PlumbMsg

	err := json.Unmarshal(jsonMsg, &msg)
	return msg, err
}

