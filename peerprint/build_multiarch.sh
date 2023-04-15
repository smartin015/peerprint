#!/usr/bin/env bash

package=$1
if [[ -z "$package" ]]; then
  echo "usage: $0 <server|tester|package-name>"
  exit 1
fi

# See https://go.dev/doc/install/source#environment for options.
# 
# Valid but failed probably due to configuration issues: 
#   illumos amd64
#   ios arm64
#   linux loong64
# 
# Not building due to lack of demand
#   linux mips
#   linux mipsle
#   linux mips64
#   linux mips64le
#   linux ppc64
#   linux ppc64le
#   linux riscv64
#   linux s390x
#   netbsd 386
#   netbsd  amd64
#   netbsd  arm
#   openbsd 386
#   openbsd amd64
#   openbsd arm
#   openbsd arm64
#   freebsd 386
#   freebsd amd64
#   freebsd arm
#   aix ppc64
#   android 386
#   android amd64
#   android arm
#   android arm64
#   darwin  amd64
#   darwin  arm64
#   dragonfly amd64
#   plan9 386
#   plan9 amd64
#   plan9 arm
#   solaris amd64
#
# Left off due to lack of size in pypi
#   linux arm
#   linux 386
#   windows arm
#   windows 386

platforms="linux amd64
linux arm64
windows amd64
windows arm64"

while IFS= read -r platform; do
	platform_split=($platform)
	GOOS=${platform_split[0]}
	GOARCH=${platform_split[1]}
	output_name=$package'-'$GOOS'-'$GOARCH
	if [ $GOOS = "windows" ]; then
		output_name+='.exe'
	fi	
  echo "Building with GOOS=$GOOS, GOARCH=$GOARCH --> $output_name"

	env GOOS=$GOOS GOARCH=$GOARCH go build -o ./bin/$output_name ./cmd/$package
	if [ $? -ne 0 ]; then
   		echo 'An error has occurred! Aborting the script execution...'
		exit 1
	fi
done <<< "$platforms"
