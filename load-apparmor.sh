#!/bin/bash

# install sigmaos-uproc apparmor profile

cp container/sigmos-uproc /etc/apparmor.d/sigmaos-uproc
apparmor_parser -r /etc/apparmor.d/sigmaos-uproc 
