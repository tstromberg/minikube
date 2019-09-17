#!/bin/bash

# Copyright 2016 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


# This script downloads the test files from the build bucket and makes some executable.

# The script expects the following env variables:
# OS_ARCH: The operating system and the architecture separated by a hyphen '-' (e.g. darwin-amd64, linux-amd64, windows-amd64)
# VM_DRIVER: the vm-driver to use for the test
# EXTRA_START_ARGS: additional flags to pass into minikube start
# EXTRA_ARGS: additional flags to pass into minikube
# JOB_NAME: the name of the logfile and check name to update on github
# PARALLEL_COUNT: number of tests to run in parallel


readonly TEST_ROOT="${HOME}/minikube-integration"
readonly TEST_HOME="${TEST_ROOT}/${OS_ARCH}-${VM_DRIVER}-${MINIKUBE_LOCATION}-$$-${COMMIT}"
echo ">> Starting at $(date)"
echo ""
echo "arch:      ${OS_ARCH}"
echo "build:     ${MINIKUBE_LOCATION}"
echo "driver:    ${VM_DRIVER}"
echo "job:       ${JOB_NAME}"
echo "test home: ${TEST_HOME}"
echo "sudo:      ${SUDO_PREFIX}"
echo "kernel:    $(uname -v)"
# Setting KUBECONFIG prevents the version ceck from erroring out due to permission issues
echo "kubectl:   $(env KUBECONFIG=${TEST_HOME} kubectl version --client --short=true)"
echo "docker:    $(docker version --format '{{ .Client.Version }}')"

case "${VM_DRIVER}" in
  kvm2)
    echo "virsh:     $(virsh --version)"
  ;;
  virtualbox)
    echo "vbox:      $(vboxmanage --version)"
  ;;
esac

echo ""
mkdir -p out/ testdata/

# Install gsutil if necessary.
if ! type -P gsutil >/dev/null; then
  if [[ ! -x "out/gsutil/gsutil" ]]; then
    echo "Installing gsutil to $(pwd)/out ..."
    curl -s https://storage.googleapis.com/pub/gsutil.tar.gz | tar -C out/ -zxf -
  fi
  PATH="$(pwd)/out/gsutil:$PATH"
fi

# Add the out/ directory to the PATH, for using new drivers.
PATH="$(pwd)/out/":$PATH
export PATH

echo ""
echo ">> Downloading test inputs from ${MINIKUBE_LOCATION} ..."
gsutil -qm cp \
  "gs://minikube-builds/${MINIKUBE_LOCATION}/minikube-${OS_ARCH}" \
  "gs://minikube-builds/${MINIKUBE_LOCATION}/docker-machine-driver"-* \
  "gs://minikube-builds/${MINIKUBE_LOCATION}/e2e-${OS_ARCH}" out

gsutil -qm cp -r "gs://minikube-builds/${MINIKUBE_LOCATION}/testdata"/* testdata/

gsutil -qm cp "gs://minikube-builds/${MINIKUBE_LOCATION}/gvisor-addon" testdata/


# Set the executable bit on the e2e binary and out binary
export MINIKUBE_BIN="out/minikube-${OS_ARCH}"
export E2E_BIN="out/e2e-${OS_ARCH}"
chmod +x "${MINIKUBE_BIN}" "${E2E_BIN}" out/docker-machine-driver-*

procs=$(pgrep "minikube-${OS_ARCH}|e2e-${OS_ARCH}" || true)
if [[ "${procs}" != "" ]]; then
  echo "Warning: found stale test processes to kill:"
  ps -f -p ${procs} || true
  kill ${procs} || true
  kill -9 ${procs} || true
fi

# Cleanup stale test outputs.
echo ""
echo ">> Cleaning up after previous test runs ..."

for stale_dir in ${TEST_ROOT}/*; do
  echo "* Cleaning stale test root: ${stale_dir}"

  for tunnel in $(find ${stale_dir} -name tunnels.json -type f); do
    env MINIKUBE_HOME="$(dirname ${tunnel})" ${MINIKUBE_BIN} tunnel --cleanup || true
  done

  for home in $(find ${stale_dir} -name .minikube -type d); do
   env MINIKUBE_HOME="$(dirname ${home})" ${MINIKUBE_BIN} delete || true
   sudo rm -Rf "${home}"
  done

  for kconfig in $(find ${stale_dir} -name kubeconfig -type f); do
    sudo rm -f "${kconfig}"
  done

  rmdir "${stale_dir}" || ls "${stale_dir}"
done

# sometimes tests left over zombie procs that won't exit
# for example:
# jenkins  20041  0.0  0.0      0     0 ?        Z    Aug19   0:00 [minikube-linux-] <defunct>
zombie_defuncts=$(ps -A -ostat,ppid | awk '/[zZ]/ && !a[$2]++ {print $2}')
if [[ "${zombie_defuncts}" != "" ]]; then
  echo "Found zombie defunct procs to kill..."
  ps -f -p ${zombie_defuncts} || true
  kill ${zombie_defuncts} || true
fi

if type -P virsh; then
  virsh -c qemu:///system list --all \
    | awk '{ print $2 }' \
    | xargs -I {} sh -c "virsh -c qemu:///system destroy {}; virsh -c qemu:///system undefine {}" \
    || true
  echo ">> virsh VM list after clean up (should be empty):"
  virsh -c qemu:///system list --all || true
fi

if type -P vboxmanage; then
  vboxmanage list vms || true
  vboxmanage list vms \
    | xargs -I {} sh -c "vboxmanage startvm {} --type emergencystop; vboxmanage unregistervm {} --delete" \
    || true

  echo ">> VirtualBox VM list after clean up (should be empty):"
  vboxmanage list vms || true
fi


if type -P hdiutil; then
  hdiutil info | grep -E "/dev/disk[1-9][^s]" || true
  hdiutil info \
      | grep -E "/dev/disk[1-9][^s]" \
      | awk '{print $1}' \
      | xargs -I {} sh -c "hdiutil detach {}" \
      || true
fi

# cleaning up stale hyperkits
if type -P hyperkit; then
  for pid in $(pgrep hyperkit); do
    echo "Killing stale hyperkit $pid"
    ps -f -p $pid || true
    kill $pid || true
    kill -9 $pid || true
  done
fi

if [[ "${VM_DRIVER}" == "hyperkit" ]]; then
  if [[ -e out/docker-machine-driver-hyperkit ]]; then
    sudo chown root:wheel out/docker-machine-driver-hyperkit || true
    sudo chmod u+s out/docker-machine-driver-hyperkit || true
  fi
fi

kprocs=$(pgrep kubectl || true)
if [[ "${kprocs}" != "" ]]; then
  echo "error: killing hung kubectl processes ..."
  ps -f -p ${kprocs} || true
  sudo -E kill ${kprocs} || true
fi

# clean up none drivers binding on 8443
  none_procs=$(sudo lsof -i :8443 | tail -n +2 | awk '{print $2}' || true)
  if [[ "${none_procs}" != "" ]]; then
    echo "Found stale api servers listening on 8443 processes to kill: "
    for p in $none_procs
    do
    echo "Kiling stale none driver:  $p"
    sudo -E ps -f -p $p || true
    sudo -E kill $p || true
    sudo -E kill -9 $p || true
    done
  fi

function cleanup_stale_routes() {
  local show="netstat -rn -f inet"
  local del="sudo route -n delete"

  if [[ "$(uname)" == "Linux" ]]; then
    show="ip route show"
    del="sudo ip route delete"
  fi

  local troutes=$($show | awk '{ print $1 }' | grep 10.96.0.0 || true)
  for route in ${troutes}; do
    echo "WARNING: deleting stale tunnel route: ${route}"
    $del "${route}" || true
  done
}

cleanup_stale_routes || true

mkdir -p "${TEST_HOME}"
export MINIKUBE_HOME="${TEST_HOME}/.minikube"
export KUBECONFIG="${TEST_HOME}/kubeconfig"

# Build the gvisor image. This will be copied into minikube and loaded by ctr.
# Used by TestContainerd for Gvisor Test.
# TODO: move this to integration test setup.
chmod +x ./testdata/gvisor-addon
# skipping gvisor mac because ofg https://github.com/kubernetes/minikube/issues/5137
if [ "$(uname)" != "Darwin" ]; then
  docker build -t gcr.io/k8s-minikube/gvisor-addon:latest -f testdata/gvisor-addon-Dockerfile ./testdata
fi

echo ""
echo ">> Starting ${E2E_BIN} at $(date)"
${SUDO_PREFIX}${E2E_BIN} \
  -minikube-start-args="--vm-driver=${VM_DRIVER} ${EXTRA_START_ARGS}" \
  -test.timeout=60m \
  -test.parallel=${PARALLEL_COUNT} \
  -binary="${MINIKUBE_BIN}" && result=$? || result=$?
echo ">> ${E2E_BIN} exited with ${result} at $(date)"
echo ""

if [[ $result -eq 0 ]]; then
  status="success"
  echo "minikube: SUCCESS"
else
  status="failure"
  echo "minikube: FAIL"
fi

echo ">> Cleaning up after ourselves ..."
${SUDO_PREFIX}${MINIKUBE_BIN} tunnel --cleanup || true
${SUDO_PREFIX}${MINIKUBE_BIN} delete >/dev/null 2>/dev/null || true
cleanup_stale_routes || true

${SUDO_PREFIX} rm -Rf "${MINIKUBE_HOME}" || true
${SUDO_PREFIX} rm -f "${KUBECONFIG}" || true
rmdir "${TEST_HOME}"
echo ">> ${TEST_HOME} completed at $(date)"

if [[ "${MINIKUBE_LOCATION}" != "master" ]]; then
  readonly target_url="https://storage.googleapis.com/minikube-builds/logs/${MINIKUBE_LOCATION}/${JOB_NAME}.txt"
  curl -s "https://api.github.com/repos/kubernetes/minikube/statuses/${COMMIT}?access_token=$access_token" \
  -H "Content-Type: application/json" \
  -X POST \
  -d "{\"state\": \"$status\", \"description\": \"Jenkins\", \"target_url\": \"$target_url\", \"context\": \"${JOB_NAME}\"}"
fi
exit $result
