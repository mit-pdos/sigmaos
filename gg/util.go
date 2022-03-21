package gg

import (
	"path"
	"strings"

	"ulambda/proc"
)

// PID constants
const (
	UPLOADER_SUFFIX       = ".uploader"
	DIR_UPLOADER_SUFFIX   = ".dir-uploader"
	DOWNLOADER_SUFFIX     = ".downloader"
	EXECUTOR_SUFFIX       = ".executor"
	TARGET_WRITER_SUFFIX  = ".target-writer"
	OUTPUT_HANDLER_SUFFIX = ".output-handler"
	REDUCTION_SUFFIX      = ".reduction"
	NO_OP_SUFFIX          = ".no-op-waiter"
)

// ========== Thunk naming ==========

func isThunk(hash string) bool {
	return hash[0] == 'T'
}

func isReduction(hash string) bool {
	return hash[0] == 'T' && strings.Contains(hash, "#")
}

func thunkHashFromReduction(reduction string) string {
	return reduction[:strings.Index(reduction, "#")]
}

func thunkHashesFromReductions(reductions []string) []string {
	hashes := []string{}
	for _, r := range reductions {
		if isReduction(r) {
			hashes = append(hashes, thunkHashFromReduction(r))
		} else {
			hashes = append(hashes, r)
		}
	}
	return hashes
}

// ========== Pids ==========

func noOpPid(pid proc.Tpid) proc.Tpid {
	return ggPid("", "", pid.String(), NO_OP_SUFFIX)
}

func executorPid(hash string) proc.Tpid {
	return ggPid("", "", hash, EXECUTOR_SUFFIX)
}

func outputHandlerPid(hash string) proc.Tpid {
	return ggPid("", "", hash, OUTPUT_HANDLER_SUFFIX)
}

func reductionWriterPid(dir string, subDir string, hash string) proc.Tpid {
	return ggPid(path.Base(dir), path.Base(subDir), hash, TARGET_WRITER_SUFFIX)
}

func reductionDownloaderPid(reductionHash string, subDir string, target string) proc.Tpid {
	return ggPid(reductionHash, subDir, target, REDUCTION_SUFFIX+DOWNLOADER_SUFFIX)
}

func origDirUploaderPid(subDir string) proc.Tpid {
	return ggPid(GG_ORIG, subDir, "", DIR_UPLOADER_SUFFIX)
}

func dirUploaderPid(hash string, subDir string) proc.Tpid {
	return ggPid(hash, subDir, "", DIR_UPLOADER_SUFFIX)
}

func uploaderPid(dir string, subDir string, hash string) proc.Tpid {
	return ggPid(path.Base(dir), subDir, hash, UPLOADER_SUFFIX)
}

func downloaderPid(dir string, subDir string, hash string) proc.Tpid {
	return ggPid(path.Base(dir), subDir, hash, DOWNLOADER_SUFFIX)
}

func ggPid(dir string, subDir string, hash string, suffix string) proc.Tpid {
	return proc.Tpid("[" + dir + "." + subDir + "]" + hash + suffix)
}

func outputHandlerPids(deps map[string]bool) []proc.Tpid {
	out := []proc.Tpid{}
	for d, _ := range deps {
		pid := outputHandlerPid(d)
		noOpPid := noOpPid(pid)
		out = append(out, noOpPid)
	}
	return out
}
