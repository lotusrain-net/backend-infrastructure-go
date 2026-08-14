package deployments

import (
	"os"
	"strings"
	"testing"
)

func TestApplicationServicesBuildTheirSharedImage(t *testing.T) {
	compose, err := os.ReadFile("compose.yml")
	if err != nil {
		t.Fatalf("read compose.yml: %v", err)
	}

	sharedApplication := "x-app: &app\n  image: backend-infrastructure-go:${IMAGE_TAG:-local}\n  build:\n    context: ..\n    dockerfile: Dockerfile"
	if !strings.Contains(string(compose), sharedApplication) {
		t.Fatal("x-app must build the shared application image so migrate, seed-admin, worker, scheduler, and api can start from a clean host")
	}
}

func TestComposeAndBuildStagesUseAConfigurableContainerRegistry(t *testing.T) {
	compose, err := os.ReadFile("compose.yml")
	if err != nil {
		t.Fatalf("read compose.yml: %v", err)
	}

	for _, snippet := range []string{
		"image: ${IMAGE_REGISTRY:-public.ecr.aws/docker}/library/postgres:15-alpine",
		"image: ${IMAGE_REGISTRY:-public.ecr.aws/docker}/library/redis:7-alpine",
		"IMAGE_REGISTRY: ${IMAGE_REGISTRY:-public.ecr.aws/docker}",
	} {
		if !strings.Contains(string(compose), snippet) {
			t.Errorf("compose.yml must contain %q", snippet)
		}
	}

	environment, err := os.ReadFile("../.env.example")
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}
	if !strings.Contains(string(environment), "IMAGE_REGISTRY=public.ecr.aws/docker") {
		t.Error(".env.example must set the default container registry")
	}

	dockerfile, err := os.ReadFile("../Dockerfile")
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	if !strings.Contains(string(dockerfile), "FROM ${IMAGE_REGISTRY}/library/golang:1.26.6 AS build") {
		t.Error("Dockerfile must use IMAGE_REGISTRY for its Go build image")
	}

	webDockerfile, err := os.ReadFile("../web/Dockerfile")
	if err != nil {
		t.Fatalf("read web/Dockerfile: %v", err)
	}
	if strings.Count(string(webDockerfile), "FROM ${IMAGE_REGISTRY}/library/node:20-alpine") != 3 {
		t.Error("web/Dockerfile must use IMAGE_REGISTRY for every Node build stage")
	}
}

func TestGoBuildUsesConfigurableReachableModuleServices(t *testing.T) {
	compose, err := os.ReadFile("compose.yml")
	if err != nil {
		t.Fatalf("read compose.yml: %v", err)
	}
	for _, snippet := range []string{
		"GOPROXY: ${GOPROXY:-https://goproxy.cn,direct}",
		"GOSUMDB: ${GOSUMDB:-sum.golang.google.cn}",
	} {
		if !strings.Contains(string(compose), snippet) {
			t.Errorf("compose.yml must contain %q", snippet)
		}
	}

	environment, err := os.ReadFile("../.env.example")
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}
	for _, snippet := range []string{
		"GOPROXY=https://goproxy.cn,direct",
		"GOSUMDB=sum.golang.google.cn",
	} {
		if !strings.Contains(string(environment), snippet) {
			t.Errorf(".env.example must contain %q", snippet)
		}
	}

	dockerfile, err := os.ReadFile("../Dockerfile")
	if err != nil {
		t.Fatalf("read Dockerfile: %v", err)
	}
	for _, snippet := range []string{
		"ARG GOPROXY=https://goproxy.cn,direct",
		"ARG GOSUMDB=sum.golang.google.cn",
		"ENV GOPROXY=$GOPROXY",
		"ENV GOSUMDB=$GOSUMDB",
	} {
		if !strings.Contains(string(dockerfile), snippet) {
			t.Errorf("Dockerfile must contain %q", snippet)
		}
	}
}
