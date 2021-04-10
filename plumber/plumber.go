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
//	"strconv"
	"strings"
	"time"
)

type Attribute struct {
	Key   string
	Value string
}

type RulePattern struct {
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
	ParseRules("/mnt/plumb/rules", &msg)
	fmt.Println("----------------")
}

func ParseRules(rulesFile string, msg *PlumbMsg) {
	var rule []string
	var capturing bool

	rulesFd, err := os.Open(rulesFile)
	if err != nil {
		log.Println(err)
	}
	scanner := bufio.NewScanner(rulesFd)

	for scanner.Scan() {
		if scanner.Err() != nil {
			log.Println("could not read rules")
			return
		}

		line, isPattern := IsPattern(scanner.Text())
		if isPattern {
			// capture patterns to make a rule
			rule = append(rule, line)
			capturing = true
		} else if capturing == true {

			// end of capture proceed to parsing
			variables := make(map[string]string)

			ruleValue, err := EvalRule(&rule, msg, &variables)
			if err != nil {
				log.Println(err)
			}
			fmt.Println(ruleValue)
			rule = nil
			capturing = false
		}
	}
}

func EvalRule(rule *[]string, msg *PlumbMsg, variables *map[string]string) (bool, error) {
	var pattern RulePattern
	var err error
	var ruleValue = true
	var patternValue bool

	for _, line := range(*rule) {
		err = CookPattern(line, &pattern)
		if err != nil {
			return false, err
		}
		patternValue, err = EvalPattern(&pattern, msg, variables)
		if err != nil {
			return false, err
		}
		ruleValue = ruleValue && patternValue
	}

	return ruleValue, err
}

func EvalPattern(pattern *RulePattern, msg *PlumbMsg, variables *map[string]string) (bool, error) {
	var patternValue = true
	var err error

	switch (*pattern).Obj {
	case "type":
		if (*pattern).Verb == "is" {
			patternValue = ((*pattern).Arg == msg.Type)
		} else if (*pattern).Verb == "isn't" {
			patternValue = ((*pattern).Arg != msg.Type)
		} else {
			err = errors.New(fmt.Sprintf("unknow verb: %s", (*pattern).Verb))
		}
		return patternValue, err

	case "data":
		fmt.Println(CookArg((*pattern).Arg, variables))
		if (*pattern).Verb == "set" {
			(*variables)["data"] = (*msg).Data
		} else if (*pattern).Verb == "matches" {
			re := BuildRegexp((*pattern).Arg, variables)
			patternValue = re.MatchString((*msg).Data)
		} else {
			err = errors.New(fmt.Sprintf("unknow verb: %s", (*pattern).Verb))
		}
		return patternValue, err

	case "dst":
		if (*pattern).Verb == "is" {
			patternValue = ((*pattern).Arg == msg.Dst)
		} else if (*pattern).Verb == "isn't" {
			patternValue = ((*pattern).Arg != msg.Dst)
		} else {
			err = errors.New(fmt.Sprintf("unknow verb: %s", (*pattern).Verb))
		}
		return patternValue, err

//	case "arg":
//		if (*pattern).Verb == "isfile" {
//			// Check for a valid arg variable name (i.e. '\$[0-9]')
//			if (*pattern).Arg[0] != '$' {
//				err = errors.New(fmt.Sprintf("invalid arg variable: %s", (*pattern).Arg))
//			} else {
//				argIndex, err := strconv.Atoi((*pattern).Arg[1:])
//				// split msg.Data into an argument array
//
//			}
//
//		} else {
//			err = errors.New(fmt.Sprintf("unknow verb: %s", (*pattern).Verb))
//		}
//		return patternValue, err
	}
	// temp return (compiler)
	return true, nil
}



func CookPattern(line string, pattern *RulePattern) error {
	var bufPattern RulePattern
	var i int
	var reterr error

	// Leading and trailing whitspaces and tabs have
	// already been removed by IsPattern().

	// Object
	for i = 0; i < len(line) && line[i] != ' ' && line[i] != '\t'; i++ {
		bufPattern.Obj += string(line[i])
	}
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	// Verb
	for ; i < len(line) && line[i] != ' ' && line[i] != '\t'; i++ {
		bufPattern.Verb += string(line[i])
	}
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	// Argument
	bufPattern.Arg = line[i:]
	if bufPattern.Obj == " " || bufPattern.Arg == " " || bufPattern.Arg == " " {
		reterr = errors.New("inconsitent rule pattern")
	}

	*pattern = bufPattern
	return reterr
}

func IsPattern(line string) (string, bool) {

	if line == "\n" {
		return "", false
	}
	// Ignore comments and format lines (i.e. line onlycomposed
	// of tabs and spaces). If the line is not skipped,
	// it is return without leading and trailing whitespaces.
	ignore := true
	for i := 0; i < len(line); i++ {
		if line[i] == ' ' {
			continue
		}
		if line[i] == '#' {
			break
		} else if line[i] != ' ' && line[i] != '\t' {
			ignore = false
			// remove leading
			line = line[i:]
			break
		}
	}
	if ignore {
		return line, false
	}
	// remove trailing
	for i := len(line) - 1; i > -1; i-- {
		if line[i] != ' ' && line[i] != '\t' {
			break
		}
	}
	return line, true
}

func BuildRegexp(str string, variables *map[string]string) (*regexp.Regexp) {
	var restr string
	var quoted bool

	// combine regex + raw text (e.g. '([a-zA-z]').png)
	for i := 0; i < len(str); i++ {
		if (i == 0 && str[i] == '\'') ||
		   (str[i] == '\'' && str[i - 1] != '\\') {
			quoted = !quoted
		} else {
			// eval variable
			if !quoted && str[i] == '$' {
				endOfVar := strings.Index(str[i:], " ")
				if endOfVar == -1 {
					endOfVar = len(str)
				}
				varName := str[i:endOfVar]
				varValue, exists := (*variables)[varName]
				if exists {
					restr += varValue
				}
				// Remove the variable in the raw text.
				// If the vriable doesn't exist, replace it by nothing.
				i = endOfVar - 1
			} else {
				restr += string(str[i])
			}
		}
	}
	return regexp.MustCompile(restr)
}

func CookArg(arg string, variables *map[string]string) (string, error) {
	var quoted bool
	var escaped bool

	reVar := regexp.MustCompile(`^\$([a-zA-Z0-9_]+)`)
	reBraceVar := regexp.MustCompile(`^\$\{([a-zA-Z0-9_]+)\}`)
	for i := 0; i < len(arg); i++ {
		if !quoted && !escaped && arg[i] == '\\' {
			escaped = true
			continue
		}
		if (i == 0 && arg[i] == '\'') ||
	       (arg[i] == '\'' && arg[i - 1] != '\\') {
			quoted = !quoted
		} else {
			if !quoted && !escaped && arg[i] == '$' {
				// eval variable
				varNames := reVar.FindAllStringSubmatch(arg[i:], -1)
				braceVarNames := reBraceVar.FindAllStringSubmatch(arg[i:], -1)

				if len(varNames) > 0 {
					varValue, exists := (*variables)[varNames[0][1]]
					if !exists {
						varValue = ""
					}
					buf := reVar.ReplaceAllString(arg[i:], varValue)
					arg = arg[:i] + buf
				} else if len(braceVarNames) > 0 {
					varValue, exists := (*variables)[braceVarNames[0][1]]
					if !exists {
						varValue = ""
					}
					buf := reBraceVar.ReplaceAllString(arg[i:], varValue)
					arg = arg[:i] + buf
				} else {
					errmsg := fmt.Sprintf("invalid variable here: %s", arg[i:])
					return "", errors.New(errmsg)
				}
			}
		}
		escaped = false
	}
	if quoted {
		errmsg := fmt.Sprintf("expected ' not newline")
		return "", errors.New(errmsg)
	}
	return arg, nil
}