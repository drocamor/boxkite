package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"launchpad.net/goyaml"
	"os"
	"text/template"
)

type Task struct {
	Name       string
	Parameters map[string]string
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

func (t Task) doTask(params map[string]string) <-chan TaskResult {
	c := make(chan TaskResult)
	go func() {
		fmt.Println("Params:", params)
		if t.Name == "core.Exec" {

			var command bytes.Buffer

			template, err := template.New("command").Parse(t.Parameters["command"])
			if err != nil {
				panic(err)
			}

			err = template.Execute(&command, params)
			if err != nil {
				panic(err)
			}

			result := fmt.Sprintf("core.Exec: %s", command.String())

			c <- TaskResult{true, result}
		} else {
			n := loadNode(fmt.Sprintf("%s/%s.yaml", boxkitePath, t.Name))
			tc := n.doNode(params)
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
	// Make a chan
	c := make(chan TaskResult)

	// in a goroutine
	go func() {

		fmt.Println("Node name:", n.Name)

		// If there are tests, run the tests
		for _, test := range n.Tests {
			fmt.Println("Test:", test.Name)
			tc := test.doTask(test.Parameters)
			result := <-tc
			fmt.Println("--", result.Message)
		}

		// If the tests fail (or there are no tests), Run the steps
		for _, step := range n.Steps {
			fmt.Println("Step:", step.Name)

			sc := step.doTask(step.Parameters)
			result := <-sc
			fmt.Println("--", result.Message)

		}

		c <- TaskResult{true, "Hooray!"}
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
