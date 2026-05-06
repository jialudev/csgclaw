#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
src="${repo_root}/runtimes"
dst="${repo_root}/internal/agent/embed/runtimes"

if [ ! -d "${src}/picoclaw/manager/workspace" ]; then
  echo "missing PicoClaw manager workspace: ${src}/picoclaw/manager/workspace" >&2
  exit 1
fi
if [ ! -d "${src}/picoclaw/worker/workspace" ]; then
  echo "missing PicoClaw worker workspace: ${src}/picoclaw/worker/workspace" >&2
  exit 1
fi
if [ ! -d "${src}/openclaw/manager/workspace" ]; then
  echo "missing OpenClaw manager workspace: ${src}/openclaw/manager/workspace" >&2
  exit 1
fi
if [ ! -d "${src}/openclaw/worker/workspace" ]; then
  echo "missing OpenClaw worker workspace: ${src}/openclaw/worker/workspace" >&2
  exit 1
fi
if [ ! -d "${src}/openclaw/csg-skills" ]; then
  echo "missing OpenClaw CSG skills pack: ${src}/openclaw/csg-skills" >&2
  exit 1
fi

# Materialize OpenClaw manager `workspace/skills` from the CSG pack (single runtimes/ source: openclaw/csg-skills).
rm -rf "${src}/openclaw/manager/workspace/skills"
mkdir -p "${src}/openclaw/manager/workspace"
cp -R "${src}/openclaw/csg-skills/." "${src}/openclaw/manager/workspace/skills/"

rm -rf "${dst}"
mkdir -p "$(dirname "${dst}")"
cp -R "${src}" "${dst}"
