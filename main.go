package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"dagger.io/dagger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"golang.org/x/sync/errgroup"
)

func newTraceProvider(exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	// Ensure default SDK resources and the required service name are set.
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("DaggerService"),
		),
	)

	if err != nil {
		panic(err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(r),
	)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("must pass in a git repo to build")
		os.Exit(1)
	}
	ctx := context.Background()
	client := otlptracehttp.NewClient()
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		fmt.Errorf("creating OTLP trace exporter: %w", err)
	}
	tp := newTraceProvider(exporter)

	// Handle shutdown properly so nothing leaks.
	defer func() { _ = tp.Shutdown(ctx) }()

	otel.SetTracerProvider(tp)

	repo := os.Args[1]
	if err := build(repo); err != nil {
		fmt.Println(err)
	}
}

const name = "multibuild"

func build(repoUrl string) error {
	_, span := otel.Tracer(name).Start(context.Background(), "Run")
	defer span.End()

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
	src := repo.Branch("main").Tree()

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
					build = build.WithExec([]string{"go", "build", "-o", path})

					output := build.Directory(path)

					_, err = output.Export(ctx, path)
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
