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

	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(repoCmd)
	serviceCmd.AddCommand(serviceListCmd)
	serviceCmd.AddCommand(serviceImagesCmd)
	serviceCmd.AddCommand(serviceReleaseCmd)
	repoCmd.AddCommand(repoImagesCmd)

	if cmd, err := rootCmd.ExecuteC(); err != nil {
		switch err.(type) {
		case *usageError:
			cmd.Println("")
			cmd.Println(cmd.UsageString())
		}
		os.Exit(1)
	}
}
