#!/bin/bash

go clean -testcache && go test -v sigmaos/named -run Dump
