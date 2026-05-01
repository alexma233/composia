#!/usr/bin/env bash
set -eu

dist_dir="${DIST_DIR:-dist}"
topdir="${RPMBUILD_TOPDIR:-${dist_dir}/copr-rpmbuild}"
metadata_file="${dist_dir}/metadata.json"
spec_file="packaging/fedora/composia.spec"

if [ ! -f "${metadata_file}" ]; then
  printf 'missing %s; run goreleaser release --snapshot --clean --skip=docker first\n' "${metadata_file}" >&2
  exit 1
fi

version="$(grep -o '"version":"[^"]*"' "${metadata_file}" | cut -d '"' -f 4)"
if [ -z "${version}" ]; then
  printf 'could not read release version from %s\n' "${metadata_file}" >&2
  exit 1
fi

source_archive="${dist_dir}/composia-${version}.tar.gz"
if [ ! -f "${source_archive}" ]; then
  printf 'missing %s; run goreleaser release --snapshot --clean --skip=docker first\n' "${source_archive}" >&2
  exit 1
fi

if [ ! -f "${spec_file}" ]; then
  printf 'missing %s\n' "${spec_file}" >&2
  exit 1
fi

rm -rf "${topdir}"
mkdir -p \
  "${topdir}/BUILD" \
  "${topdir}/BUILDROOT" \
  "${topdir}/RPMS" \
  "${topdir}/SOURCES" \
  "${topdir}/SPECS" \
  "${topdir}/SRPMS"

cp "${source_archive}" "${topdir}/SOURCES/"
cp "${spec_file}" "${topdir}/SPECS/"

# Temporary workaround for GoReleaser/nFPM SRPMs containing RPMTAG_SOURCERPM.
rpmbuild -bs \
  --target noarch \
  --define "_topdir $(realpath "${topdir}")" \
  --define "composia_version ${version}" \
  "${topdir}/SPECS/composia.spec"

set -- "${topdir}"/SRPMS/*.src.rpm
if [ "$(rpm -qp --qf '%{SOURCERPM}' "$1")" != "(none)" ]; then
  printf 'generated SRPM still has RPMTAG_SOURCERPM: %s\n' "$1" >&2
  exit 1
fi

printf '%s\n' "$1"
