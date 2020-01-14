package main

import (
	"fmt"
	"os"
	"os/exec"

	"gtoc/docopt"
	"github.com/leaanthony/mewn"
	"github.com/wailsapp/wails"
	"go.uber.org/zap"
)

func basic() string {
	return "World!"
}

func Pretty_print(pat *docopt.Pattern) {
	pretty_print(pat, "")
}

func pretty_print(pat *docopt.Pattern, tabs string) {
	var pat_type = pat.T.String()
	if pat_type == "argument" || pat_type == "command" {
		fmt.Printf("%s%s(%s, %+v)\n", tabs, pat_type, pat.Name, pat.Value)
	} else if pat_type == "option" {
		fmt.Printf("%s%s(%s, %s, %d, %+v)\n", tabs, pat.T, pat.Short, pat.Long, pat.Argcount, pat.Value)
	} else {
		fmt.Printf("%s%s\n", tabs, pat_type)
		for _, child := range pat.Children {
			pretty_print(child, tabs+"   ")
		}
	}
}

func get_pattern(command string) (*docopt.Pattern, error) {
	zap.S().Debug("Trying with --help option")
	var output, err = exec.Command("sh", "-c", command, "--help").Output()
	if err != nil {
		zap.S().Warnf("Executing the command '%s --help' failed: %s", command, err)
		zap.S().Debug("Trying with -h option")
		output, err = exec.Command("sh", "-c", command, "-h").Output()
		if err != nil {
			return nil, fmt.Errorf("Executing the command '%s -h' failed: %s", command, err)
		}
	}
	var pat *docopt.Pattern
	pat, err = docopt.ParsePattern(string(output))
	if err != nil {
		return nil, fmt.Errorf("Parsing pattern failed:\n%s", err)
	}
	Pretty_print(pat)
	return pat, err
}

func main() {
	// Initializes the global logger
	plain, err := zap.NewDevelopment()
	if err != nil {
		fmt.Printf("can't initialize zap logger: %v", err)
		os.Exit(1)
	}
	defer plain.Sync()
	zap.ReplaceGlobals(plain)

	pat, err := get_pattern("./test.sh")
	if err != nil {
		zap.S().Errorf("Getting pattern failed: %s", err)
	}
	Pretty_print(pat)

	// if len(argv) == 0 {
	// 	zap.S().Fatal("No command is entered. exiting...")
	// } else if len(argv) == 1 {
	// 	zap.S().Debugf("Executing command: %s", argv[0])
	// 	var output, err = exec.Command("sh", "-c", argv[0], "--help").Output()
	// 	if err != nil {
	// 		zap.S().Debugf("Error occurred when executing the command: %s --help", argv[0])
	// 	}
	// 	zap.S().Debugf("The help message is:\n%s", output)
	// 	return
	// } else {
	// 	zap.S().Fatal("Multiple commands are entered. exiting...")
	// }

	js := mewn.String("./frontend/build/static/js/main.js")
	css := mewn.String("./frontend/build/static/css/main.css")

	app := wails.CreateApp(&wails.AppConfig{
		Width:  1024,
		Height: 768,
		Title:  "cli2gui",
		JS:     js,
		CSS:    css,
		Colour: "#242424",
	})
	app.Bind(basic)
	app.Bind(get_pattern)
	app.Run()

	// // print after flat (flat seems to return leaves only)
	// var patternList docopt.PatternList
	// patternList, err = pat.Flat(0)
	// for _, pat := range patternList {
	// 	fmt.Println(pat.T.String())
	// }
	// // fmt.Println(pat)
	// fmt.Println("hello world")
}
