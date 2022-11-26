# Installation

* Install mariadb

  https://wiki.archlinux.org/title/MariaDB#Installation
  # systemctl start mariadb
  
  use root and default pw

* Make mariadb accessible by remote hosts

  $ printf "[mysqld]\nbind-address = 0.0.0.0" | sudo tee /etc/mysql/my.cnf

* Configure mariadb security settings.

  $ sudo mysql_secure_installation

* Create db

  sudo mysql -u root -p
  mysql> create database sigmaos;
  mysql> use sigmaos;
  mysql> source init-db.sql

* Create user sigma with pw sigmaos

  CREATE USER 'sigma'@'localhost' IDENTIFIED BY 'sigmaos';
  GRANT ALL PRIVILEGES ON sigmaos.* TO 'sigma'@'localhost';
  FLUSH PRIVILEGES;

* Run db
  $ go run .
