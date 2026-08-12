#!/bin/sh
# Local build for testing the production image. Releases are made by CI on push
# to main -- see .releaserc.json -- so nothing here pushes or names a version.
set -e
docker build --build-arg VERSION="${1:-dev}" --tag onlogs:local .
