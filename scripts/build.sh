#!/bin/bash
set -e

git_tag=$(git describe --tags --abbrev=0)
git_commit=$(git rev-list -1 HEAD)
os=linux
arch=amd64
project=github.com/emsixteeen/nvidia-runc-wrapper
sources=cmd/nvidia-runc-wrapper
binary=nvidia-runc-wrapper

GOOS=${os} GOARCH=${arch} \
  go build \
  -ldflags "-X main.AppGitCommit=${git_commit} -X main.AppVersion=${git_tag}" \
  -o ${binary}-${os}-${arch}-${git_tag} \
  ${project}/${sources}
