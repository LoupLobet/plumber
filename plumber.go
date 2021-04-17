package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
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
	AFFECT  = iota
	BLANK
	COMMENT
	ERROR
	PATTERN
)

type Msg struct {
	Data string
	Dst  string
	Src  string
	Type string
	Wdir string
}

type Pattern struct {
	Obj  string
	Verb string
	Arg  string
}

type Rule struct {
	Patterns  []Pattern
	Vars  Variables
	Value bool
}

type Variables map[string]string

const DefaultShell = "/bin/sh"
const DefaultShellOpts = "-c"

var LogFile   = flag.String("l", "/mnt/plumb/log", "log file")
var PlumbFile = flag.String("p", "/mnt/plumb/send", "plumb file")
var RulesFile = flag.String("r", "/mnt/plumb/rules", "rules file")
var DebugMode = flag.Bool("d", false, "debug mode")

func main() {
	var jsonMsg bytes.Buffer

	flag.Parse()

	// Select the log file.
	if *DebugMode {
		*LogFile = "/dev/stdout"
	}
	logFd, err := os.OpenFile(*LogFile, os.O_WRONLY | os.O_APPEND, os.ModeAppend)
	if err != nil {
		fmt.Printf("Plumber: Can't start: couldn't open the log file: %s\n", *LogFile)
	}
	defer logFd.Close()
	log.SetOutput(logFd)

	sendFd, err := os.OpenFile(*PlumbFile, os.O_RDONLY, os.ModeNamedPipe)
	if err != nil {
		log.Println(err)
	}
	defer sendFd.Close()
	// Listening loop.
	for {
		_, err := io.Copy(&jsonMsg, sendFd)
		if err != nil {
			log.Println(err)
			continue
		}
		if jsonMsg.Len() > 0 {
			go ProcessMsg(jsonMsg.Bytes())
			jsonMsg.Reset()
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func AffectVar(line string, rule *Rule) error {
	var name string
	var value string

	for i := 0; i < len(line); i++ {
		if line[i] == '=' {
			name = line[:i]
			value = line[i + 1:]
			if name == "" {
				return errors.New("Missing name is variable declaration")
			}
			break
		}
		if !IsAlphaNum(line[i]) {
			return errors.New("Invalid character '" + string(line[i]) +
			                  "' in variable declaration: " + line)
		}
	}
	value, err := Expand([]byte(value), &rule.Vars)
	if err != nil {
		return err
	}
	(*rule).Vars[name] = value
	return nil
}

func CookPattern(line string, pattern *Pattern) error {
	var tokNb int
	var isTok bool

	for i := 0; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' || tokNb == 2 && isTok {
			switch tokNb {
			case 0:
				(*pattern).Obj += string(line[i])
			case 1:
				(*pattern).Verb += string(line[i])
			case 2:
				(*pattern).Arg += string(line[i])
			}
			isTok = true
		} else if isTok && tokNb < 2 {
			tokNb++
			isTok = false
		}
	}
	if (*pattern).Arg == "" || (*pattern).Verb == "" || (*pattern).Obj == "" {
		return errors.New("Inconsistent pattern: " + line)
	}
	return nil
}

func EvalPattern(rule *Rule, i int) (bool, error) {
	var patternValue bool
	var err error

	pattern := (*rule).Patterns[i]
	pattern.Arg, err = Expand([]byte(pattern.Arg), &rule.Vars)
	if err != nil {
		return false, err
	}

	if pattern.Obj == "arg" || pattern.Obj == "data" || pattern.Obj == "dst" ||
	   pattern.Obj == "src" || pattern.Obj == "type" || pattern.Obj == "wdir" {
		switch pattern.Verb {
		case "is":
			patternValue = (pattern.Arg == (*rule).Vars[pattern.Obj])
		case "isn't":
			patternValue = (pattern.Arg != (*rule).Vars[pattern.Obj])
		case "set":
			(*rule).Vars[pattern.Obj] = pattern.Arg
			patternValue = true
		case "matches":
			re := regexp.MustCompile(pattern.Arg)
			patternValue = re.MatchString((*rule).Vars[pattern.Obj])
			if patternValue {
				submatch := re.FindStringSubmatch((*rule).Vars[pattern.Obj])
				// set $0 $1 ... $n
				for i := 0; i < len(submatch); i++ {
					(*rule).Vars[strconv.Itoa(i)] = submatch[i]
				}
			}
		case "isfile": fallthrough
		case "isdir":
			// If pattern.Arg isn't an absolute path,
			// concatenate it with the working directory.
			if len(pattern.Arg) > 0 && pattern.Arg[0] != '/' {
				pattern.Arg = (*rule).Vars["wdir"] + "/" + pattern.Arg
			}
			stat, err := os.Stat(pattern.Arg)
			if err != nil {
				break
			}
			mode := stat.Mode()
			if pattern.Verb == "isfile" && mode.IsRegular() ||
			   pattern.Verb == "isdir" && mode.IsDir() {
				patternValue = true
				(*rule).Vars[pattern.Obj] = pattern.Arg
			}
		default:
			err = errors.New("Inconsistent verb with object '" +
			                 pattern.Obj + "': " + pattern.Verb)
		}
	} else if pattern.Obj == "plumb" {
		switch pattern.Verb {
		case "start":
			PlumbStart(pattern.Arg, &rule.Vars)
			patternValue = true
		case "to":
			// If pattern.Arg isn't an absolute path,
			// concatenate it with the working directory.
			if len(pattern.Arg) > 0 && pattern.Arg[0] != '/' {
				pattern.Arg = (*rule).Vars["wdir"] + "/" + pattern.Arg
			}
			PlumbTo((*rule).Vars["arg"], pattern.Arg)
			patternValue = true
		default:
			err = errors.New("Inconsistent verb with object '" +
			                 pattern.Obj + "': " + pattern.Verb)
		}
	} else {
		err = errors.New("Unknow object: " + pattern.Obj)
		patternValue = false
	}
	return patternValue, err
}

func EvalRule(rule *Rule) {
	(*rule).Value = true
	for i := 0; i < len((*rule).Patterns) && (*rule).Value; i++ {
		patternValue, err := EvalPattern(rule, i)
		if err != nil {
			log.Println(err)
		}
		(*rule).Value = (*rule).Value && patternValue
	}
}

func LineType(line string) int {
	if line == "\n" || line == "" {
		return BLANK
	} else if line[0] == '#' {
		return COMMENT
	}
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' || line[i] == '\t' {
			return PATTERN
		} else if line[i] == '=' {
			return AFFECT
		}
	}
	return ERROR
}

func PlumbStart(command string, vars *Variables) {
	// Use the shell, and shell options defined in rules,
	// if they aren't defined, use the default ones.
	shell, shellExists := (*vars)["SHELL"]
	shellOpts, shellOptsExists := (*vars)["SHELL_OPTS"]
	if !shellExists || !shellOptsExists {
		shell = DefaultShell
		shellOpts = DefaultShellOpts
	}
	cmd := exec.Command(shell, shellOpts, command)
	err := cmd.Start()
	if err != nil {
		log.Println(err)
	}
}

func PlumbTo(text string, fileName string) {
	fd, err := os.OpenFile(fileName, os.O_CREATE | os.O_APPEND | os.O_WRONLY, 0777)
	if err != nil {
		log.Println(err)
	}
	defer fd.Close()
	fd.WriteString(text + "\n")
}

func ProcessMsg(jsonMsg []byte) {
	var msg Msg
	var rule Rule
	var ruleTemplate Rule
	var ruleErr bool
	var capturing bool

	// Unpack the json encoded message
	err := json.Unmarshal(jsonMsg, &msg)
	if err != nil {
		log.Println(err)
		return
	}

	// Create a rule template with message variables.
	ruleTemplate.Vars = Variables {
		"data": msg.Data, "dst": msg.Dst, "src": msg.Src,
		"type": msg.Type, "wdir": msg.Wdir,
	}

	rulesFd, err := os.Open(*RulesFile)
	if err != nil {
		log.Println(err)
		return
	}
	defer rulesFd.Close()

	scanner := bufio.NewScanner(rulesFd)
	for scanner.Scan() {
		err := scanner.Err()
		if err != nil {
			log.Println(err)
		}
		line := strings.Trim(scanner.Text(), " \t")

		switch LineType(line) {
		case AFFECT:
			// Add the variable to the rule template
			err := AffectVar(line, &ruleTemplate)
			if err != nil {
				log.Println(err)
			}
			ruleErr = true
		case BLANK:
			if capturing && !ruleErr && len(rule.Patterns) > 0 {
				EvalRule(&rule)
				if rule.Value {
					return
				}
				capturing = false
			}
			ruleErr = false
		case COMMENT: // Skip the line
		case PATTERN:
			// New rule encountered: initialize a new rule from the template
			if !capturing {
				rule = ruleTemplate
			}
			if ruleErr {
				break
			}
			var pattern Pattern
			err := CookPattern(line, &pattern)
			if err != nil {
				ruleErr = true
				log.Println(err)
				break
			}
			rule.Patterns = append(rule.Patterns, pattern)
			capturing = true
		case ERROR:
			ruleErr = true
		}
	}
}

