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

func (t Task) doTask(params map[string]string) <-chan TaskResult {
	c := make(chan TaskResult)
	go func() {
		fmt.Println("In Task:", t.Name)
		fmt.Println("Params are:", params)

		for i, arg := range t.Args {
			t.Args[i] = templatize(arg, params)
		}

		for k, v := range t.Parameters {
			t.Parameters[k] = templatize(v, params)
		}

		if t.Name == "core.Exec" {

			cmd := exec.Command(t.Args[0], t.Args[1:]...)
			cmdOut, err := cmd.Output()

			if err != nil {
				c <- TaskResult{false, string(cmdOut)}
			} else {
				c <- TaskResult{true, string(cmdOut)}
			}

		} else {
			n := loadNode(fmt.Sprintf("%s/%s.yaml", boxkitePath, t.Name))

			tc := n.doNode(t.Parameters)

			result := <-tc
			if result.Success {
				fmt.Println("SUCCESS:", result.Message)
			} else {
				fmt.Println("FAILURE:", result.Message)
			}
			c <- result
		}
	}()
	return c
}

func (n Node) doNode(params map[string]string) <-chan TaskResult {

	c := make(chan TaskResult)

	go func() {

		fmt.Println("In Node:", n.Name)
		fmt.Println("Params are:", params)

		var testsPassed bool

		if len(n.Tests) > 0 {
			testResults := make([]bool, len(n.Tests))
			testsPassed = true

			for i, test := range n.Tests {
				fmt.Println(n.Name, "- Test:", test.Name)
				tc := test.doTask(params)
				result := <-tc
				testResults[i] = result.Success
			}

			for _, testResult := range testResults {
				if testResult == false {
					testsPassed = false
					break
				}
			}
			c <- TaskResult{true, "Tests passed, no action taken!"}
		} else {
			testsPassed = false
		}

		if testsPassed == false {

			stepsPassed := true

			for _, step := range n.Steps {
				fmt.Println(n.Name, "- Step:", step.Name)

				sc := step.doTask(params)
				result := <-sc
				fmt.Println("--", result.Message)

				if result.Success == false {
					c <- TaskResult{false, fmt.Sprintf("\"%s\" failed. Step \"%s\" - \"%s\".", n.Name, step.Name, result.Message)}
					stepsPassed = false
					break
				}

			}
			if stepsPassed == true {
				c <- TaskResult{true, fmt.Sprintf("\"%s\" succeeded.", n.Name)}
			}
		}

	}()
	return c
}

func main() {
	flag.StringVar(&boxkitePath, "b", "/etc/boxkite", "Directory where Boxkite files live")

	flag.Parse()

	// You must provide at least one boxkite file to start with
	if len(flag.Args()) != 1 {
		flag.Usage()
		os.Exit(1)
	}

	fmt.Println("Path is:", boxkitePath)
	fmt.Println("Root is:", flag.Arg(0))
	n := loadNode(flag.Arg(0))
	c := n.doNode(make(map[string]string))

	result := <-c

	if result.Success {
		fmt.Println("SUCCESS:", result.Message)
	} else {
		fmt.Println("FAILURE:", result.Message)
	}

}
