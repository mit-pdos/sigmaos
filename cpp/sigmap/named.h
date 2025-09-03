#pragma once

namespace sigmaos {
namespace sigmap::named {

//
const std::string ANY = "~any";
const std::string LOCAL = "~local";

const std::string KNAMED = "knamed";
const std::string NAME = "name";
const std::string ROOT = "root";
const std::string NAMED = NAME + "/";
const std::string NAMEDREL = "named";
const std::string MEMFSREL = "memfs";
const std::string MEMFS = NAMED + MEMFSREL + "/";
const std::string REALMREL = "realm";
const std::string REALM = NAMED + REALMREL + "/";
const std::string REALMDREL = "realmd";
const std::string REALMD = NAMED + REALMREL + "/" + REALMDREL;
const std::string REALMSREL = "realms";
const std::string REALMS = NAMED + "/" + REALMSREL;
const std::string BOOTREL = "boot";
const std::string BOOT = NAMED + BOOTREL + "/";
const std::string PROCDREL = "procd";
const std::string S3REL = "s3";
const std::string S3 = NAMED + S3REL + "/";
const std::string UXREL = "ux";
const std::string UX = NAMED + UXREL + "/";
const std::string CHUNKDREL = "chunkd";
const std::string CHUNKD = NAMED + CHUNKDREL + "/";
const std::string MSCHEDREL = "msched";
const std::string MSCHED = NAMED + MSCHEDREL + "/";
const std::string LCSCHEDREL = "lcsched";
const std::string LCSCHED = NAMED + LCSCHEDREL + "/";
const std::string SPPROXYDREL = "spproxyd";
const std::string BESCHEDREL = "besched";
const std::string BESCHED = NAMED + BESCHEDREL + "/";
const std::string DBREL = "db";
const std::string DB = NAMED + DBREL + "/";
const std::string DBD = DB + ANY + "/";
const std::string MONGOREL = "mongo";
const std::string MONGO = NAMED + MONGOREL + "/";

const std::string IMGREL = "img";
const std::string IMG = NAMED + IMGREL + "/";

const std::string KPIDSREL = "kpids";
const std::string KPIDS = NAMED + KPIDSREL;

// MSched
const std::string PIDS = "pids";

// special devs/dirs exported by SigmaSrv/SessSrv
const std::string STATSD = ".statsd";
const std::string FENCEDIR = ".fences";

// stats exported by named
const std::string PSTATSD = ".pstatsd";

// names for directly-mounted services
const std::string S3CLNT = "s3clnt";

};  // namespace sigmap::named
};  // namespace sigmaos
