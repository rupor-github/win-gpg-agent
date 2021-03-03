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
zip -9 ../win-gpg-agent.zip *
cd ..
echo ${BUILD_PSWD} | minisign -S -s ~/.minisign/build.key -c "win-gpg-agent release signature" -m win-gpg-agent.zip
