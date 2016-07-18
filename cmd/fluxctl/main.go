package main

import "os"

func main() {
	root := newRoot()

	service := newService(root)
	serviceList := newServiceList(service)
	serviceImages := newServiceImages(service)
	serviceRelease := newServiceRelease(service)
	repo := newRepo(root)
	repoImages := newRepoImages(repo)

	rootCmd := root.Command()

	serviceCmd := service.Command()
	serviceListCmd := serviceList.Command()
	serviceImagesCmd := serviceImages.Command()
	serviceReleaseCmd := serviceRelease.Command()
	repoCmd := repo.Command()
	repoImagesCmd := repoImages.Command()

	serviceCmd.AddCommand(serviceListCmd, serviceReleaseCmd, serviceImagesCmd)
	repoCmd.AddCommand(repoImagesCmd)

	rootCmd.AddCommand(serviceCmd, repoCmd)

	if cmd, err := rootCmd.ExecuteC(); err != nil {
		switch err.(type) {
		case *usageError:
			cmd.Println("")
			cmd.Println(cmd.UsageString())
		}
		os.Exit(1)
	}
}
