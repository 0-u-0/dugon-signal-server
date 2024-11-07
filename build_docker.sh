#!/bin/bash

version=$(git describe --tags)
commit=$(git rev-parse HEAD)
buildTime=$(date +'%Y-%m-%d %H:%M:%S')

docker build --build-arg version="$version" --build-arg commit="$commit" --build-arg buildTime="$buildTime" -t signal .
