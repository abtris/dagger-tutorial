package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"dagger.io/dagger"
	"golang.org/x/sync/errgroup"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("must pass in a git repo to build")
		os.Exit(1)
	}
	repo := os.Args[1]
	if err := build(repo); err != nil {
		fmt.Println(err)
	}
}

func build(repoUrl string) error {
	fmt.Printf("Building %s\n", repoUrl)

	ctx := context.Background()
	g, ctx := errgroup.WithContext(ctx)

	oses := []string{"linux", "darwin"}
	arches := []string{"amd64", "arm64"}
	goVersions := []string{"1.18", "1.19"}

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	repo := client.Git(repoUrl)
	src, err := repo.Branch("main").Tree().ID(ctx)
	if err != nil {
		return err
	}

	workdir := client.Host().Workdir()

	for _, version := range goVersions {
		imageTag := fmt.Sprintf("golang:%s", version)
		golang := client.Container().From(imageTag)
		golang = golang.WithMountedDirectory("/src", src).WithWorkdir("/src")

		for _, goos := range oses {
			for _, goarch := range arches {
				goos, goarch, version := goos, goarch, version
				g.Go(func() error {
					path := fmt.Sprintf("build/%s/%s/%s/", version, goos, goarch)
					outpath := filepath.Join(".", path)
					err = os.MkdirAll(outpath, os.ModePerm)
					if err != nil {
						return err
					}

					build := golang.WithEnvVariable("GOOS", goos)
					build = build.WithEnvVariable("GOARCH", goarch)
					build = build.Exec(dagger.ContainerExecOpts{
						Args: []string{"go", "build", "-o", path},
					})

					output, err := build.Directory(path).ID(ctx)
					if err != nil {
						return err
					}

					_, err = workdir.Write(ctx, output, dagger.HostDirectoryWriteOpts{Path: path})
					if err != nil {
						return err
					}
					return nil
				})
			}
		}
	}
	if err := g.Wait(); err != nil {
		return err
	}
	return nil
}
