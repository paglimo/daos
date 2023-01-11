#!/bin/bash

set -eux

distro="$1"
# shellcheck disable=SC2034
client_ver="$2"

if [[ $distro = ubuntu20* ]]; then
    #pkgs="openmpi-bin ndctl fio"
    # these should be package dependencies
    pkgs=""
elif [[ $distro = el* ]] || [[ $distro = centos* ]] ||
     [[ $distro = leap* ]]; then
    openmpi="openmpi"
    pyver="3"
    prefix=""

    if [[ $distro = el7* ]] || [[ $distro = centos7* ]]; then
        pyver="36"
        openmpi="openmpi3"
        prefix="--exclude ompi"
    elif [[ $distro = leap15* ]]; then
        openmpi="openmpi3"
    fi

    pkgs="$prefix ndctl                \
          fio patchutils               \
          romio-tests                  \
          testmpio                     \
          python$pyver-mpi4py-tests    \
          hdf5-mpich-tests             \
          hdf5-$openmpi-tests          \
          hdf5-vol-daos-$openmpi-tests \
          hdf5-vol-daos-mpich-tests    \
          simul-mpich                  \
          simul-$openmpi               \
          MACSio-mpich                 \
          MACSio-$openmpi"
else
    echo "I don't know which packages should be installed for distro" \
         "\"$distro\""
    exit 1
fi

# DO NOT LAND
# this belongs in the test image
if [[ $distro = el9 ]]; then
    pkgs="$pkgs s-nail"
fi

echo "$pkgs"

exit 0
