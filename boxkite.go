package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"launchpad.net/goyaml"
	"os"
)

type BoxkiteTask struct {
	Name       string
	Parameters map[string]string
}

type BoxkiteDefinition struct {
	Name  string
	Tests []BoxkiteTask
	Steps []BoxkiteTask
}

type TaskResult struct {
	Success bool
	Message string
}

var boxkitePath string

// Function takes
//  file path, params map
// returns a result chan (sends done to result chan)
func doTask(path string, params map[string]string) <-chan TaskResult {
	// Make a chan
	c := make(chan TaskResult)

	// in a goroutine
	go func() {
		file, e := ioutil.ReadFile(path)
		if e != nil {
			fmt.Printf("File error: %v\n", e)
			os.Exit(1)
		}

		var boxkiteNode BoxkiteDefinition

		err := goyaml.Unmarshal(file, &boxkiteNode)
		if err != nil {
			fmt.Printf("YAML error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("Node name:", boxkiteNode.Name)

		// If there are tests, run the tests
		for _, test := range boxkiteNode.Tests {
			fmt.Println("Test:", test.Name)
		}

		// If the tests fail (or there are no tests), Run the steps
		for _, step := range boxkiteNode.Steps {
			fmt.Println("Step:", step.Name)
			fmt.Println("Path is:", fmt.Sprintf("%s/%s.yaml", boxkitePath, step.Name))

			sc := doTask(fmt.Sprintf("%s/%s.yaml", boxkitePath, step.Name), step.Parameters)
			result := <-sc

			if result.Success {
				fmt.Println("SUCCESS:", result.Message)
			} else {
				fmt.Println("FAILURE:", result.Message)
			}
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

	c := doTask(flag.Arg(0), make(map[string]string))

	result := <-c

	if result.Success {
		fmt.Println("SUCCESS:", result.Message)
	} else {
		fmt.Println("FAILURE:", result.Message)
	}

}
