package command

import (
	"errors"
	"fmt"
	"github.com/pufferpanel/pufferpanel/v3"
	"github.com/pufferpanel/pufferpanel/v3/logging"
	"github.com/pufferpanel/pufferpanel/v3/utils"
)

type Command struct {
	Commands  []string
	Env       map[string]string
	StdIn     pufferpanel.StdinConsoleConfiguration
	Variables map[string]interface{}
}

func (c Command) Run(args pufferpanel.RunOperatorArgs) pufferpanel.OperationResult {
	env := args.Environment

	for _, cmd := range c.Commands {
		logging.Info.Printf("Executing command: %s", cmd)
		env.DisplayToConsole(true, fmt.Sprintf("Executing: %s\n", cmd))
		cmdToExec, cmdArgs := utils.SplitArguments(cmd)
		ch := make(chan error, 1)
		err := env.Execute(pufferpanel.ExecutionData{
			Command:     cmdToExec,
			Arguments:   cmdArgs,
			Environment: c.Env,
			Callback: func(exitCode int) {
				if exitCode != 0 {
					ch <- errors.New("failed to run command")
				}
				ch <- nil
			},
			StdInConfig: c.StdIn,
			Variables:   c.Variables,
		})
		if err != nil {
			return pufferpanel.OperationResult{Error: err}
		}
		err = <-ch
		if err != nil {
			return pufferpanel.OperationResult{Error: err}
		}
	}

	return pufferpanel.OperationResult{Error: nil}
}
