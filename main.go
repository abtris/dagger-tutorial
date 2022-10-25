package main

import (
	"context"
	"fmt"
	"os"

	"dagger.io/dagger"
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

	// 1. Get a context
	ctx := context.Background()
	// 2. Initialize dagger client
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()
	// 3. Clone the repo using Dagger
	repo := client.Git(repoUrl)
	src, err := repo.Branch("main").Tree().ID(ctx)
	if err != nil {
		return err
	}
	// 4. Load the golang image
	golang := client.Container().From("golang:latest")
	// 5. Mount the cloned repo to the golang image
	golang = golang.WithMountedDirectory("/src", src).WithWorkdir("/src")
	// 6. Do the go build
	_, err = golang.Exec(dagger.ContainerExecOpts{
		Args: []string{"go", "build", "-o", "build/"},
	}).ExitCode(ctx)
	if err != nil {
		return err
	}
	return nil
}
