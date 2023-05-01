CREATE TABLE socialnetwork_user (
  userid     BIGINT NOT NULL,
  firstname  VARCHAR(128) NOT NULL,
  lastname   VARCHAR(128) NOT NULL,
  username   VARCHAR(128) NOT NULL,
  password   VARCHAR(255) NOT NULL,
  PRIMARY KEY (`userid`)
);

CREATE TABLE socialnetwork_graph (
  followerid BIGINT NOT NULL,
  followeeid BIGINT NOT NULL,
  PRIMARY KEY (`followerid`, `followeeid`)
);

CREATE TABLE socialnetwork_media (
  medianame    VARCHAR(128) NOT NULL,
  mediacontent LONGBLOB,
  PRIMARY KEY (`medianame`)
);

CREATE TABLE socialnetwork_post (
  postid      BIGINT NOT NULL,
  postcontent MEDIUMBLOB,
  PRIMARY KEY (`postid`)
);
