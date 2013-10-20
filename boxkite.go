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

type TaskResult struct {
	Success bool
	Message string
	Summary string
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

func (t Task) doTask(params map[string]string, logChan chan TaskResult) TaskResult {
	var result TaskResult

	for i, arg := range t.Args {
		t.Args[i] = templatize(arg, params)
	}

	for k, v := range t.Parameters {
		t.Parameters[k] = templatize(v, params)
	}

	if t.Name == "core.Exec" {
		result.Summary = fmt.Sprintf("%s %s", t.Name, t.Args)
		cmd := exec.Command(t.Args[0], t.Args[1:]...)
		cmdOut, err := cmd.Output()

		result.Message = string(cmdOut)

		if err != nil {
			result.Success = false
		} else {
			result.Success = true
		}

	} else {
		result.Summary = fmt.Sprintf("%s[%s]", t.Name, params)
		n := loadNode(fmt.Sprintf("%s/%s.yaml", boxkitePath, t.Name))

		result = n.doNode(t.Parameters, logChan)

	}
	logChan <- result
	return result
}

func (n Node) runTests(params map[string]string, logChan chan TaskResult) bool {
	var testsPassed bool
	if len(n.Tests) > 0 {
		testResults := make([]bool, len(n.Tests))
		testsPassed = true

		for i, test := range n.Tests {

			r := test.doTask(params, logChan)
			testResults[i] = r.Success
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

func (n Node) runSteps(params map[string]string, logChan chan TaskResult) bool {
	for _, step := range n.Steps {

		rs := step.doTask(params, logChan)

		if rs.Success == false {
			return false

		}

	}
	return true
}

func (n Node) doNode(params map[string]string, logChan chan TaskResult) TaskResult {

	var result TaskResult
	result.Summary = fmt.Sprintf("%s %s", n.Name, params)

	testsPassed := n.runTests(params, logChan)

	if testsPassed == true {
		result.Success = true
		result.Message = fmt.Sprintf("Tests passed for %s", n.Name)
		logChan <- result
		return result
	}

	stepsComplete := n.runSteps(params, logChan)

	if stepsComplete == true {
		result.Success = true
		result.Message = fmt.Sprintf("Steps passed for %s", n.Name)
	} else {
		result.Success = false
		result.Message = fmt.Sprintf("Steps failed for %s", n.Name)
	}
	logChan <- result
	return result

}

func logger() chan TaskResult {
	logChan := make(chan TaskResult)
	var status string

	go func() {
		for {
			result := <-logChan
			if result.Success == true {
				status = "SUCCESS"
			} else {
				status = "FAILURE"
			}
			fmt.Printf("%s: (%s) %s\n", status, result.Summary, result.Message)
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
	result := n.doNode(make(map[string]string), logChan)

	logChan <- result

}
