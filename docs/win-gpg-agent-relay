#!/bin/bash

# Script origins:
#   https://gist.github.com/andsens/2ebd7b46c9712ac205267136dc677ac1
#   https://gist.github.com/Nimamoh/e2df2ba0a99ef221d8cca360c931e5e6

GNUPGHOME="$HOME/.gnupg"

if [ ! -d ${GNUPGHOME} ]; then
    mkdir ${GNUPGHOME}
    chmod 0700 ${GNUPGHOME}
fi
PIDFILE="$GNUPGHOME/win-gpg-agent-relay.pid"

SORELAY_BIN="${HOME}/winhome/.wsl/sorelay.exe"
WINAGENT_HOME_DIR="${WIN_AGENT_HOME}"
WINGPGAGENT_SOCKETS_DIR="${WIN_GNUPG_SOCKETS}"

GPG_AGENT_SOCK="$WINGPGAGENT_SOCKETS_DIR/S.gpg-agent"
GPG_AGENT_EXTRA_SOCK="$WINGPGAGENT_SOCKETS_DIR/S.gpg-agent.extra"
GPG_AGENT_SSH_SOCK="$WINAGENT_HOME_DIR/S.gpg-agent.ssh"

log() {
    echo >&2 "$@"
}

is_pid_running() {
    if [[ -z "$1" ]]; then
        return 1
    fi
    ps -p "$1" >/dev/null
    return $?
}

_cleanup() {
    log "Cleaning up relay to $GPG_AGENT_SOCK..."
    if is_pid_running "$SOCAT_GPG_AGENT_PID"; then
        kill -SIGTERM "$SOCAT_GPG_AGENT_PID" || log "Failed."
    fi
    log "Cleaning up relay to $GPG_AGENT_EXTRA_SOCK..."
    if is_pid_running "$SOCAT_GPG_AGENT_EXTRA_PID"; then
        kill -SIGTERM "$SOCAT_GPG_AGENT_EXTRA_PID" || log "Failed."
    fi
    log "Cleaning up relay to $GPG_AGENT_SSH_SOCK..."
    if is_pid_running "$SOCAT_GPG_AGENT_SSH_PID"; then
        kill -SIGTERM "$SOCAT_GPG_AGENT_SSH_PID" || log "Failed."
    fi
}

die() {
    if [[ -n "$1" ]]; then
        log "$1"
    fi
    log "Exiting."
    exit 1
}

usage() {
    log "Usage: win-gpg-agent-relay [OPTIONS] COMMAND"
    log ""
    log "  SUMMARY: Relay local GPG sockets to win-gpg-agent's ones in order to integrate WSL2 and host."
    log "           Do debug use foreground command"
    log ""
    log "  OPTIONS:"
    log "    -h|--help     this page"
    log ""
    log "    -v|--verbose  verbose mode"
    log ""
    log "  COMMAND: start, stop, foreground"
}

fg_opts() {
    FG_OPTS=()
    # Generate opts for passing it to foreground version
    if [[ -n "$VERBOSE" ]]; then
        FG_OPTS+=("-v")
    fi
}

main() {

    POSITIONAL=()
    VERBOSE=""
    while (($# > 0)); do
        case "$1" in
        -v | --verbose)
            VERBOSE="ENABLED"
            shift # shift once since flags have no values
            ;;

        -h | --help)
            usage
            exit 0
            ;;

        *) # unknown flag/switch
            POSITIONAL+=("$1")
            shift
            if [[ "${#POSITIONAL[@]}" -gt 1 ]]; then
                usage
                die
            fi
            ;;
        esac
    done

    set -- "${POSITIONAL[@]}" # restore positional params

    if [[ -z "$VERBOSE" ]]; then
        QUIET="QUIET"
    fi

    case "${POSITIONAL[0]}" in
    start)
        fg_opts
        start-stop-daemon --start --oknodo --pidfile "$PIDFILE" --name win-gpg-agent-r --make-pidfile --background --startas "$0" ${VERBOSE:+--verbose} ${QUIET:+--quiet} -- foreground "${FG_OPTS[@]}"
        ;;

    stop)
        start-stop-daemon --pidfile "$PIDFILE" --stop --remove-pidfile ${VERBOSE:+--verbose} ${QUIET:+--quiet}
        ;;

    status)
        start-stop-daemon --pidfile "$PIDFILE" --status ${VERBOSE:+--verbose} ${QUIET:+--quiet}
        local result=$?
        case $result in
        0) log "$0 is running" ;;
        1 | 3) log "$0 is not running" ;;
        4) log "$0 unable to determine status" ;;
        esac
        return $result
        ;;

    foreground)
        relay
        ;;

    *)
        usage
        die
        ;;
    esac
}

# Serve socket at path $1 relaying to windows AF_LINUX socket $2 through sorelay tool with args in $3
# return 0 on successfully setup socat, outputs the PID
sorelay() {
    log "Set up $1 as socket relaying to $2"

    local pid
    local exec
    if [ -z "$3" ]; then
        exec="\'$SORELAY_BIN\' \'$2\'"
    else 
        exec="\'$SORELAY_BIN\' $3 \'$2\'"
    fi
    socat UNIX-LISTEN:"\"$1\"",fork EXEC:"\"$exec\"",nofork 1>/dev/null 2>&1 &
    pid="$!"

    # quickly check if socat still running to catch early errors
    if ! is_pid_running "$pid"; then
        log "socat $1 failed"
        return 1
    fi

    echo "$pid"
}

relay() {

    trap _cleanup EXIT

    log "Using gpg-agent sockets in: $WINGPGAGENT_SOCKETS_DIR"
    log "Using agent-gui sockets in: $WINAGENT_HOME_DIR"

    [[ -f "${SORELAY_BIN}" ]] || die "Unable to access ${SORELAY_BIN}"
    [[ -z "$WINGPGAGENT_SOCKETS_DIR" ]] && die "Wrong directory of gpg-agent sockets"
    [[ -z "$WINAGENT_HOME_DIR" ]] && die "Wrong directory of agent-gui home"

    if pgrep -fx "^gpg-agent\s.+" >/dev/null; then
        log "Killing previously started local gpg-agent..."
        echo "KILLAGENT" | gpg-connect-agent >/dev/null 2>&1
    fi

    if [ -f "$PIDFILE" ]; then
        if ! pgrep "^win-gpg-agent-r$"; then
            # wsl has been shutdown ungracefully, leaving garbage behind
            rm "$PIDFILE" "$HOME/.gnupg/S.gpg-agent" "$HOME/.gnupg/S.gpg-agent.extra" "$HOME/.gnupg/S.gpg-agent.ssh"
        fi
    fi

    if ! SOCAT_GPG_AGENT_PID=$(sorelay "$HOME/.gnupg/S.gpg-agent" "$GPG_AGENT_SOCK" "-a"); then die; fi
    log "socat running with PID: $SOCAT_GPG_AGENT_PID"
    if ! SOCAT_GPG_AGENT_EXTRA_PID=$(sorelay "$HOME/.gnupg/S.gpg-agent.extra" "$GPG_AGENT_EXTRA_SOCK" "-a"); then die; fi
    log "socat running with PID: $SOCAT_GPG_AGENT_EXTRA_PID"
    if ! SOCAT_GPG_AGENT_SSH_PID=$(sorelay "$HOME/.gnupg/S.gpg-agent.ssh" "$GPG_AGENT_SSH_SOCK"); then die; fi
    log "socat running with PID: $SOCAT_GPG_AGENT_SSH_PID"

    log -n "Polling remote gpg-agent... "
    gpg-connect-agent /bye >/dev/null 2>&1 || die "[$?] Failure communicating with gpg-agent"
    log "OK"
    log -n "Polling remote ssh-agent..."
    SSH_AUTH_SOCK="$HOME/.gnupg/S.gpg-agent.ssh" ssh-add -L >/dev/null 2>&1 || die "[$?] Failure communicating with ssh-agent"
    log "OK"

    # Everything checks, we are ready for actions
    log "Entering wait..."
    while [ -e /proc/$SOCAT_GPG_AGENT_PID ] || [ -e /proc/$SOCAT_GPG_AGENT_EXTRA_PID ] || [ -e /proc/$SOCAT_GPG_AGENT_SSH_PID ]; do
        local time_s
        time_s=$(mult_by_two_and_max "${time_s:-1}" 600)
        if [[ -n "$VERBOSE" ]]; then
            echo "Wait $time_s seconds"
        fi
    sleep "$time_s"
    done
}

# outputs max(x * 2, y) where x is first argument, y the second one
mult_by_two_and_max() {
    local max
    max=${2:-600}
    last=${1:-1}

    curr=$((last * 2))
    curr=$((curr > max ? max : curr))
    echo "$curr"
}

main "$@"
