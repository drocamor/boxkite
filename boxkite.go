package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"launchpad.net/goyaml"
	"os"
	"os/exec"
	"text/template"
)

type Task struct {
	Name       string
	Parameters map[string]string
	Args       []string
}

type Node struct {
	Name  string
	Tests []Task
	Steps []Task
}

// unused now, but soon...
type MessageType int

const (
	TaskSuccess MessageType = iota
	TaskFailure
	EnteringNode
	TestsPassed
)

type LogMessage struct {
	Message string
	Type    MessageType
}

var boxkitePath string

func loadNode(path string) Node {
	file, e := ioutil.ReadFile(path)
	if e != nil {
		fmt.Printf("File error: %v\n", e)
		os.Exit(1)
	}

	var n Node

	err := goyaml.Unmarshal(file, &n)
	if err != nil {
		fmt.Printf("YAML error: %v\n", err)
		os.Exit(1)
	}
	return n
}

func templatize(s string, p map[string]string) string {
	var result bytes.Buffer

	template, err := template.New("result").Parse(s)
	if err != nil {
		panic(err)
	}

	err = template.Execute(&result, p)
	if err != nil {
		panic(err)
	}
	return result.String()
}

func (t Task) doTask(params map[string]string, logChan chan LogMessage) (success bool, message string) {

	var summary string

	for i, arg := range t.Args {
		t.Args[i] = templatize(arg, params)
	}

	for k, v := range t.Parameters {
		t.Parameters[k] = templatize(v, params)
	}

	if t.Name == "core.Exec" {
		summary = fmt.Sprintf("%s %s", t.Name, t.Args)
		cmd := exec.Command(t.Args[0], t.Args[1:]...)
		cmdOut, err := cmd.Output()

		message = string(cmdOut)

		if err != nil {
			success = false
		} else {
			success = true
		}

	} else {
		summary = fmt.Sprintf("%s[%s]", t.Name, params)
		n := loadNode(fmt.Sprintf("%s/%s.yaml", boxkitePath, t.Name))

		success, message = n.doNode(t.Parameters, logChan)

	}

	if success == true {
		logChan <- LogMessage{Message: summary, Type: TaskSuccess}
	} else {
		logChan <- LogMessage{Message: summary, Type: TaskFailure}
	}

	return success, message
}

func (n Node) runTests(params map[string]string, logChan chan LogMessage) bool {
	var testsPassed bool

	if len(n.Tests) > 0 {
		testResults := make([]bool, len(n.Tests))
		testsPassed = true

		for i, test := range n.Tests {

			testResults[i], _ = test.doTask(params, logChan)
		}

		for _, testResult := range testResults {
			if testResult == false {
				testsPassed = false
				break
			}
		}

	}
	return testsPassed
}

func (n Node) runSteps(params map[string]string, logChan chan LogMessage) bool {
	for _, step := range n.Steps {

		r, _ := step.doTask(params, logChan)

		if r == false {
			return false

		}

	}
	return true
}

func (n Node) doNode(params map[string]string, logChan chan LogMessage) (success bool, message string) {

	logChan <- LogMessage{Message: n.Name, Type: EnteringNode}

	message = fmt.Sprintf("%s %s", n.Name, params)

	testsPassed := n.runTests(params, logChan)

	if testsPassed == true {
		success = true
		message = fmt.Sprintf("Tests passed for %s", n.Name)

		logChan <- LogMessage{Message: message, Type: TestsPassed}
		return true, message
	}

	stepsComplete := n.runSteps(params, logChan)

	if stepsComplete == true {
		success = true
		message = fmt.Sprintf("Steps passed for %s", n.Name)
	} else {
		success = false
		message = fmt.Sprintf("Steps failed for %s", n.Name)
	}

	return success, message

}

func logger() chan LogMessage {
	logChan := make(chan LogMessage)
	var status string

	go func() {
		for {
			m := <-logChan

			switch m.Type {
			case TaskSuccess:
				status = "SUCCESS"
			case TaskFailure:
				status = "FAILURE"
			case EnteringNode:
				status = "In Node"
			case TestsPassed:
				status = "Tests Passed"
			}
			fmt.Printf("%s: %s\n", status, m.Message)
		}
	}()
	return logChan
}

func main() {
	flag.StringVar(&boxkitePath, "b", "/etc/boxkite", "Directory where Boxkite files live")

	flag.Parse()

	// You must provide at least one boxkite file to start with
	if len(flag.Args()) != 1 {
		flag.Usage()
		os.Exit(1)
	}

	logChan := logger()

	n := loadNode(flag.Arg(0))
	_, _ = n.doNode(make(map[string]string), logChan)

}
