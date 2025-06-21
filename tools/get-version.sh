#!/bin/bash
# get-version.sh <directory>
# Generates version in format YYYY.MM.MICRO where MICRO is commits in current month for specified directory

DIR=${1:-.}
YEAR=$(date +%Y)
MONTH=$(date +%m)

# Get commits in current month for this directory
SINCE="${YEAR}-${MONTH}-01"
MICRO=$(git rev-list --count --since="${SINCE}" HEAD -- "${DIR}/")

VERSION="${YEAR}.${MONTH}.${MICRO}"
echo $VERSION
