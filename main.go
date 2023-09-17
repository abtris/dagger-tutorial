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
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

const (
	instrumentationName    = "github.com/abtris/dagger-tutorial"
	instrumentationVersion = "0.1.0"
)

var (
	tracer = otel.GetTracerProvider().Tracer(
		instrumentationName,
		trace.WithInstrumentationVersion(instrumentationVersion),
		trace.WithSchemaURL(semconv.SchemaURL),
	)
	sc trace.SpanContext
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("must pass in a git repo to build")
		os.Exit(1)
	}
	ctx := context.Background()
	opts := otlptracehttp.WithInsecure()
	client := otlptracehttp.NewClient(opts)
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		fmt.Errorf("creating OTLP trace exporter: %w", err)
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.Default()),
	)
	otel.SetTracerProvider(tracerProvider)

	// Handle shutdown properly so nothing leaks.
	defer func() { _ = tracerProvider.Shutdown(ctx) }()

	repo := os.Args[1]
	if err := build(ctx, repo); err != nil {
		fmt.Println(err)
	}
}

func build(ctx context.Context, repoUrl string) error {
	ctx, span := tracer.Start(ctx, "build")
	defer span.End()
	fmt.Printf("Building %s\n", repoUrl)

	g, ctx := errgroup.WithContext(ctx)

	oses := []string{"linux", "darwin"}
	arches := []string{"amd64", "arm64"}
	goVersions := []string{"1.20", "1.21"}

	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	repo := client.Git(repoUrl)
	src := repo.Branch("main").Tree()

	for _, version := range goVersions {
		ctx, span := tracer.Start(ctx, fmt.Sprintf("go-%s", version))
		defer span.End()
		imageTag := fmt.Sprintf("golang:%s", version)
		golang := client.Container().From(imageTag)
		golang = golang.WithMountedDirectory("/src", src).WithWorkdir("/src")

		for _, goos := range oses {
			ctx, span := tracer.Start(ctx, fmt.Sprintf("os-%s", goos))
			defer span.End()
			for _, goarch := range arches {
				g.Go(func() error {
					ctx, span := tracer.Start(ctx, fmt.Sprintf("arch-%s", goarch))
					goos, goarch, version := goos, goarch, version
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
					span.End()
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
