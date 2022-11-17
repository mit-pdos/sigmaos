CREATE TABLE user (
  username   VARCHAR(128) NOT NULL,
  password   VARCHAR(255) NOT NULL,
  PRIMARY KEY (`username`)
);

CREATE TABLE reservation (
  hotelid    VARCHAR(128) NOT NULL,
  customer   VARCHAR(128) NOT NULL,
  indate     VARCHAR(128) NOT NULL,
  outdate    VARCHAR(128) NOT NULL,
  number     INT NOT NULL,
  PRIMARY KEY (`hotelid`)
);

CREATE TABLE number (
  hotelid    VARCHAR(128) NOT NULL,
  number     INT NOT NULL,
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

