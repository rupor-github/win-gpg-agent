#!/bin/bash

_dist=bin
[ -d ${_dist} ] && rm -rf ${_dist}
(
    [ -d release ] && rm -rf release
    mkdir release
    cd release

    cmake -DCMAKE_BUILD_TYPE=Release  ..
    make install
)

cd ${_dist}
7z a -r ../win-gpg-agent

