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

	ctx := context.Background()

	client, err := dagger.Connect(ctx)
	if err != nil {
		return err
	}
	defer client.Close()
	return nil
}
