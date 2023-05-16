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
  mediaid      BIGINT NOT NULL,
  mediatype    VARCHAR(128) NOT NULL,
  mediacontent LONGBLOB,
  PRIMARY KEY (`mediaid`)
);

CREATE TABLE socialnetwork_post (
  postid      BIGINT NOT NULL,
  postcontent MEDIUMBLOB,
  PRIMARY KEY (`postid`)
);

CREATE TABLE socialnetwork_timeline (
  userid    BIGINT NOT NULL,
  postid    BIGINT NOT NULL,
  timestamp BIGINT NOT NULL,
  PRIMARY KEY (`userid`, `postid`)
);

CREATE TABLE socialnetwork_url (
  shorturl    VARCHAR(128) NOT NULL,
  extendedurl VARCHAR(2048) NOT NULL,
  PRIMARY KEY (`shorturl`)
);
