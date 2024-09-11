import importlib
import os

pipeFile = "/tmp/proxy-in.log"
pipeResFile = "/tmp/proxy-out.log"

def sp_import_std(lib):
    with open(pipeFile, "w", buffering=1) as pf:
        with open(pipeResFile) as rd:
            pf.write(f"pdpylib/Lib/{lib}\n")
            wait_done(rd)
            pf.write(f"pfpylib/Lib/{lib}.py\n")
            wait_done(rd)
    
    importlib.invalidate_caches()
    while True:
        try:
            return importlib.import_module(lib)
        except ModuleNotFoundError as e:
            if e.name != lib:
                sp_import_std(e.name)
            else: 
                raise NestedModuleNotFoundError(e)

def sp_exit():
    with open(pipeFile, "w", buffering=1) as pf:
        with open(pipeResFile) as rd:
            pf.write("x\n")
            rd.read(3)

def clear_cache(file):
    with open(pipeFile, "w", buffering=1) as pf:
        with open(pipeResFile) as rd:
            pf.write(f"u{file}\n")
            wait_done(rd)

def download_named(file):
    with open(pipeFile, "w", buffering=1) as pf:
        with open(pipeResFile) as rd:
            pf.write(f"d{file}\n")
            wait_done(rd)

class NamedReader:
    def __init__(self, file, write=False):
        self.file = file
        self.write = write
    def __enter__(self):
        base64 = sp_import_std("base64")
        download_named(self.file)
        print(os.listdir("/filecache"), flush=True)
        if(self.write):
            self.fd = open(f"/filecache/{base64.b64encode(bytes(self.file, 'utf-8')).decode('utf-8')}", "w")
        else: 
            self.fd = open(f"/filecache/{base64.b64encode(bytes(self.file, 'utf-8')).decode('utf-8')}", "r")
        return self
    def __exit__(self, exception_type, exception_value, exception_traceback):
        #Exception handling here
        self.fd.close()
        if(self.write):
            clear_cache(self.file)

def wait_done(rd):
    x = rd.read(1)
    while x != 'd':
        x = rd.read(1)

class NestedModuleNotFoundError(ImportError):
    pass
