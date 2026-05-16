#!/bin/sh
set -eu

site_dir="${GOBIN_SITE_ROOT:-/site}"

mkdir -p "$site_dir"
cd "$site_dir"

if [ ! -f config.yaml ] && [ ! -f config.yml ] && [ ! -f _config.yml ] && [ ! -f _config.yaml ]; then
	if [ "${GOBIN_AUTO_INIT:-true}" = "true" ]; then
		if [ -z "$(find . -mindepth 1 -maxdepth 1 ! -name public -print -quit)" ]; then
			echo "No Gobin site found in $site_dir; initializing a new site."
			gobin init .
		else
			echo "No Gobin config file found in $site_dir, and the directory is not empty." >&2
			echo "Mount an initialized site root, or use an empty directory so Gobin can initialize it." >&2
			exit 1
		fi
	fi
fi

exec "$@"
