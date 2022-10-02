#!/bin/bash
set -e
passage show build | minisign -S -s ${HOME}/.minisign/build.key -c "win-gpg-agent release signature" -m win-gpg-agent.zip
sed -i "s/__CURRENT_HASH__/$(sha256sum -z win-gpg-agent.zip | awk '{ print $1; }')/g" win-gpg-agent.json
