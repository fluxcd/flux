package main

import "os"

func main() {
	root := newRoot()

	service := newService(root)
	serviceList := newServiceList(service)
	serviceImages := newServiceImages(service)
	serviceRelease := newServiceRelease(service)
	serviceStatus := newServiceStatus(service)

	repo := newRepo(root)
	repoImages := newRepoImages(repo)

	config := newConfig(root)
	configUpdate := newConfigUpdate(config)

	rootCmd := root.Command()

	serviceCmd := service.Command()
	serviceListCmd := serviceList.Command()
	serviceImagesCmd := serviceImages.Command()
	serviceReleaseCmd := serviceRelease.Command()
	serviceStatusCmd := serviceStatus.Command()

	repoCmd := repo.Command()
	repoImagesCmd := repoImages.Command()

	configCmd := config.Command()
	configUpdateCmd := configUpdate.Command()

	serviceCmd.AddCommand(serviceListCmd, serviceReleaseCmd, serviceImagesCmd, serviceStatusCmd)
	repoCmd.AddCommand(repoImagesCmd)
	configCmd.AddCommand(configUpdateCmd)

	rootCmd.AddCommand(serviceCmd, repoCmd, configCmd)

	if cmd, err := rootCmd.ExecuteC(); err != nil {
		switch err.(type) {
		case usageError:
			cmd.Println("")
			cmd.Println(cmd.UsageString())
		}
		os.Exit(1)
	}
}
