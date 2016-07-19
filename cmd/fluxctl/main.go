package main

import "os"

func main() {
	root := newRoot()
	images := newImages(root)
	service := newService(root)
	serviceList := newServiceList(service)
	serviceRelease := newServiceRelease(service)

	rootCmd := root.Command()
	imagesCmd := images.Command()
	serviceCmd := service.Command()
	serviceListCmd := serviceList.Command()
	serviceReleaseCmd := serviceRelease.Command()

	rootCmd.AddCommand(imagesCmd)
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(serviceListCmd)
	serviceCmd.AddCommand(serviceReleaseCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
