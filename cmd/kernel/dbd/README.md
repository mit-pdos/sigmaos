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
  mysql> create database books;
  mysql> use books;
  mysql> source init-db.sql
  mysql> select * from book;

+----+------------------------------+------------+-------+
| id | title                        | author     | price |
+----+------------------------------+------------+-------+
|  1 | Computer systems engineering | J. Saltzer | 56.99 |
|  2 | Xv6                          | R. Morris  | 63.99 |
|  3 | Odyssey                      | Homer      | 34.98 |
+----+------------------------------+------------+-------+

* Create user sigma with pw sigmaos

  CREATE USER 'sigma'@'localhost' IDENTIFIED BY 'sigmaos';
  GRANT ALL PRIVILEGES ON books.* TO 'sigma'@'%';
  FLUSH PRIVILEGES;

* Run db
  $ go run .
