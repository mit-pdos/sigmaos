#!/bin/bash

#
# script to set up container with mariadb in and initialize db for hotel
#

#
# to stop:
# $ docker stop sigmadb
#
# to remove
# $ docker rm -v sigmadb
#

usage() {
  echo "Usage: $0 yml..."
}

PORT=4406  # use non-default port number on host

docker pull mariadb:10.4
if ! docker ps | grep -q sigmadb; then
    echo "start db"
    docker run --name sigmadb -e MYSQL_ROOT_PASSWORD=sigmadb -p $PORT:3306 -d mariadb
fi

until [ "`docker inspect -f {{.State.Running}} sigmadb`"=="true" ]; do
    echo -n "." 1>&2
    sleep 0.1;
done;

ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' sigmadb)

echo "db IP: $ip"

until mysqlshow -h $ip -u root -psigmadb 2> /dev/null; do
    echo -n "." 1>&2
    sleep 0.1;
done;    

if ! mysqlshow -h $ip -u root -psigmadb | grep -q sigmaos; then
    echo "initialize db"
    mysql -h $ip -u root -psigmadb <<ENDOFSQL
CREATE database sigmaos;
USE sigmaos;
source hotel/init-db.sql;
source socialnetwork/init-db.sql;
CREATE USER 'sigma1'@'172.17.%.%' IDENTIFIED BY 'sigmaos1';
GRANT ALL PRIVILEGES ON sigmaos.* TO 'sigma1'@'172.17.%.%';
CREATE USER 'sigma1'@'192.168.%.%' IDENTIFIED BY 'sigmaos1';
GRANT ALL PRIVILEGES ON sigmaos.* TO 'sigma1'@'192.168.%.%';
FLUSH PRIVILEGES;
SET GLOBAL max_connections = 100000;
ENDOFSQL
fi
