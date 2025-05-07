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

PORT=3306  # use non-default port number on host
MONGO_PORT=4407

ROOT=$(dirname $(realpath $0))
source $ROOT/env/env.sh

DB_IMAGE_NAME="sigmadb"
MONGO_IMAGE_NAME="sigmamongo"
TESTER_NETWORK="host"
if ! [ -z "$SIGMAUSER" ]; then
  DB_IMAGE_NAME=$DB_IMAGE_NAME-$SIGMAUSER
  MONGO_IMAGE_NAME=$MONGO_IMAGE_NAME-$SIGMAUSER
  TESTER_NETWORK="sigmanet-testuser-${SIGMAUSER}"
fi

docker pull mariadb:10.4
if ! docker ps | grep -q $DB_IMAGE_NAME; then
    echo "start db $TESTER_NETWORK $PORT $DB_IMAGE_NAME"
    docker run \
    --network $TESTER_NETWORK \
    --name $DB_IMAGE_NAME \
    -e MYSQL_ROOT_PASSWORD=sigmadb \
    -p $PORT:3306 \
    -d mariadb
fi

until [ "`docker inspect -f {{.State.Running}} $DB_IMAGE_NAME`"=="true" ]; do
    echo -n "." 1>&2
    sleep 0.1;
done;

if [[ "$TESTER_NETWORK" == "host" ]]; then
    ip=127.0.0.1
else
    # ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $DB_IMAGE_NAME)
    ip=127.0.0.1
fi

echo "db net: $TESTER_NETWORK IP: $ip"

until mariadb-show --skip-ssl -h $ip -P $PORT -u root -psigmadb 2> /dev/null; do
    echo -n "." 1>&2
    sleep 0.1;
done;    

if ! mariadb-show --skip-ssl -h $ip -u root -psigmadb | grep -q sigmaos; then
    echo "initialize db"
    mariadb --skip-ssl -h $ip -u root -psigmadb <<ENDOFSQL
CREATE database sigmaos;
USE sigmaos;
source apps/hotel/init-db.sql;
CREATE USER 'sigma1'@'172.17.%.%' IDENTIFIED BY 'sigmaos1';
GRANT ALL PRIVILEGES ON sigmaos.* TO 'sigma1'@'172.17.%.%';
CREATE USER 'sigma1'@'192.168.%.%' IDENTIFIED BY 'sigmaos1';
GRANT ALL PRIVILEGES ON sigmaos.* TO 'sigma1'@'192.168.%.%';
CREATE USER 'sigma1'@'10.10.%.%' IDENTIFIED BY 'sigmaos1';
GRANT ALL PRIVILEGES ON sigmaos.* TO 'sigma1'@'10.10.%.%';
CREATE USER 'sigma1'@'10.0.%.%' IDENTIFIED BY 'sigmaos1';
GRANT ALL PRIVILEGES ON sigmaos.* TO 'sigma1'@'10.0.%.%';
CREATE USER 'sigma1'@'127.0.%.%' IDENTIFIED BY 'sigmaos1';
GRANT ALL PRIVILEGES ON sigmaos.* TO 'sigma1'@'127.0.%.%';
FLUSH PRIVILEGES;
SET GLOBAL max_connections = 100000;
ENDOFSQL
fi

if [[ "$TESTER_NETWORK" != "host" ]]; then
  docker network connect $TESTER_NETWORK $DB_IMAGE_NAME
  docker network disconnect bridge $DB_IMAGE_NAME
fi

echo "db IP post reconnect: $ip"

docker pull mongo:4.4.6
if ! docker ps | grep -q $MONGO_IMAGE_NAME; then
    echo "start mongodb"
    docker run \
      --name $MONGO_IMAGE_NAME \
      --network $TESTER_NETWORK \
      -p $MONGO_PORT:27017 \
      -d mongo:4.4.6
fi

until [ "`docker inspect -f {{.State.Running}} $MONGO_IMAGE_NAME`"=="true" ]; do
    echo -n "." 1>&2
    sleep 0.1;
done;

ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $MONGO_IMAGE_NAME)

echo "mongo IP: $ip"
