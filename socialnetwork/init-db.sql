CREATE TABLE socialnetwork_user (
  userid     BIGINT NOT NULL,
  firstname  VARCHAR(128) NOT NULL,
  lastname   VARCHAR(128) NOT NULL,
  username   VARCHAR(128) NOT NULL,
  password   VARCHAR(255) NOT NULL,
  PRIMARY KEY (`userid`)
);

