#!/usr/bin/env bash
set -euo pipefail

action=${1:-inspect}

nginx_binary() {
  if command -v nginx >/dev/null 2>&1; then
    command -v nginx
  elif [[ -x /usr/sbin/nginx ]]; then
    printf '%s\n' /usr/sbin/nginx
  else
    echo 'Nginx is not installed' >&2
    return 1
  fi
}

upstream_version() {
  local raw=$1
  if [[ $raw =~ ([0-9]+)\.([0-9]+)\.([0-9]+) ]]; then
    printf '%s.%s.%s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[3]}"
  else
    echo "Unable to parse Nginx version: $raw" >&2
    return 1
  fi
}

is_patched_version() {
  local version major minor patch
  version=$(upstream_version "$1") || return 1
  IFS=. read -r major minor patch <<<"$version"
  (( major > 1 )) && return 0
  (( major == 1 && minor > 31 )) && return 0
  (( major == 1 && minor == 31 && patch >= 9 )) && return 0
  (( major == 1 && minor == 30 && patch >= 7 )) && return 0
  return 1
}

privileged() {
  if (( EUID == 0 )); then
    "$@"
  else
    sudo -n "$@"
  fi
}

verify_config_if_permitted() {
  local nginx=$1
  if (( EUID == 0 )); then
    "$nginx" -t
  elif sudo -n true 2>/dev/null; then
    sudo -n "$nginx" -t
  else
    echo 'nginx_config_test=skipped_sudo_required'
  fi
}

inspect() {
  local nginx current candidate='unavailable'
  nginx=$(nginx_binary)
  current=$($nginx -v 2>&1)

  printf 'host=%s\n' "$(hostname)"
  if [[ -r /etc/os-release ]]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    printf 'os=%s\n' "${PRETTY_NAME:-unknown}"
  fi
  printf 'user=%s\n' "$(id -un)"
  printf 'nginx_binary=%s\nnginx_version=%s\n' "$nginx" "$current"
  if is_patched_version "$current"; then
    echo 'nginx_security_status=patched'
  else
    echo 'nginx_security_status=vulnerable'
  fi

  $nginx -V 2>&1 | sed -n '1,2p'
  systemctl is-active nginx
  verify_config_if_permitted "$nginx"

  if command -v dpkg-query >/dev/null 2>&1; then
    echo 'installed_packages:'
    dpkg-query -W -f='${db:Status-Abbrev} ${binary:Package} ${Version}\n' 'nginx*' 2>/dev/null \
      | awk '$1 == "ii" { print $2, $3 }' \
      | sort
  fi
  if command -v apt-cache >/dev/null 2>&1; then
    candidate=$(apt-cache policy nginx | awk '/Candidate:/ { print $2; exit }')
    printf 'apt_candidate=%s\n' "${candidate:-unavailable}"
    if [[ -n $candidate ]] && is_patched_version "$candidate"; then
      echo 'apt_candidate_security_status=patched'
    else
      echo 'apt_candidate_security_status=vulnerable_or_unavailable'
    fi
    apt-cache policy nginx | sed -n '1,20p'
  fi
}

upgrade() {
  local nginx current candidate
  nginx=$(nginx_binary)
  current=$($nginx -v 2>&1)
  if is_patched_version "$current"; then
    echo "Nginx is already patched: $current"
    verify_config_if_permitted "$nginx"
    systemctl is-active nginx
    return
  fi

  command -v apt-get >/dev/null 2>&1 || {
    echo 'Automatic upgrade currently supports apt-based hosts only' >&2
    return 1
  }
  privileged apt-get update
  candidate=$(apt-cache policy nginx | awk '/Candidate:/ { print $2; exit }')
  if [[ -z $candidate ]] || ! is_patched_version "$candidate"; then
    echo "Refusing to install unpatched Nginx candidate: ${candidate:-unavailable}" >&2
    echo 'Configure an Nginx repository that provides stable 1.30.7+ or mainline 1.31.9+.' >&2
    return 1
  fi

  echo "Upgrading Nginx from $current to package candidate $candidate"
  privileged env DEBIAN_FRONTEND=noninteractive apt-get install --only-upgrade --yes nginx
  current=$($nginx -v 2>&1)
  is_patched_version "$current" || {
    echo "Installed Nginx is still vulnerable: $current" >&2
    return 1
  }
  privileged "$nginx" -t
  privileged systemctl reload nginx
  systemctl is-active nginx
  echo "Nginx security upgrade complete: $current"
}

case "$action" in
  inspect) inspect ;;
  upgrade) upgrade ;;
  check-version)
    is_patched_version "${2:?version required}"
    ;;
  *)
    echo "Usage: $0 {inspect|upgrade}" >&2
    exit 2
    ;;
esac
