# Temporary COPR SRPM spec until GoReleaser/nFPM stops emitting SOURCERPM in SRPMs.
Name:           composia
Version:        %{composia_version}
Release:        1%{?dist}
Summary:        Self-hosted Docker Compose control plane and CLI.

License:        AGPL-3.0-only
URL:            https://docs.composia.xyz
Source0:        %{name}-%{version}.tar.gz

# go.mod declares go 1.25.0. Builds on Fedora 42 (Go 1.24) will fail;
# exclude those chroots in COPR (--exclude-chroot fedora-42-*).
BuildRequires:  golang >= 1.25

%global debug_package %{nil}

%description
Composia is a self-hosted control plane for Docker Compose. It keeps service
definitions as plain files while coordinating deployment and visibility across
one or many nodes.

%prep
%autosetup -n %{name}-%{version}

%build
export CGO_ENABLED=0
export GOTOOLCHAIN=local
export GOFLAGS="-buildvcs=false -trimpath"

go build \
  -ldflags "-s -w -X forgejo.alexma.top/alexma233/composia/internal/version.Value=%{version}" \
  -o "_build/composia" \
  "forgejo.alexma.top/alexma233/composia/cmd/composia"

go build \
  -ldflags "-s -w -X forgejo.alexma.top/alexma233/composia/internal/version.Value=%{version}" \
  -o "_build/composia-controller" \
  "forgejo.alexma.top/alexma233/composia/cmd/composia-controller"

go build \
  -ldflags "-s -w -X forgejo.alexma.top/alexma233/composia/internal/version.Value=%{version}" \
  -o "_build/composia-agent" \
  "forgejo.alexma.top/alexma233/composia/cmd/composia-agent"

%install
install -m 0755 -vd "%{buildroot}%{_bindir}"
install -m 0755 -vp "_build/composia" "%{buildroot}%{_bindir}/composia"
install -m 0755 -vp "_build/composia-controller" "%{buildroot}%{_bindir}/composia-controller"
install -m 0755 -vp "_build/composia-agent" "%{buildroot}%{_bindir}/composia-agent"

%files
%license LICENSE
%doc README.md
%{_bindir}/composia
%{_bindir}/composia-controller
%{_bindir}/composia-agent

%changelog
* Fri May 01 2026 AlexMa <i@fur.im> - %{version}-1
- Package Composia for COPR.
