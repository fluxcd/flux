package main

import "os"

func main() {
	root := newRoot()

	service := newService(root)
	serviceList := newServiceList(service)
	serviceShow := newServiceShow(service)
	serviceRelease := newServiceRelease(service)

	image := newImage(root)
	imageList := newImageList(image)

	config := newConfig(root)
	configUpdate := newConfigUpdate(config)

	rootCmd := root.Command()

	serviceCmd := service.Command()
	serviceListCmd := serviceList.Command()
	serviceShowCmd := serviceShow.Command()
	serviceReleaseCmd := serviceRelease.Command()

	imageCmd := image.Command()
	imageListCmd := imageList.Command()

	configCmd := config.Command()
	configUpdateCmd := configUpdate.Command()

	serviceCmd.AddCommand(serviceListCmd, serviceReleaseCmd, serviceShowCmd)
	imageCmd.AddCommand(imageListCmd)
	configCmd.AddCommand(configUpdateCmd)

	rootCmd.AddCommand(serviceCmd, imageCmd, configCmd)

	if cmd, err := rootCmd.ExecuteC(); err != nil {
		switch err.(type) {
		case usageError:
			cmd.Println("")
			cmd.Println(cmd.UsageString())
		}
		os.Exit(1)
	}
}
