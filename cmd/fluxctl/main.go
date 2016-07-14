package main

import "os"

func main() {
	root := rootCommand()
	service := serviceCommand(root)
	images := imagesCommand(root)
	serviceList := serviceListCommand(service)
	serviceRelease := serviceReleaseCommand(service)

	rootCmd := root.Command()
	imagesCmd := images.Command()
	serviceCmd := service.Command()
	serviceListCmd := serviceList.Command()
	serviceReleaseCmd := serviceRelease.Command()

	rootCmd.AddCommand(imagesCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(imagesCommand(rootOpts))
	serviceCmd.AddCommand(serviceListCmd)
	serviceCmd.AddCommand(serviceReleaseCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
