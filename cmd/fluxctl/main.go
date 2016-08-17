package main

import "os"

func main() {
	rootCmd := newRoot().Command()
	if cmd, err := rootCmd.ExecuteC(); err != nil {
		switch err.(type) {
		case usageError:
			cmd.Println("")
			cmd.Println(cmd.UsageString())
		}
		os.Exit(1)
	}
}
