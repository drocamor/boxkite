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

		if t.Name == "core.Exec" {
			command := templatize(t.Parameters["command"], params)

			result := fmt.Sprintf("core.Exec: %s", command)

			c <- TaskResult{true, result}
		} else {
			n := loadNode(fmt.Sprintf("%s/%s.yaml", boxkitePath, t.Name))

			for k, v := range t.Parameters {

				t.Parameters[k] = templatize(v, params)

			}

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
		for _, test := range n.Tests {
			fmt.Println(n.Name, "- Test:", test.Name)
			tc := test.doTask(params)
			result := <-tc
			fmt.Println("--", result.Message)
		}

		for _, step := range n.Steps {
			fmt.Println(n.Name, "- Step:", step.Name)

			sc := step.doTask(params)
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
