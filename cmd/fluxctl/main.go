package main

import "os"

func main() {
	root := newRoot()
	service := newService(root)
	images := newImages(root)
	serviceList := newServiceList(service)
	serviceImages := newServiceImages(service)
	serviceRelease := newServiceRelease(service)

	rootCmd := root.Command()
	imagesCmd := images.Command()
	serviceCmd := service.Command()
	serviceListCmd := serviceList.Command()
	serviceImagesCmd := serviceImages.Command()
	serviceReleaseCmd := serviceRelease.Command()

	rootCmd.AddCommand(imagesCmd)
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(serviceListCmd)
	serviceCmd.AddCommand(serviceImagesCmd)
	serviceCmd.AddCommand(serviceReleaseCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
