CREATE TABLE user (
  username   VARCHAR(128) NOT NULL,
  password   VARCHAR(255) NOT NULL,
  PRIMARY KEY (`username`)
);

CREATE TABLE reservation (
  id         INT AUTO_INCREMENT NOT NULL,
  hotelid    VARCHAR(128) NOT NULL,
  customer   VARCHAR(128) NOT NULL,
  indate     VARCHAR(128) NOT NULL,
  outdate    VARCHAR(128) NOT NULL,
  number     INT NOT NULL,
  PRIMARY KEY (`id`)
);

CREATE TABLE number (
  hotelid    VARCHAR(128) NOT NULL,
  number     INT NOT NULL,
  PRIMARY KEY (`hotelid`)
);

CREATE TABLE rate (
  hotelid    VARCHAR(128) NOT NULL,
  code       VARCHAR(128) NOT NULL,
  indate     VARCHAR(128) NOT NULL,
  outdate    VARCHAR(128) NOT NULL,
  roombookrate        FLOAT NOT NULL,
  roomtotalrate       FLOAT NOT NULL,
  roomtotalinclusive  FLOAT NOT NULL,
  roomcode            VARCHAR(128) NOT NULL,
  roomcurrency        VARCHAR(128) NOT NULL,
  roomdescription     VARCHAR(512) NOT NULL,
  PRIMARY KEY (`hotelid`)
);

CREATE TABLE profile (
  hotelid    VARCHAR(128) NOT NULL,
  name       VARCHAR(128) NOT NULL,
  phone      VARCHAR(128) NOT NULL,
  description  VARCHAR(512) NOT NULL,
  streetnumber VARCHAR(128) NOT NULL,
  streetname VARCHAR(128) NOT NULL,
  city       VARCHAR(128) NOT NULL,
  state      VARCHAR(128) NOT NULL,
  country    VARCHAR(128) NOT NULL,
  postal     VARCHAR(128) NOT NULL,
  lat        FLOAT NOT NULL,
  lon        FLOAT NOT NULL,
  PRIMARY KEY (`hotelid`)
);

