#!/bin/bash

function a {
    echo 'A'
}
export -f a

timeout 9s bash -c 'a'
a
timeout 9s bash -c 'echo A'
cmd="echo A"
timeout 9s bash -c "${cmd}"
