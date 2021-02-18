#!/bin/sh

SUDO_OPTION=-E
if [ "$SUDO" = "" ]; then
    SUDO_OPTION=""
fi

PLACEMAT_MAJOR_VERSION="1"
PLACEMAT_PID="$(pgrep --exact placemat2)"
if [ "$PLACEMAT_PID" != "" ]; then
    PLACEMAT_MAJOR_VERSION="2"
fi
export PLACEMAT_MAJOR_VERSION

go mod download
if [ "$PLACEMAT_MAJOR_VERSION" = "1" ]; then
    EXTERNAL_PID=$(pmctl pod show external | jq .pid)
    export EXTERNAL_PID

    $SUDO $SUDO_OPTION nsenter -t $(pmctl pod show operation | jq .pid) -n env PATH=$PATH SUITE=$SUITE $GINKGO
else
    $SUDO $SUDO_OPTION ip netns exec operation env PATH=$PATH SUITE=$SUITE $GINKGO
fi
