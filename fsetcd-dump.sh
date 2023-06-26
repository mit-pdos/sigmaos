#!/bin/bash

go clean -testcache && go test -v sigmaos/fsetcd -run Dump
